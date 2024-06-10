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
	"sync"

	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
)

func newNodes() *nodes {
	return &nodes{
		nodes: map[string]*node{},
	}
}

type nodes struct {
	m     sync.RWMutex
	nodes map[string]*node
}

type node struct {
	nodeID   infrabev1alpha1.NodeID
	systemID string
	provider string
	ipv4     string
	ipv6     string
	routerID string
	as       uint32
	edge     bool
}

func (r *nodes) List() []*node {
	r.m.RLock()
	defer r.m.RUnlock()

	nodes := make([]*node, 0, len(r.nodes))
	for _, node := range r.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (r *nodes) Get(nodeName string) *node {
	r.m.RLock()
	defer r.m.RUnlock()

	n, ok := r.nodes[nodeName]
	if ok {
		return n
	}
	return nil
}

func (r *nodes) Add(nodeName string, n *node) {
	r.m.Lock()
	defer r.m.Unlock()

	r.nodes[nodeName] = n
}
