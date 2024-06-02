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

package networkdesign

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/henderiw/logger/log"
	asbev1alpha1 "github.com/kuidio/kuid/apis/backend/as/v1alpha1"
	genidbev1alpha1 "github.com/kuidio/kuid/apis/backend/genid/v1alpha1"
	ipambev1alpha1 "github.com/kuidio/kuid/apis/backend/ipam/v1alpha1"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	"github.com/kuidio/kuid/pkg/reconcilers/resource"
	"github.com/kuidio/kuid/pkg/resources"
	netwv1alpha1 "github.com/kuidio/kuidapps/apis/network/v1alpha1"
	topov1alpha1 "github.com/kuidio/kuidapps/apis/topo/v1alpha1"
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
	crName         = "networkdesign"
	controllerName = "NetworkDesignController"
	finalizer      = "networkdesign.network.app.kuid.dev/finalizer"
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
		For(&netwv1alpha1.NetworkDesign{}).
		Owns(&ipambev1alpha1.IPClaim{}).
		Owns(&asbev1alpha1.ASClaim{}).
		Owns(&asbev1alpha1.ASIndex{}).
		Owns(&genidbev1alpha1.GENIDClaim{}).
		Watches(&topov1alpha1.Topology{},
			&eventhandler.TopologyEventHandler{
				Client:  mgr.GetClient(),
				ObjList: &netwv1alpha1.NetworkDesignList{},
			}).
		Complete(r)
}

type reconciler struct {
	client.Client
	finalizer *resource.APIFinalizer
	recorder  record.EventRecorder
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = ctrlconfig.InitContext(ctx, controllerName, req.NamespacedName)
	log := log.FromContext(ctx)
	log.Info("reconcile")

	cr := &netwv1alpha1.NetworkDesign{}
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

	if !r.isTopologyReady(ctx, cr) {
		// we do not release resources at this stage
		cr.SetConditions(conditionv1alpha1.Failed("topology not ready"))
		r.recorder.Eventf(cr, corev1.EventTypeWarning, crName, "topology not ready")
		return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}

	if err := r.apply(ctx, cr); err != nil {
		if errd := r.delete(ctx, cr); errd != nil {
			err = errors.Join(err, errd)
			r.handleError(ctx, cr, "cannot delete resource after apply failed", err)
			return reconcile.Result{RequeueAfter: 2 * time.Second}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		}
		r.handleError(ctx, cr, "cannot apply resource", err)
		return ctrl.Result{RequeueAfter: 2 * time.Second}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(conditionv1alpha1.Ready())
	r.recorder.Eventf(cr, corev1.EventTypeNormal, crName, "ready")
	return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) handleError(ctx context.Context, cr *netwv1alpha1.NetworkDesign, msg string, err error) {
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

func (r *reconciler) apply(ctx context.Context, cr *netwv1alpha1.NetworkDesign) error {
	// First claim the global identifiers
	// the ASClaim depends on ASIndex
	res := resources.New(r.Client, resources.Config{
		Owns: []schema.GroupVersionKind{
			genidbev1alpha1.SchemeGroupVersion.WithKind(genidbev1alpha1.GENIDClaimKind),
			asbev1alpha1.SchemeGroupVersion.WithKind(asbev1alpha1.ASIndexKind),
		},
	})

	res.AddNewResource(ctx, cr, cr.GetASIndex())

	//if err := res.AddNewResource(ctx, cr, cr.GetGENIDClaim()); err != nil {
	//	return err
	//}

	if err := res.APIApply(ctx, cr); err != nil {
		return err
	}

	res = resources.New(r.Client, resources.Config{
		Owns: []schema.GroupVersionKind{
			asbev1alpha1.SchemeGroupVersion.WithKind(asbev1alpha1.ASClaimKind),
			ipambev1alpha1.SchemeGroupVersion.WithKind(ipambev1alpha1.IPClaimKind),
		},
	})
	for _, o := range cr.GetIPClaims() {
		res.AddNewResource(ctx, cr, o)
	}
	for _, o := range cr.GetASClaims() {
		res.AddNewResource(ctx, cr, o)
	}
	if err := res.APIApply(ctx, cr); err != nil {
		return err
	}
	return nil
}

func (r *reconciler) delete(ctx context.Context, cr *netwv1alpha1.NetworkDesign) error {
	// First claim the global identifiers
	res := resources.New(r.Client, resources.Config{
		Owns: []schema.GroupVersionKind{
			genidbev1alpha1.SchemeGroupVersion.WithKind(genidbev1alpha1.GENIDClaimKind),
			asbev1alpha1.SchemeGroupVersion.WithKind(asbev1alpha1.ASIndexKind),
			asbev1alpha1.SchemeGroupVersion.WithKind(asbev1alpha1.ASIndexKind),
			ipambev1alpha1.SchemeGroupVersion.WithKind(ipambev1alpha1.IPClaimKind),
		},
	})

	if err := res.APIDelete(ctx, cr); err != nil {
		return err
	}
	return nil
}

func (r *reconciler) isTopologyReady(ctx context.Context, cr *netwv1alpha1.NetworkDesign) bool {
	log := log.FromContext((ctx))
	key := types.NamespacedName{
		Namespace: cr.Namespace,
		Name:      cr.Spec.Topology,
	}

	topo := &topov1alpha1.Topology{}
	if err := r.Client.Get(ctx, key, topo); err != nil {
		if resource.IgnoreNotFound(err) != nil {
			log.Error("cannot get topology", "error", err)
		}
		return false
	}

	return topo.GetCondition(conditionv1alpha1.ConditionTypeReady).Status == metav1.ConditionTrue
}
