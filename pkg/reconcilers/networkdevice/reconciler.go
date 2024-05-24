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
	"time"

	"github.com/henderiw/logger/log"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	"github.com/kuidio/kuid/pkg/reconcilers/resource"
	"github.com/kuidio/kuid/pkg/resources"
	netwv1alpha1 "github.com/kuidio/kuidapps/apis/network/v1alpha1"
	"github.com/kuidio/kuidapps/pkg/devbuilder"
	"github.com/kuidio/kuidapps/pkg/reconcilers"
	"github.com/kuidio/kuidapps/pkg/reconcilers/ctrlconfig"
	"github.com/kuidio/kuidapps/pkg/reconcilers/eventhandler"
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
	crName         = "networkdevice"
	controllerName = "NetworkDeviceController"
	finalizer      = "networkdevice.network.app.kuid.dev/finalizer"
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
		Watches(&netwv1alpha1.Network{},
			&eventhandler.NetworkDeviceEventHandler{
				Client:  mgr.GetClient(),
				ObjList: &netwv1alpha1.NetworkDeviceList{},
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
	cr = cr.DeepCopy()

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

	// check if the processing was done properly
	if cr.GetCondition(conditionv1alpha1.ConditionTypeReady).Status == metav1.ConditionFalse {
		return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}

	//defaultNetwork := isDefaultNetwork(cr.Name)
	// always use default network to fecth the SRE config
	nc, err := r.getNetworkConfig(ctx, types.NamespacedName{
		Namespace: cr.GetNamespace(),
		Name:      fmt.Sprintf("%s.%s", cr.Spec.Topology, netwv1alpha1.DefaultNetwork),
	})
	if err != nil {
		// we need networkconfig for the default network
		// we do not release resources at this stage -> decision do far is no
		r.handleError(ctx, cr, "network config not provided, needed for a default network", nil)
		//return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		return ctrl.Result{Requeue: true}, nil
	}

	if err := r.apply(ctx, cr, nc); err != nil {
		// The delete is not needed as the condition will indicate this is not ready and the resources will not be picked
		/*
			if errd := r.delete(ctx, cr); errd != nil {
				err = errors.Join(err, errd)
				r.handleError(ctx, cr, "cannot delete resource after apply failed", err)
				return reconcile.Result{RequeueAfter: 2 * time.Second}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
			}
		*/
		r.handleError(ctx, cr, "cannot apply resource", err)
		return ctrl.Result{RequeueAfter: 2 * time.Second}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(conditionv1alpha1.DeviceConfigReady()) // This is the DeviceConfigReady condition, not the Ready condition
	r.recorder.Eventf(cr, corev1.EventTypeNormal, crName, "ready")
	return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) handleError(ctx context.Context, cr *netwv1alpha1.Network, msg string, err error) {
	log := log.FromContext(ctx)
	if err == nil {
		cr.SetConditions(conditionv1alpha1.DeviceConfigFailed(msg))
		log.Error(msg)
		r.recorder.Eventf(cr, corev1.EventTypeWarning, crName, msg)
	} else {
		cr.SetConditions(conditionv1alpha1.DeviceConfigFailed(err.Error()))
		log.Error(msg, "error", err)
		r.recorder.Eventf(cr, corev1.EventTypeWarning, crName, fmt.Sprintf("%s, err: %s", msg, err.Error()))
	}
}

func (r *reconciler) apply(ctx context.Context, cr *netwv1alpha1.Network, nc *netwv1alpha1.NetworkConfig) error {
	res := resources.New(r.Client, resources.Config{
		Owns: []schema.GroupVersionKind{
			netwv1alpha1.SchemeGroupVersion.WithKind(netwv1alpha1.NetworkDeviceKind),
		},
	})

	b := devbuilder.New(r.Client, types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name})
	if err := b.Build(ctx, cr, nc); err != nil {
		return err
	}
	for _, nd := range b.GetNetworkDeviceConfigs() {
		res.AddNewResource(ctx, cr, nd)
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
			netwv1alpha1.SchemeGroupVersion.WithKind(netwv1alpha1.NetworkDeviceKind),
		},
	})

	if err := res.APIDelete(ctx, cr); err != nil {
		return err
	}
	return nil
}

func (r *reconciler) getNetworkConfig(ctx context.Context, key types.NamespacedName) (*netwv1alpha1.NetworkConfig, error) {
	//log := log.FromContext((ctx))

	o := &netwv1alpha1.NetworkConfig{}
	if err := r.Client.Get(ctx, key, o); err != nil {
		return nil, err
	}
	return o, nil
}
