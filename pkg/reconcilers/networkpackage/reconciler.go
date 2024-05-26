/*
Copyright 2024 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package networkpackage

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/henderiw/logger/log"
	"github.com/henderiw/store"
	"github.com/kform-dev/kform/pkg/pkgio"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	"github.com/kuidio/kuid/pkg/reconcilers/resource"
	netwv1alpha1 "github.com/kuidio/kuidapps/apis/network/v1alpha1"
	"github.com/kuidio/kuidapps/pkg/reconcilers"
	"github.com/kuidio/kuidapps/pkg/reconcilers/ctrlconfig"
	"github.com/kuidio/kuidapps/pkg/reconcilers/lease"
	perrors "github.com/pkg/errors"
	"github.com/pkgserver-dev/pkgserver/apis/condition"
	"github.com/pkgserver-dev/pkgserver/apis/generated/clientset/versioned"
	pkgv1alpha1 "github.com/pkgserver-dev/pkgserver/apis/pkg/v1alpha1"
	"github.com/pkgserver-dev/pkgserver/apis/pkgrevid"
	configv1alpha1 "github.com/sdcio/config-server/apis/config/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	syaml "sigs.k8s.io/yaml"
)

func init() {
	reconcilers.Register(crName, &reconciler{})
}

const (
	crName                    = "networkpackage"
	controllerName            = "NetworkPackageController"
	finalizer                 = crName + "." + "network.app.kuid.dev/finalizer"
	controllerCondition       = string(netwv1alpha1.ConditionTypeNetworkDeployReady)
	controllerConditionWithCR = controllerCondition + "." + crName

	controllerReadinessCondition = "pkg.pkgserver.dev/network"
	// errors
	errGetCr        = "cannot get cr"
	errUpdateStatus = "cannot update status"
)

// SetupWithManager sets up the controller with the Manager.
func (r *reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, c interface{}) (map[schema.GroupVersionKind]chan event.GenericEvent, error) {

	cfg, ok := c.(*ctrlconfig.ControllerConfig)
	if !ok {
		return nil, fmt.Errorf("cannot initialize, expecting controllerConfig, got: %s", reflect.TypeOf(c).Name())
	}

	r.Client = mgr.GetClient()
	r.finalizer = resource.NewAPIFinalizer(mgr.GetClient(), finalizer)
	r.recorder = mgr.GetEventRecorderFor(controllerName)
	r.clientset = cfg.ClientSet

	return nil, ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&pkgv1alpha1.PackageRevision{}).
		// we only do a watch at the higher level object -> not sure if we need to revisit this
		Complete(r)
}

type reconciler struct {
	//resource.APIPatchingApplicator
	client.Client
	finalizer *resource.APIFinalizer
	recorder  record.EventRecorder
	clientset *versioned.Clientset
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = ctrlconfig.InitContext(ctx, controllerName, req.NamespacedName)
	log := log.FromContext(ctx)
	log.Info("reconcile")

	cr := &pkgv1alpha1.PackageRevision{}
	if err := r.Client.Get(ctx, req.NamespacedName, cr); err != nil {
		// if the resource no longer exists the reconcile loop is done
		if resource.IgnoreNotFound(err) != nil {
			log.Error(errGetCr, "error", err)
			return ctrl.Result{}, perrors.Wrap(resource.IgnoreNotFound(err), errGetCr)
		}
		return ctrl.Result{}, nil
	}
	key := types.NamespacedName{
		Namespace: cr.GetNamespace(),
		Name:      cr.GetName(),
	}
	cr = cr.DeepCopy()

	// if the pkgRev is a catalog packageRevision or
	// if the pkgrev does not have a condition to process the pkgRev
	// this event is not relevant
	if strings.HasPrefix(cr.GetName(), pkgrevid.PkgTarget_Catalog) ||
		!cr.HasReadinessGate(controllerReadinessCondition) {
		return ctrl.Result{}, nil
	}

	if cr.GetDeletionTimestamp().IsZero() &&
		// we need to wait before acting upon creaate/update (for delete the dependency does not matter)
		// 1. if the pkgRev Controller is not done e.g. to clone the repo,
		// 2. if the previous condition is not finished,
		(cr.GetCondition(condition.ConditionTypeReady).Status == metav1.ConditionFalse ||
			cr.GetPreviousCondition(controllerReadinessCondition).Status == metav1.ConditionFalse) {
		return ctrl.Result{}, nil
	}

	l := lease.New(r.Client, key)
	if err := l.AcquireLease(ctx, controllerName); err != nil {
		log.Debug("cannot acquire lease", "key", key.String(), "error", err.Error())
		r.recorder.Eventf(cr, corev1.EventTypeWarning,
			"lease", "error %s", err.Error())
		return ctrl.Result{Requeue: true, RequeueAfter: lease.RequeueInterval}, nil
	}
	r.recorder.Eventf(cr, corev1.EventTypeWarning,
		"lease", "acquired")

	if !cr.GetDeletionTimestamp().IsZero() {

		network, err := r.gatherNetworkInPRR(ctx, cr)
		if err != nil {
			// TODO this needs to be reworked like KFORM where we store the applied resources in inventory
			r.handleError(ctx, cr, "delete, cannot get network resource in package", err)
			return ctrl.Result{Requeue: true}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		}
		log.Info("delete network with package", "network", network)
		if err := r.Client.Delete(ctx, network); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				// TODO this needs to be reworked like KFORM where we store the applied resources in inventory
				r.handleError(ctx, cr, "cannot delete network resource from package", err)
				return ctrl.Result{Requeue: true}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
			}
		}

		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			r.handleError(ctx, cr, "cannot remove finalizer", err)
			return ctrl.Result{Requeue: true}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		}
		log.Debug("Successfully deleted resource")
		return ctrl.Result{}, nil
	}

	if err := r.finalizer.AddFinalizer(ctx, cr); err != nil {
		r.handleError(ctx, cr, "cannot add finalizer", err)
		return ctrl.Result{Requeue: true}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}

	// only act if your condition is not ready
	if cr.GetCondition(controllerReadinessCondition).Status == metav1.ConditionFalse {
		network, err := r.gatherNetworkInPRR(ctx, cr)
		if err != nil {
			// TODO this needs to be reworked like KFORM where we store the applied resources in inventory
			r.handleError(ctx, cr, "cannot get network resource in package", err)
			return ctrl.Result{Requeue: true}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		}

		// apply the network CR to the cluster
		network, err = r.applyNetworkCR(ctx, network)
		if err != nil {
			// The delete is not needed as the condition will indicate this is not ready and the resources will not be picked
			r.handleError(ctx, cr, "cannot apply resource", err)
			return ctrl.Result{RequeueAfter: 2 * time.Second}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		}

		// Check preconditions before deploying the CR(s) on the cluster
		// First the Network global params need to be ready, after the device configs need to be derived and after the specific
		// network device configs needs to be derived
		if !network.AreChildConditionsReady() {
			if network.DidAChildConditionFail() {
				network.SetConditions(conditionv1alpha1.Failed(network.GetFailedMessage()))
			} else {
				network.SetConditions(conditionv1alpha1.Processing(network.GetProcessingMessage()))
			}
			return ctrl.Result{RequeueAfter: 2 * time.Second}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		}

		network.SetConditions(conditionv1alpha1.Ready())
		if err := r.Client.Status().Update(ctx, network); err != nil {
			r.handleError(ctx, cr, "cannot update network cr", err)
			return ctrl.Result{RequeueAfter: 2 * time.Second}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		}

		if err := r.applyResourcesToPRR(ctx, cr, network); err != nil {
			// The delete is not needed as the condition will indicate this is not ready and the resources will not be picked
			r.handleError(ctx, cr, "cannot apply resource", err)
			return ctrl.Result{RequeueAfter: 2 * time.Second}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		}

		log.Debug("network processor complete")
		r.recorder.Eventf(cr, corev1.EventTypeNormal,
			controllerConditionWithCR, "ready")
		cr.SetConditions(condition.ConditionReady(controllerReadinessCondition))
		// set the next condition if applicable
		cType := cr.NextReadinessGate(controllerReadinessCondition)
		if cType != condition.ConditionTypeEnd {
			if !cr.HasCondition(cType) {
				cr.SetConditions(condition.ConditionUpdate(cType, "init", ""))
			}
		}
		return ctrl.Result{}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
	}
	return ctrl.Result{}, nil
}

func (r *reconciler) handleError(ctx context.Context, cr *pkgv1alpha1.PackageRevision, msg string, err error) {
	log := log.FromContext(ctx)
	if err == nil {
		cr.SetConditions(condition.ConditionUpdate(condition.ConditionType(controllerReadinessCondition), "failed", msg))
		log.Error(msg)
		r.recorder.Eventf(cr, corev1.EventTypeWarning, controllerConditionWithCR, msg)
	} else {
		cr.SetConditions(condition.ConditionUpdate(condition.ConditionType(controllerReadinessCondition), "failed", err.Error()))
		log.Error(msg, "error", err)
		r.recorder.Eventf(cr, corev1.EventTypeWarning, controllerConditionWithCR, fmt.Sprintf("%s, err: %s", msg, err.Error()))
	}
}

func (r *reconciler) applyNetworkCR(ctx context.Context, network *netwv1alpha1.Network) (*netwv1alpha1.Network, error) {
	log := log.FromContext(ctx)
	key := types.NamespacedName{
		Name:      network.Name,
		Namespace: network.Namespace,
	}
	if len(network.Annotations) == 0 {
		network.Annotations = map[string]string{}
	}
	log.Info("applyNetworkCR", "networkCR", network)
	network.Annotations["kuid.dev/gitops"] = "true"
	existingNetwork := &netwv1alpha1.Network{}
	if err := r.Client.Get(ctx, key, existingNetwork); err != nil {
		if resource.IgnoreNotFound(err) != nil {
			log.Error("applyNetworkCR get failed", "error", err.Error())
			return nil, err
		}

		if err := r.Client.Create(ctx, network); err != nil {
			log.Error("applyNetworkCR create failed", "error", err.Error())
			return nil, err
		}
	}
	existingNetwork.Spec = network.Spec
	if err := r.Client.Update(ctx, existingNetwork); err != nil {
		log.Error("applyNetworkCR update failed", "error", err.Error(), "existing network CR", existingNetwork)
		return nil, err
	}
	if err := r.Client.Get(ctx, key, network); err != nil {
		log.Error("applyNetworkCR end get failed", "error", err.Error())
		return nil, err
	}
	return network, nil
}

func (r *reconciler) applyResourcesToPRR(ctx context.Context, cr *pkgv1alpha1.PackageRevision, network *netwv1alpha1.Network) error {
	log := log.FromContext(ctx)

	nds, err := r.getNetworkDeviceConfigs(ctx, network)
	if err != nil {
		return err
	}

	deviceConfigs := map[string]string{}
	for _, nd := range nds {
		log.Info("nd config", "name", nd.Name)
		if nd.Status.ProviderConfig == nil || nd.Status.ProviderConfig.Raw == nil {
			return fmt.Errorf("something went wrong, cannot apply configs with empty config")
		}

		config := &configv1alpha1.Config{}
		if err := json.Unmarshal(nd.Status.ProviderConfig.Raw, config); err != nil {
			return err
		}

		b, err := syaml.Marshal(config)
		if err != nil {
			return err
		}

		deviceConfigs[fmt.Sprintf(
			"out/%s.%s.%s.%s.yaml",
			strings.ReplaceAll(config.APIVersion, "/", "_"),
			config.Kind,
			config.Namespace,
			config.Name,
		)] = string(b)
	}

	log.Info("applyResourcesToPRR", "resourceVersion", cr.ResourceVersion)
	newPkgrevResources := pkgv1alpha1.BuildPackageRevisionResources(
		*cr.ObjectMeta.DeepCopy(),
		pkgv1alpha1.PackageRevisionResourcesSpec{
			PackageRevID: *cr.Spec.PackageRevID.DeepCopy(),
			Resources:    deviceConfigs,
		},
		pkgv1alpha1.PackageRevisionResourcesStatus{},
	)
	return r.Client.Update(ctx, newPkgrevResources)
}

func (r *reconciler) getNetworkDeviceConfigs(ctx context.Context, cr *netwv1alpha1.Network) ([]*netwv1alpha1.NetworkDevice, error) {
	nds := []*netwv1alpha1.NetworkDevice{}

	log := log.FromContext(ctx)
	opts := []client.ListOption{}

	//ownObjList := ownObj.NewObjList()
	ndList := netwv1alpha1.NetworkDeviceList{}
	if err := r.Client.List(ctx, &ndList, opts...); err != nil {
		log.Error("get network devices failed", "err", err.Error())
		return nil, err
	}

	for _, nd := range ndList.Items {
		for _, ref := range nd.GetOwnerReferences() {
			if ref.UID == cr.GetUID() {
				nds = append(nds, &nd)
			}
		}
	}

	return nds, nil

}

func (r *reconciler) AreAllDeviceConfigsReady(ctx context.Context, cr *netwv1alpha1.Network) (bool, bool, error) {
	// GetExistingResources retrieves the exisiting resource that match the label selector and the owner reference
	// and puts the results in the resource inventory
	log := log.FromContext(ctx)

	opts := []client.ListOption{}

	//ownObjList := ownObj.NewObjList()
	configList := configv1alpha1.ConfigList{}
	if err := r.Client.List(ctx, &configList, opts...); err != nil {
		log.Error("list configs failed", "err", err.Error())
		return false, false, err
	}
	devicesStatus := []*netwv1alpha1.NetworkStatusDeviceStatus{}
	ready := true
	failures := false
	for _, config := range configList.Items {
		for _, ref := range config.GetOwnerReferences() {
			if ref.UID == cr.GetUID() {
				condition := config.GetCondition(configv1alpha1.ConditionTypeReady)
				if condition.Status == metav1.ConditionFalse {
					if condition.Reason == string(conditionv1alpha1.ConditionReasonFailed) {
						failures = true
					}
					ready = false
					devicesStatus = append(devicesStatus, &netwv1alpha1.NetworkStatusDeviceStatus{
						Node:   getNodeName(config.Name),
						Ready:  false,
						Reason: &condition.Reason,
					})
				} else {
					devicesStatus = append(devicesStatus, &netwv1alpha1.NetworkStatusDeviceStatus{
						Node:   getNodeName(config.Name),
						Ready:  true,
						Reason: nil,
					})
				}
			}
		}
	}
	cr.Status.DevicesDeployStatus = devicesStatus
	return ready, failures, nil
}

func getNodeName(name string) string {
	lastDotIndex := strings.LastIndex(name, ".")
	if lastDotIndex == -1 {
		// If there's no dot, return the original string
		return name
	}
	return name[lastDotIndex+1:]
}

func (r *reconciler) gatherNetworkInPRR(ctx context.Context, cr *pkgv1alpha1.PackageRevision) (*netwv1alpha1.Network, error) {
	log := log.FromContext(ctx)

	/*
		key := types.NamespacedName{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		}
			pkgRevResources := &pkgv1alpha1.PackageRevisionResources{}
			if err := r.Client.Get(ctx, key, pkgRevResources); err != nil {
				return nil, err
			}
	*/

	pkgRevResources, err := r.clientset.PkgV1alpha1().PackageRevisionResourceses(cr.GetNamespace()).Get(ctx, cr.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	networks := []*netwv1alpha1.Network{}
	for path, v := range pkgRevResources.Spec.Resources {
		// the path contains the files with their relative path within the package
		// Anything with a / is a file from a subdirectory and is not a business logic resource
		// for kform
		if !strings.Contains(path, "/") {
			// gather the kform resources which form the business logic for the kform fn run

			if match, err := pkgio.MatchFilesGlob(pkgio.YAMLMatch).ShouldSkipFile(path); err != nil {
				continue // not a yaml file
			} else if match {
				continue // not. ayaml file
			}

			log.Info("gatherNetworkInPRR", "path", path)

			reader := pkgio.YAMLReader{Reader: strings.NewReader(v), Path: path}
			datastore, err := reader.Read(ctx)
			if err != nil {
				return nil, err
			}

			datastore.List(ctx, func(ctx context.Context, k store.Key, rn *yaml.RNode) {
				if rn.GetApiVersion() == netwv1alpha1.SchemeGroupVersion.Identifier() &&
					rn.GetKind() == netwv1alpha1.NetworkKind {

					log.Info("gatherNetworkInPRR network resource", "path", path, "data", rn.MustString())
					network := netwv1alpha1.Network{}
					if err := syaml.Unmarshal([]byte(rn.MustString()), &network); err != nil {
						log.Error("cannot unmarshal network", "err", err.Error())
					}
					networks = append(networks, &network)
				}
			})
		}
	}
	if len(networks) == 0 {
		return nil, fmt.Errorf("no network found in package")
	}
	// HACK retruning 1 network for now
	return networks[0], nil

}
