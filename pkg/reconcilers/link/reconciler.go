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

package link

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/henderiw/logger/log"
	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	"github.com/kuidio/kuid/pkg/reconcilers/resource"
	"github.com/kuidio/kuidapps/pkg/reconcilers"
	"github.com/kuidio/kuidapps/pkg/reconcilers/ctrlconfig"
	"github.com/kuidio/kuidapps/pkg/reconcilers/eventhandler"
	perrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	reconcilers.Register("link", &reconciler{})
}

const (
	crName         = "link"
	controllerName = "LinkController"
	finalizer      = "link.be.kuid.dev/finalizer"
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
		For(&infrabev1alpha1.Link{}).
		Watches(&infrabev1alpha1.Endpoint{},
			&eventhandler.EndpointEventHandler{
				Client:  mgr.GetClient(),
				ObjList: &infrabev1alpha1.LinkList{},
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

	cr := &infrabev1alpha1.Link{}
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
		if err, requeue := r.delete(ctx, cr); err != nil {
			r.handleError(ctx, cr, "canot delete resources", err)
			return reconcile.Result{Requeue: requeue}, perrors.Wrap(r.Update(ctx, cr), errUpdateStatus)
		}

		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			r.handleError(ctx, cr, "cannot remove finalizer", err)
			return ctrl.Result{Requeue: true}, perrors.Wrap(r.Client.Update(ctx, cr), errUpdateStatus)
		}
		log.Debug("Successfully deleted resource")
		return ctrl.Result{}, nil
	}

	if err := r.finalizer.AddFinalizer(ctx, cr); err != nil {
		r.handleError(ctx, cr, "cannot add finalizer", err)
		return ctrl.Result{Requeue: true}, perrors.Wrap(r.Client.Update(ctx, cr), errUpdateStatus)
	}

	if err, requeue := r.apply(ctx, cr); err != nil {
		if errd, _ := r.delete(ctx, cr); errd != nil {
			err = errors.Join(err, errd)
			r.handleError(ctx, cr, "cannot delete resource after apply failed", err)
			return reconcile.Result{Requeue: requeue, RequeueAfter: 1 * time.Second}, perrors.Wrap(r.Update(ctx, cr), errUpdateStatus)
		}
		r.handleError(ctx, cr, "cannot apply resource", err)
		return ctrl.Result{Requeue: requeue}, perrors.Wrap(r.Client.Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(conditionv1alpha1.Ready())
	r.recorder.Eventf(cr, corev1.EventTypeNormal, crName, "ready")
	return ctrl.Result{}, perrors.Wrap(r.Client.Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) handleError(ctx context.Context, cr *infrabev1alpha1.Link, msg string, err error) {
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

func (r *reconciler) apply(ctx context.Context, cr *infrabev1alpha1.Link) (error, bool) {
	// the nil values of ep is alaready checked before so we just look at non nil values
	var errm error
	var requeue bool
	if cr.GetEndPointIDA() != nil {
		err, rq := r.claimEndpoint(ctx, cr, cr.GetEndPointIDA())
		if err != nil {
			errm = errors.Join(errm, err)
			if rq {
				requeue = true
			}
		}
	}

	if cr.GetEndPointIDB() != nil {
		err, rq := r.claimEndpoint(ctx, cr, cr.GetEndPointIDB())
		if err != nil {
			errm = errors.Join(errm, err)
			if rq {
				requeue = true
			}
		}
	}
	return errm, requeue
}

func (r *reconciler) claimEndpoint(ctx context.Context, cr *infrabev1alpha1.Link, epID *infrabev1alpha1.NodeGroupEndpointID) (error, bool) {
	key := types.NamespacedName{
		Namespace: cr.GetNamespace(),
		Name:      epID.KuidString(),
	}
	ep := &infrabev1alpha1.Endpoint{}
	if err := r.Client.Get(ctx, key, ep); err != nil {
		if resource.IgnoreNotFound(err) != nil {
			return err, true
		}
		return err, true
	}

	owned, ref := ep.IsClaimed(cr)
	switch owned {
	case infrabev1alpha1.Free:
		ep.Claim(cr)
		if err := r.Client.Update(ctx, ep); err != nil {
			return err, true
		}
		return nil, false
	case infrabev1alpha1.Owned:
		// do nothing
		return nil, false
	case infrabev1alpha1.Claimed:
		return fmt.Errorf("owned by another reference, ref : %v", ref), false
	default:
		return fmt.Errorf("unexpected owner status got %v", owned), false
	}
}

func (r *reconciler) delete(ctx context.Context, cr *infrabev1alpha1.Link) (error, bool) {

	var errm error
	var requeue bool
	if cr.GetEndPointIDA() != nil {
		err, rq := r.releaseEndpoint(ctx, cr, cr.GetEndPointIDA())
		if err != nil {
			errm = errors.Join(errm, err)
			if rq {
				requeue = true
			}
		}
	}

	if cr.GetEndPointIDB() != nil {
		err, rq := r.releaseEndpoint(ctx, cr, cr.GetEndPointIDB())
		if err != nil {
			errm = errors.Join(errm, err)
			if rq {
				requeue = true
			}
		}
	}
	return errm, requeue
}

func (r *reconciler) releaseEndpoint(ctx context.Context, cr *infrabev1alpha1.Link, epID *infrabev1alpha1.NodeGroupEndpointID) (error, bool) {
	key := types.NamespacedName{
		Namespace: cr.GetNamespace(),
		Name:      epID.KuidString(),
	}
	ep := &infrabev1alpha1.Endpoint{}
	if err := r.Client.Get(ctx, key, ep); err != nil {
		if resource.IgnoreNotFound(err) != nil {
			return err, true
		}
		return nil, false // ep is gone so we are good
	}

	owned, _ := ep.IsClaimed(cr)
	switch owned {
	case infrabev1alpha1.Free:
		return nil, false
	case infrabev1alpha1.Owned:
		ep.Release(cr)
		if err := r.Client.Update(ctx, ep); err != nil {
			return err, true
		}
		return nil, false
	case infrabev1alpha1.Claimed:
		return nil, false // we return positively
	default:
		return fmt.Errorf("unexpected owner status got %v", owned), false
	}
}
