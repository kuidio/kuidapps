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

package devbuilder

import (
	"context"
	"fmt"

	"github.com/kuidio/kuid/apis/backend"
	asbev1alpha1 "github.com/kuidio/kuid/apis/backend/as/v1alpha1"
	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	ipambev1alpha1 "github.com/kuidio/kuid/apis/backend/ipam/v1alpha1"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *DeviceBuilder) GetNodes(ctx context.Context) ([]*infrabev1alpha1.Node, error) {
	nodes := make([]*infrabev1alpha1.Node, 0)

	opts := []client.ListOption{
		client.InNamespace(r.network.Namespace),
	}
	nodeList := &infrabev1alpha1.NodeList{}
	if err := r.Client.List(ctx, nodeList, opts...); err != nil {
		return nil, err
	}

	for _, n := range nodeList.Items {
		if r.network.Spec.Topology == n.Spec.NodeGroup {
			nodes = append(nodes, &n)
		}
	}
	return nodes, nil
}

func (r *DeviceBuilder) GetLinks(ctx context.Context) ([]*infrabev1alpha1.Link, error) {
	links := make([]*infrabev1alpha1.Link, 0)
	opts := []client.ListOption{
		client.InNamespace(r.network.Namespace),
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
			if ep.NodeGroup != r.network.Spec.Topology {
				continue
			}
		}
		links = append(links, &l)
	}
	return links, nil
}

func (r *DeviceBuilder) getIPClaim(ctx context.Context, key types.NamespacedName) (string, error) {
	claim := &ipambev1alpha1.IPClaim{}
	if err := r.Client.Get(ctx, key, claim); err != nil {
		return "", err
	}
	if claim.GetCondition(conditionv1alpha1.ConditionTypeReady).Condition.Status == metav1.ConditionFalse {
		return "", fmt.Errorf("ipclaim %s condition not ready", key.String())
	}
	if claim.Status.Address == nil {
		return "", fmt.Errorf("ipclaim %s address not found ", key.String())
	}
	return *claim.Status.Address, nil
}

func (r *DeviceBuilder) getASClaim(ctx context.Context, nsn types.NamespacedName) (uint32, error) {
	claim := &asbev1alpha1.ASClaim{}
	if err := r.Client.Get(ctx, nsn, claim); err != nil {
		return 0, err
	}
	if claim.GetCondition(conditionv1alpha1.ConditionTypeReady).Condition.Status == metav1.ConditionFalse {
		return 0, fmt.Errorf("asclaim %s condition not ready", nsn.String())
	}
	if claim.Status.ID == nil {
		return 0, fmt.Errorf("asclaim %s id not found ", nsn.String())
	}
	return *claim.Status.ID, nil
}

func (r *DeviceBuilder) getEndpoint(ctx context.Context, nsn types.NamespacedName) (*infrabev1alpha1.Endpoint, error) {
	ep := &infrabev1alpha1.Endpoint{}
	if err := r.Client.Get(ctx, nsn, ep); err != nil {
		return nil, err
	}
	return ep, nil

}

func (r *DeviceBuilder) getNode(ctx context.Context, nsn types.NamespacedName) (*infrabev1alpha1.Node, error) {
	n := &infrabev1alpha1.Node{}
	if err := r.Client.Get(ctx, nsn, n); err != nil {
		return nil, err
	}
	return n, nil

}
