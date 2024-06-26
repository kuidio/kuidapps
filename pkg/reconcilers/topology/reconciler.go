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

package topology

import (
	"context"
	"fmt"
	"reflect"

	"github.com/henderiw/logger/log"
	genidbev1alpha1 "github.com/kuidio/kuid/apis/backend/genid/v1alpha1"
	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	"github.com/kuidio/kuid/pkg/reconcilers/resource"
	"github.com/kuidio/kuid/pkg/resources"
	topov1alpha1 "github.com/kuidio/kuidapps/apis/topo/v1alpha1"
	"github.com/kuidio/kuidapps/pkg/clab"
	"github.com/kuidio/kuidapps/pkg/reconcilers"
	"github.com/kuidio/kuidapps/pkg/reconcilers/ctrlconfig"
	perrors "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	reconcilers.Register("topology", &reconciler{})
}

const (
	crName         = "topology"
	controllerName = "TopologyController"
	finalizer      = "topology.topo.app.kuid.dev/finalizer"
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
	//r.APIPatchingApplicator = resource.NewAPIPatchingApplicator(mgr.GetClient())
	r.finalizer = resource.NewAPIFinalizer(mgr.GetClient(), finalizer)
	r.recorder = mgr.GetEventRecorderFor(controllerName)

	return nil, ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&topov1alpha1.Topology{}).
		Owns(&infrabev1alpha1.Node{}).
		Owns(&infrabev1alpha1.Link{}).
		Owns(&genidbev1alpha1.GENIDIndex{}).
		/*Watches(&vxlanbev1alpha1.VXLANIndex{},
		&eventhandler.IPEntryEventHandler{
			Client:  mgr.GetClient(),
			ObjList: &vxlanbev1alpha1.VXLANIndexList{},
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

	cr := &topov1alpha1.Topology{}
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
			return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
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

	if cr.Spec.ContainerLab == nil {
		r.handleError(ctx, cr, "no containerlab topology provided", nil)
		return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}

	if err := r.apply(ctx, cr); err != nil {
		// TODO should be put back -> debugging error
		/*
			if errd := r.delete(ctx, cr); errd != nil {
				err = errors.Join(err, errd)
				r.handleError(ctx, cr, "cannot delete resources after populate failed", err)
				return reconcile.Result{Requeue: true}, perrors.Wrap(r.Status().Update(ctx, cr), errUpdateStatus)
			}
		*/
		r.handleError(ctx, cr, "cannot apply topology resources", err)
		return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
	}

	cr.SetConditions(conditionv1alpha1.Ready())
	r.recorder.Eventf(cr, corev1.EventTypeNormal, crName, "ready")
	return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) handleError(ctx context.Context, cr *topov1alpha1.Topology, msg string, err error) {
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

func (r *reconciler) apply(ctx context.Context, cr *topov1alpha1.Topology) error {
	resources := resources.New(r.Client, resources.Config{
		Owns: []schema.GroupVersionKind{
			genidbev1alpha1.SchemeGroupVersion.WithKind(genidbev1alpha1.GENIDIndexKind),
			infrabev1alpha1.SchemeGroupVersion.WithKind(infrabev1alpha1.NodeKind),
			infrabev1alpha1.SchemeGroupVersion.WithKind(infrabev1alpha1.LinkKind),
		},
	})

	clab, err := clab.NewClabKuid(cr.GetSiteID(), *cr.Spec.ContainerLab)
	if err != nil {
		return fmt.Errorf("parsing clab topo failed: %s", err.Error())
	}

	for _, o := range clab.GetNodes(ctx) {
		resources.AddNewResource(ctx, cr, o)
	}
	for _, o := range clab.GetLinks(ctx) {
		resources.AddNewResource(ctx, cr, o)
	}
	resources.AddNewResource(ctx, cr, cr.GetGENIDIndex())

	return resources.APIApply(ctx, cr)
}

func (r *reconciler) delete(ctx context.Context, cr *topov1alpha1.Topology) error {
	resources := resources.New(r.Client, resources.Config{
		Owns: []schema.GroupVersionKind{
			genidbev1alpha1.SchemeGroupVersion.WithKind(genidbev1alpha1.GENIDIndexKind),
			infrabev1alpha1.SchemeGroupVersion.WithKind(infrabev1alpha1.NodeKind),
			infrabev1alpha1.SchemeGroupVersion.WithKind(infrabev1alpha1.LinkKind),
		},
	})

	return resources.APIDelete(ctx, cr)
}
