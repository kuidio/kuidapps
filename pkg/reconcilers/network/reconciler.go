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

package network

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/henderiw/logger/log"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	"github.com/kuidio/kuid/pkg/reconcilers/resource"
	"github.com/kuidio/kuid/pkg/resources"
	netwv1alpha1 "github.com/kuidio/kuidapps/apis/network/v1alpha1"
	"github.com/kuidio/kuidapps/pkg/reconcilers"
	"github.com/kuidio/kuidapps/pkg/reconcilers/ctrlconfig"
	"github.com/kuidio/kuidapps/pkg/reconcilers/eventhandler"
	"github.com/kuidio/kuidapps/pkg/reconcilers/lease"
	perrors "github.com/pkg/errors"
	configv1alpha1 "github.com/sdcio/config-server/apis/config/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func init() {
	reconcilers.Register(crName, &reconciler{})
}

const (
	crName         = "network"
	controllerName = "NetworkController"
	finalizer      = "network.network.app.kuid.dev/finalizer"
	// errors
	errGetCr        = "cannot get cr"
	errUpdateStatus = "cannot update status"
)

// SetupWithManager sets up the controller with the Manager.
func (r *reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, c interface{}) (map[schema.GroupVersionKind]chan event.GenericEvent, error) {

	_, ok := c.(*ctrlconfig.ControllerConfig)
	if !ok {
		return nil, fmt.Errorf("cannot initialize, expecting controllerConfig, got: %s", reflect.TypeOf(c).Name())
	}

	r.Client = mgr.GetClient()
	r.finalizer = resource.NewAPIFinalizer(mgr.GetClient(), finalizer)
	r.recorder = mgr.GetEventRecorderFor(controllerName)

	return nil, ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&netwv1alpha1.Network{}).
		Owns(&configv1alpha1.Config{}).
		Watches(&netwv1alpha1.NetworkDevice{},
			&eventhandler.NetworkDeviceForNetworkEventHandler{
				Client:  mgr.GetClient(),
				ObjList: &netwv1alpha1.NetworkList{},
			}).
		Complete(r)
}

type reconciler struct {
	//resource.APIPatchingApplicator
	client.Client
	finalizer *resource.APIFinalizer
	recorder  record.EventRecorder
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = ctrlconfig.InitContext(ctx, controllerName, req.NamespacedName)
	log := log.FromContext(ctx)
	log.Info("reconcile")

	cr := &netwv1alpha1.Network{}
	if err := r.Client.Get(ctx, req.NamespacedName, cr); err != nil {
		// if the resource no longer exists the reconcile loop is done
		if resource.IgnoreNotFound(err) != nil {
			log.Error(errGetCr, "error", err)
			return ctrl.Result{}, perrors.Wrap(resource.IgnoreNotFound(err), errGetCr)
		}
		return ctrl.Result{}, nil
	}

	// The network is deployed using gitops, this controller
	// should not act
	if _, ok := cr.GetAnnotations()["kuid.dev/gitops"]; ok {
		return ctrl.Result{}, nil
	}
	key := types.NamespacedName{
		Namespace: cr.GetNamespace(),
		Name:      cr.GetName(),
	}
	cr = cr.DeepCopy()

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
		if err := r.delete(ctx, cr); err != nil {
			r.handleError(ctx, cr, "canot delete resources", err)
			return ctrl.Result{Requeue: true}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
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

	// Check preconditions before deploying the CR(s) on the cluster
	// First the Network global params need to be ready, after the device configs need to be derived and after the specific
	// network device configs needs to be derived
	if !cr.AreChildConditionsReady() {
		if cr.DidAChildConditionFail() {
			cr.SetConditions(conditionv1alpha1.Failed(cr.GetFailedMessage()))
		} else {
			cr.SetConditions(conditionv1alpha1.Processing(cr.GetProcessingMessage()))
		}
		return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}
	// All precondiitons are met so we can deploy
	// the configs via SDCIO
	if err := r.apply(ctx, cr); err != nil {
		// The delete is not needed as the condition will indicate this is not ready and the resources will not be picked
		r.handleError(ctx, cr, "cannot apply resource", err)
		return ctrl.Result{RequeueAfter: 2 * time.Second}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}
	// After applying the configs to SDCIO, check the condition if all the configs were successfull
	// before we declare victory
	allDevicesready, failures, err := r.AreAllDeviceConfigsReady(ctx, cr)
	if err != nil {
		r.handleError(ctx, cr, "cannot gather network device status", err)
		return ctrl.Result{RequeueAfter: 2 * time.Second}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}
	if !allDevicesready {
		if failures {
			cr.SetConditions(conditionv1alpha1.Failed("some devices failed, see device status list"))
		}
		cr.SetConditions(conditionv1alpha1.Processing("some devices are still processing"))
		return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(conditionv1alpha1.Ready())
	r.recorder.Eventf(cr, corev1.EventTypeNormal, crName, "ready")
	return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) handleError(ctx context.Context, cr *netwv1alpha1.Network, msg string, err error) {
	log := log.FromContext(ctx)
	if err == nil {
		cr.SetConditions(conditionv1alpha1.Failed(msg))
		log.Error(msg)
		r.recorder.Eventf(cr, corev1.EventTypeWarning, crName, msg)
	} else {
		cr.SetConditions(conditionv1alpha1.Failed(err.Error()))
		log.Error(msg, "error", err)
		r.recorder.Eventf(cr, corev1.EventTypeWarning, crName, fmt.Sprintf("%s, err: %s", msg, err.Error()))
	}
}

func (r *reconciler) apply(ctx context.Context, cr *netwv1alpha1.Network) error {
	log := log.FromContext(ctx)
	res := resources.New(r.Client, resources.Config{
		Owns: []schema.GroupVersionKind{
			configv1alpha1.SchemeGroupVersion.WithKind(configv1alpha1.ConfigKind),
		},
	})

	nds, err := r.getNetworkDeviceConfigs(ctx, cr)
	if err != nil {
		return err
	}

	for _, nd := range nds {
		log.Info("nd config", "name", nd.Name)
		if nd.Status.ProviderConfig == nil || nd.Status.ProviderConfig.Raw == nil {
			return fmt.Errorf("something went wrong, cannot not apply configs with emoty config")
		}
		config := &configv1alpha1.Config{}
		if err := json.Unmarshal(nd.Status.ProviderConfig.Raw, config); err != nil {
			return err
		}
		res.AddNewResource(ctx, cr, config)
	}

	if err := res.APIApply(ctx, cr); err != nil {
		return err
	}
	return nil
}

func (r *reconciler) delete(ctx context.Context, cr *netwv1alpha1.Network) error {
	// First claim the global identifiers
	res := resources.New(r.Client, resources.Config{
		Owns: []schema.GroupVersionKind{
			configv1alpha1.SchemeGroupVersion.WithKind(configv1alpha1.ConfigKind),
		},
	})

	if err := res.APIDelete(ctx, cr); err != nil {
		return err
	}
	return nil
}

func (r *reconciler) getNetworkDeviceConfigs(ctx context.Context, cr *netwv1alpha1.Network) ([]*netwv1alpha1.NetworkDevice, error) {
	nds := []*netwv1alpha1.NetworkDevice{}

	log := log.FromContext(ctx)
	opts := []client.ListOption{}

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
	sort.SliceStable(devicesStatus, func(i, j int) bool {
		return devicesStatus[i].Node < devicesStatus[j].Node
	})

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
