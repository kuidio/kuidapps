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
	netwv1alpha1 "github.com/kuidio/kuidapps/apis/network/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type NetworkDesignForNetworkEventHandler struct {
	Client  client.Client
	NetworkList *netwv1alpha1.NetworkList
}

// Create enqueues a request
func (r *NetworkDesignForNetworkEventHandler) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.Object, q)
}

// Create enqueues a request
func (r *NetworkDesignForNetworkEventHandler) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.ObjectOld, q)
	r.add(ctx, evt.ObjectNew, q)
}

// Create enqueues a request
func (r *NetworkDesignForNetworkEventHandler) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.Object, q)
}

// Create enqueues a request
func (r *NetworkDesignForNetworkEventHandler) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	r.add(ctx, evt.Object, q)
}

func (r *NetworkDesignForNetworkEventHandler) add(ctx context.Context, obj runtime.Object, queue adder) {
	nd, ok := obj.(*netwv1alpha1.NetworkDesign)
	if !ok {
		return
	}

	log := log.FromContext(ctx)
	//log.Info("event", "gvk", ipambev1alpha1.SchemeGroupVersion.WithKind(ipambev1alpha1.IPEntryKind).String(), "name", cr.GetName())

	opts := []client.ListOption{
		client.InNamespace(nd.Namespace),
	}
	networkList := r.NetworkList
	if err := r.Client.List(ctx, networkList, opts...); err != nil {
		log.Error("cannot list object", "error", err)
		return
	}
	// walk over the links
	// if endpoint has the same endpointID -> retrigger
	// if the ownerref is link retrigger
	for _, network := range networkList.Items {
		if  network.Spec.Topology == nd.Spec.Topology {
			key := types.NamespacedName{
				Namespace: network.GetNamespace(),
				Name:      network.GetName()}
			log.Info("event requeue", "key", key.String())
			queue.Add(reconcile.Request{NamespacedName: key})
			continue
		}
	}
}
