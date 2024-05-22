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
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/henderiw/logger/log"
	"github.com/kuidio/kuid/apis/backend"
	asbev1alpha1 "github.com/kuidio/kuid/apis/backend/as/v1alpha1"
	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	ipambev1alpha1 "github.com/kuidio/kuid/apis/backend/ipam/v1alpha1"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	"github.com/kuidio/kuid/pkg/reconcilers/resource"
	"github.com/kuidio/kuid/pkg/resources"
	netwv1alpha1 "github.com/kuidio/kuidapps/apis/network/v1alpha1"
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
	reconcilers.Register("network", &reconciler{})
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
		Owns(&ipambev1alpha1.IPClaim{}).
		Watches(&netwv1alpha1.NetworkConfig{},
			&eventhandler.NetworkConfigEventHandler{
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

	defaultNetwork := isDefaultNetwork(cr.Name)
	nc, err := r.getNetworkConfig(ctx, cr)
	if err != nil {
		if defaultNetwork {
			// we need networkconfig for the default network
			// we do not release resources at this stage -> decision do far is no
			r.handleError(ctx, cr, "network config not provided, needed for a default network", nil)
			return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		}
	} else {
		if nc.GetCondition(conditionv1alpha1.ConditionTypeReady).Status == metav1.ConditionFalse {
			r.handleError(ctx, cr, "network config not ready", nil)
			return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		}
	}

	if defaultNetwork {
		if err := r.applyDefaultNetwork(ctx, cr, nc); err != nil {
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
	} else {
		if err := r.applyNetwork(ctx, cr, nc); err != nil {
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

func (r *reconciler) applyDefaultNetwork(ctx context.Context, cr *netwv1alpha1.Network, nc *netwv1alpha1.NetworkConfig) error {
	res := resources.New(r.Client, resources.Config{
		Owns: []schema.GroupVersionKind{
			ipambev1alpha1.SchemeGroupVersion.WithKind(ipambev1alpha1.IPClaimKind),
			asbev1alpha1.SchemeGroupVersion.WithKind(asbev1alpha1.ASClaimKind),
		},
	})

	nodes, err := r.GetNodes(ctx, cr)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		for _, ipclaim := range nc.GetNodeIPClaims(cr, n) {
			res.AddNewResource(ctx, cr, ipclaim)
		}

		if asClaim := nc.GetNodeASClaim(cr, n); asClaim != nil {
			res.AddNewResource(ctx, cr, asClaim)
		}
	}
	links, err := r.GetLinks(ctx, cr)
	if err != nil {
		return err
	}
	for _, l := range links {
		for _, ipclaim := range nc.GetLinkIPClaims(cr, l) {
			res.AddNewResource(ctx, cr, ipclaim)
		}
	}

	if err := res.APIApply(ctx, cr); err != nil {
		return err
	}
	return nil
}

func (r *reconciler) applyNetwork(ctx context.Context, cr *netwv1alpha1.Network, nc *netwv1alpha1.NetworkConfig) error {
	/*
		res := resources.New(r.Client, resources.Config{
			Owns: []schema.GroupVersionKind{
				ipambev1alpha1.SchemeGroupVersion.WithKind(ipambev1alpha1.IPClaimKind),
			},
		})
	*/

	return nil
}

func (r *reconciler) delete(ctx context.Context, cr *netwv1alpha1.Network) error {
	// First claim the global identifiers
	res := resources.New(r.Client, resources.Config{
		Owns: []schema.GroupVersionKind{
			ipambev1alpha1.SchemeGroupVersion.WithKind(ipambev1alpha1.IPClaimKind),
			asbev1alpha1.SchemeGroupVersion.WithKind(asbev1alpha1.ASClaimKind),
		},
	})

	if err := res.APIDelete(ctx, cr); err != nil {
		return err
	}
	return nil
}

func (r *reconciler) getNetworkConfig(ctx context.Context, cr *netwv1alpha1.Network) (*netwv1alpha1.NetworkConfig, error) {
	//log := log.FromContext((ctx))
	key := types.NamespacedName{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}

	o := &netwv1alpha1.NetworkConfig{}
	if err := r.Client.Get(ctx, key, o); err != nil {
		return nil, err
	}
	return o, nil
}

func isDefaultNetwork(crName string) bool {
	parts := strings.Split(crName, ".")
	networkName := crName
	if len(parts) > 0 {
		networkName = parts[len(parts)-1]
	}
	if networkName == "default" {
		return true
	}
	return false
}

func (r *reconciler) GetNodes(ctx context.Context, cr *netwv1alpha1.Network) ([]*infrabev1alpha1.Node, error) {
	nodes := make([]*infrabev1alpha1.Node, 0)
	topology := cr.Spec.Topology

	opts := []client.ListOption{
		client.InNamespace(cr.Namespace),
	}
	nodeList := &infrabev1alpha1.NodeList{}
	if err := r.Client.List(ctx, nodeList, opts...); err != nil {
		return nil, err
	}

	for _, n := range nodeList.Items {
		if topology == n.Spec.NodeGroup {
			nodes = append(nodes, &n)
		}
	}
	return nodes, nil
}

//KuidINVLinkTypeKey

func (r *reconciler) GetLinks(ctx context.Context, cr *netwv1alpha1.Network) ([]*infrabev1alpha1.Link, error) {
	links := make([]*infrabev1alpha1.Link, 0)
	topology := cr.Spec.Topology

	opts := []client.ListOption{
		client.InNamespace(cr.Namespace),
	}
	linkList := &infrabev1alpha1.LinkList{}
	if err := r.Client.List(ctx, linkList, opts...); err != nil {
		return nil, err
	}

	for _, l := range linkList.Items {
		linkType, ok := l.Spec.UserDefinedLabels.Labels[backend.KuidINVLinkTypeKey]
		if !ok {
			continue
		}
		if linkType != "infra" {
			continue
		}
		for _, ep := range l.Spec.Endpoints {
			if ep.NodeGroup != topology {
				continue
			}
		}
		links = append(links, &l)
	}
	return links, nil
}
