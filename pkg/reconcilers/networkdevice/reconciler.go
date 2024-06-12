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

package networkdevice

import (
	"context"
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
	"github.com/kuidio/kuidapps/pkg/devbuilder"
	"github.com/kuidio/kuidapps/pkg/reconcilers"
	"github.com/kuidio/kuidapps/pkg/reconcilers/ctrlconfig"
	perrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	reconcilers.Register(crName, &reconciler{})
}

const (
	crName                    = "networkdevice"
	controllerName            = "NetworkDeviceController"
	finalizer                 = "networkdevice.network.app.kuid.dev/finalizer"
	controllerCondition       = string(netwv1alpha1.ConditionTypeNetworkDeviceReady)
	controllerConditionWithCR = controllerCondition + "." + crName
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
		Owns(&netwv1alpha1.NetworkDevice{}).
		/*
			Watches(&netwv1alpha1.NetworkDesign{},
				&eventhandler.NetworkDesignForNetworkEventHandler{
					Client:  mgr.GetClient(),
					ObjList: &netwv1alpha1.NetworkList{},
				}).
		*/
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

	cr = cr.DeepCopy()

	/*
		key := types.NamespacedName{
			Namespace: cr.GetNamespace(),
			Name:      cr.GetName(),
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
	*/
	if !cr.GetDeletionTimestamp().IsZero() {
		if err := r.delete(ctx, cr); err != nil {
			r.handleError(ctx, cr, "canot delete resources", err)
			return reconcile.Result{Requeue: true}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
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

	// check if the processing was done properly of the network parameter controller
	if cr.GetCondition(netwv1alpha1.ConditionTypeNetworkParamReady).Status == metav1.ConditionFalse {
		//cr.SetConditions(netwv1alpha1.NetworkDeviceProcessing("network parameter controller not ready"))
		//return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		return ctrl.Result{}, nil
	}
	// if the condition was met we dont need to act any longer -> the uber network reconciler will change
	// the condition if a change was detected
	if cr.GetCondition(netwv1alpha1.ConditionTypeNetworkDeviceReady).Status == metav1.ConditionTrue {
		return ctrl.Result{}, nil
	}

	// always use default network to fetch the SRE config
	nd, err := r.getNetworkDesign(ctx, types.NamespacedName{
		Namespace: cr.GetNamespace(),
		Name:      fmt.Sprintf("%s.%s", cr.Spec.Topology, netwv1alpha1.DefaultNetwork),
	})
	if err != nil {
		// a network design for the default network is mandatory
		// we do not release resources at this stage -> decision do far is no
		r.handleError(ctx, cr, "cannot reconcile a network without a default network design", nil)
		//return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		return ctrl.Result{Requeue: true}, nil
	}

	if err := r.apply(ctx, cr, nd); err != nil {
		// The delete is not needed as the condition will indicate this is not ready and the resources will not be picked
		r.handleError(ctx, cr, "cannot apply resource", err)
		return ctrl.Result{RequeueAfter: 2 * time.Second}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}

	allDevicesready, failures, err := r.AreAllDeviceConfigsReady(ctx, cr)
	if err != nil {
		r.handleError(ctx, cr, "cannot gather network device status", err)
		return ctrl.Result{RequeueAfter: 2 * time.Second}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}
	if !allDevicesready {
		if failures {
			cr.SetConditions(netwv1alpha1.NetworkDeviceFailed("some devices failed, see device status list"))
			return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		}
		cr.SetConditions(netwv1alpha1.NetworkDeviceProcessing("some devices are still processing"))
		return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(netwv1alpha1.NetworkDeviceReady()) // This is the DeviceConfigReady condition, not the Ready condition
	r.recorder.Eventf(cr, corev1.EventTypeNormal, controllerConditionWithCR, "ready")
	return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) handleError(ctx context.Context, cr *netwv1alpha1.Network, msg string, err error) {
	log := log.FromContext(ctx)
	if err == nil {
		cr.SetConditions(netwv1alpha1.NetworkDeviceFailed(msg))
		log.Error(msg)
		r.recorder.Eventf(cr, corev1.EventTypeWarning, controllerConditionWithCR, msg)
	} else {
		cr.SetConditions(netwv1alpha1.NetworkDeviceFailed(err.Error()))
		log.Error(msg, "error", err)
		r.recorder.Eventf(cr, corev1.EventTypeWarning, controllerConditionWithCR, fmt.Sprintf("%s, err: %s", msg, err.Error()))
	}
}

func (r *reconciler) apply(ctx context.Context, network *netwv1alpha1.Network, networkDesign *netwv1alpha1.NetworkDesign) error {
	res := resources.New(r.Client, resources.Config{
		Owns: []schema.GroupVersionKind{
			netwv1alpha1.SchemeGroupVersion.WithKind(netwv1alpha1.NetworkDeviceKind),
		},
	})

	b := devbuilder.New(r.Client, network, networkDesign)
	if err := b.Build(ctx); err != nil {
		return err
	}
	for _, networkDeviceConfig := range b.GetNetworkDeviceConfigs() {
		res.AddNewResource(ctx, network, networkDeviceConfig)
	}

	if err := res.APIApply(ctx, network); err != nil {
		return err
	}
	return nil
}

func (r *reconciler) delete(ctx context.Context, cr *netwv1alpha1.Network) error {
	// First claim the global identifiers
	res := resources.New(r.Client, resources.Config{
		Owns: []schema.GroupVersionKind{
			netwv1alpha1.SchemeGroupVersion.WithKind(netwv1alpha1.NetworkDeviceKind),
		},
	})

	if err := res.APIDelete(ctx, cr); err != nil {
		return err
	}
	return nil
}

func (r *reconciler) getNetworkDesign(ctx context.Context, key types.NamespacedName) (*netwv1alpha1.NetworkDesign, error) {
	//log := log.FromContext((ctx))

	o := &netwv1alpha1.NetworkDesign{}
	if err := r.Client.Get(ctx, key, o); err != nil {
		return nil, err
	}
	return o, nil
}

func (r *reconciler) AreAllDeviceConfigsReady(ctx context.Context, cr *netwv1alpha1.Network) (bool, bool, error) {
	// GetExistingResources retrieves the exisiting resource that match the label selector and the owner reference
	// and puts the results in the resource inventory
	log := log.FromContext(ctx)

	opts := []client.ListOption{}

	//ownObjList := ownObj.NewObjList()
	ndList := netwv1alpha1.NetworkDeviceList{}
	if err := r.Client.List(ctx, &ndList, opts...); err != nil {
		log.Error("get network devices failed", "err", err.Error())
		return false, false, err
	}
	devicesStatus := []*netwv1alpha1.NetworkStatusDeviceStatus{}
	ready := true
	failures := false
	for _, nd := range ndList.Items {
		for _, ref := range nd.GetOwnerReferences() {
			if ref.UID == cr.GetUID() {
				condition := nd.GetCondition(conditionv1alpha1.ConditionTypeReady)
				if condition.Status == metav1.ConditionFalse {
					if condition.Reason == string(conditionv1alpha1.ConditionReasonFailed) {
						failures = true
					}
					ready = false
					devicesStatus = append(devicesStatus, &netwv1alpha1.NetworkStatusDeviceStatus{
						Node:   getNodeName(nd.Name),
						Ready:  false,
						Reason: &condition.Reason,
					})
				} else {
					devicesStatus = append(devicesStatus, &netwv1alpha1.NetworkStatusDeviceStatus{
						Node:   getNodeName(nd.Name),
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

	cr.Status.DevicesConfigStatus = devicesStatus
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
