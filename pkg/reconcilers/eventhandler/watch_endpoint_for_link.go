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

package eventhandler

import (
	"context"

	"github.com/henderiw/logger/log"
	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type EndpointEventHandler struct {
	Client  client.Client
	ObjList *infrabev1alpha1.LinkList
}

// Create enqueues a request
func (r *EndpointEventHandler) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.Object, q)
}

// Create enqueues a request
func (r *EndpointEventHandler) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.ObjectOld, q)
	r.add(ctx, evt.ObjectNew, q)
}

// Create enqueues a request
func (r *EndpointEventHandler) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.Object, q)
}

// Create enqueues a request
func (r *EndpointEventHandler) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.Object, q)
}

func (r *EndpointEventHandler) add(ctx context.Context, obj runtime.Object, queue adder) {
	cr, ok := obj.(*infrabev1alpha1.Endpoint)
	if !ok {
		return
	}

	log := log.FromContext(ctx)
	//log.Info("event", "gvk", ipambev1alpha1.SchemeGroupVersion.WithKind(ipambev1alpha1.IPEntryKind).String(), "name", cr.GetName())

	opts := []client.ListOption{
		client.InNamespace(cr.Namespace),
	}
	objList := r.ObjList
	if err := r.Client.List(ctx, objList, opts...); err != nil {
		log.Error("cannot list object", "error", err)
		return
	}
	// walk over the links
	// if endpoint has the same endpointID -> retrigger
	// if the ownerref is link retrigger
	for _, obj := range objList.GetLinks() {
		// check if the connection profile is referenced in the discoveryProfile
		//log.Info("event", "objOwnerRef", obj.GetOwnerReference().String(), "crOwnerRef", cr.GetOwnerReference().String())

		if obj.GetEndPointIDA() != nil && *obj.GetEndPointIDA() == cr.Spec.NodeGroupEndpointID {
			key := types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName()}
			log.Info("event requeue", "key", key.String())
			queue.Add(reconcile.Request{NamespacedName: key})
			continue
		}

		if obj.GetEndPointIDB() != nil && *obj.GetEndPointIDB() == cr.Spec.NodeGroupEndpointID {
			key := types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName()}
			log.Info("event requeue", "key", key.String())
			queue.Add(reconcile.Request{NamespacedName: key})
			continue
		}

		if cr.IsOwnedBy(obj) {
			key := types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      obj.GetName()}
			log.Info("event requeue", "key", key.String())
			queue.Add(reconcile.Request{NamespacedName: key})
			continue
		}
	}
}
