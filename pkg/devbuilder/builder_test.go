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
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kuidio/kuid/apis/backend"
	asbev1alpha1 "github.com/kuidio/kuid/apis/backend/as/v1alpha1"
	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	ipambev1alpha1 "github.com/kuidio/kuid/apis/backend/ipam/v1alpha1"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	bebackend "github.com/kuidio/kuid/pkg/backend/backend"
	"github.com/kuidio/kuid/pkg/backend/ipam"
	netwv1alpha1 "github.com/kuidio/kuidapps/apis/network/v1alpha1"
	topov1alpha1 "github.com/kuidio/kuidapps/apis/topo/v1alpha1"
	"github.com/kuidio/kuidapps/pkg/clab"
	"github.com/kuidio/kuidapps/pkg/testhelper"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

const (
	topologyFileName      = "topology.yaml"
	ipindexFileName       = "ipindex.yaml"
	networkDesignFileName = "networkdesign.yaml"
)

func TestDeviceBuilder(t *testing.T) {
	cases := map[string]struct {
		path        string
		network     *netwv1alpha1.Network
		expectedErr error
	}{
		/*
			"3nodeDefaultDualStackOSPF": {
				path: "data/3node-dualstack-ospf",
				network: netwv1alpha1.BuildNetwork(
					v1.ObjectMeta{
						Name:      "topo3nodesrl.default",
						Namespace: "default",
					},
					&netwv1alpha1.NetworkSpec{
						Topology: "topo3nodesrl",
					},
					nil,
				),
				expectedErr: nil,
			},
		*/

		"3nodeDefaultDualStackEBGP": {
			path: "data/3node-dualstack-ebgp",
			network: netwv1alpha1.BuildNetwork(
				v1.ObjectMeta{
					Name:      "topo3nodesrl.default",
					Namespace: "default",
				},
				&netwv1alpha1.NetworkSpec{
					Topology: "topo3nodesrl",
				},
				nil,
			),
			expectedErr: nil,
		},

		/*
			"3nodeDefaultDualStackISIS": {
				path: "data/3node-dualstack-isis",
				network: netwv1alpha1.BuildNetwork(
					v1.ObjectMeta{
						Name:      "topo3nodesrl.default",
						Namespace: "default",
					},
					&netwv1alpha1.NetworkSpec{
						Topology: "topo3nodesrl",
					},
					nil,
				),
				expectedErr: nil,
			},
		*/
		/*
			"3nodeDefaultIpv6Unnumbered": {
				path: "data/3node-ipv6unnumbered",
				network: netwv1alpha1.BuildNetwork(
					v1.ObjectMeta{
						Name:      "topo3nodesrl.default",
						Namespace: "default",
					},
					&netwv1alpha1.NetworkSpec{
						Topology: "topo3nodesrl",
					},
					nil,
				),
				expectedErr: nil,
			},
			/*
			"3nodeDefaultIpv4Unnumbered": {
				path: "data/3node-ipv4unnumbered",
				network: netwv1alpha1.BuildNetwork(
					v1.ObjectMeta{
						Name:      "topo3nodesrl.default",
						Namespace: "default",
					},
					&netwv1alpha1.NetworkSpec{
						Topology: "topo3nodesrl",
					},
					nil,
				),
				expectedErr: nil,
			},
		*/
		/*
			"3nodeDefaultIpv4only": {
				path: "data/3node-ipv4only",
				network: netwv1alpha1.BuildNetwork(
					v1.ObjectMeta{
						Name:      "topo3nodesrl.default",
						Namespace: "default",
					},
					&netwv1alpha1.NetworkSpec{
						Topology: "topo3nodesrl",
					},
					nil,
				),
				expectedErr: nil,
			},
		*/
		/*
			"3nodeDefaultIpv6only": {
				path: "data/3node-ipv6only",
				network: netwv1alpha1.BuildNetwork(
					v1.ObjectMeta{
						Name:      "topo3nodesrl.default",
						Namespace: "default",
					},
					&netwv1alpha1.NetworkSpec{
						Topology: "topo3nodesrl",
					},
					nil,
				),
				expectedErr: nil,
			},
		*/
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			tctx, err := initializeTestCtx(ctx, tc.path, tc.network)
			if err != nil {
				t.Errorf("cannot initialize test environment, err: %s", err.Error())
				return
			}
			nc := tctx.getNetworkDesign()

			ipclaimList := &ipambev1alpha1.IPClaimList{}
			err = tctx.getClient().List(ctx, ipclaimList)
			assert.NoError(t, err, "cannot initialize test environment")
			for _, ipclaim := range ipclaimList.Items {
				if ipclaim.Status.Prefix != nil {
					fmt.Println("ipclaim prefix", ipclaim.Name, *ipclaim.Status.Prefix)
				}
				if ipclaim.Status.Address != nil {
					fmt.Println("ipclaim address", ipclaim.Name, *ipclaim.Status.Address)
				}
			}

			asclaimList := &asbev1alpha1.ASClaimList{}
			err = tctx.getClient().List(ctx, asclaimList)
			assert.NoError(t, err, "cannot initialize test environment")
			for _, asclaim := range asclaimList.Items {
				if asclaim.Status.ID != nil {
					fmt.Println("asclaim id", asclaim.Name, *asclaim.Status.ID)
				}
				if asclaim.Status.Range != nil {
					fmt.Println("asclaim range", asclaim.Name, *asclaim.Status.Range)
				}
			}

			b := New(tctx.getClient(), types.NamespacedName{Namespace: tc.network.Namespace, Name: tc.network.Name})
			err = b.Build(ctx, tc.network, nc)
			assert.NoError(t, err, "cannot build network configs")
			for _, nd := range b.GetNetworkDeviceConfigs() {
				b, err := yaml.Marshal(nd)
				if err != nil {
					return
				}
				fmt.Println(nd.Name, "\n", string(b))
			}
		})
	}
}

type testCtx struct {
	runScheme *runtime.Scheme
	client    client.Client
	ipambe    bebackend.Backend
	asbe      bebackend.Backend
	nd        *netwv1alpha1.NetworkDesign
}

func initializeTestCtx(ctx context.Context, path string, network *netwv1alpha1.Network) (*testCtx, error) {
	tctx := &testCtx{
		ipambe: ipam.New(nil),
		asbe:   bebackend.New(nil, nil, nil, nil, nil, schema.GroupVersionKind{}, schema.GroupVersionKind{}),
	}
	tctx.initializeSchemasAndClient()
	if err := tctx.initializeTopology(ctx, path); err != nil {
		return nil, err
	}
	if err := tctx.initializeIPIndex(ctx, path); err != nil {
		return nil, err
	}
	if err := tctx.initializeNetworkDesign(ctx, path); err != nil {
		return nil, err
	}
	// first we claim the necessary parameters
	if network.IsDefaultNetwork() {
		if err := tctx.initializeDefaultNetworkParameters(ctx, network); err != nil {
			return nil, err
		}
	}

	return tctx, nil
}

func (r *testCtx) getClient() client.Client {
	return r.client
}

func (r *testCtx) getNetworkDesign() *netwv1alpha1.NetworkDesign {
	return r.nd
}

func (r *testCtx) initializeSchemasAndClient() {
	r.runScheme = runtime.NewScheme()
	infrabev1alpha1.AddToScheme(r.runScheme)
	ipambev1alpha1.AddToScheme(r.runScheme)
	asbev1alpha1.AddToScheme(r.runScheme)
	r.client = fake.NewClientBuilder().WithScheme(r.runScheme).Build()
}

func (r *testCtx) initializeTopology(ctx context.Context, path string) error {
	topo, err := getTopology(filepath.Join(path, topologyFileName))
	if err != nil {
		return err
	}
	clab, err := clab.NewClabKuid(topo.GetSiteID(), *topo.Spec.ContainerLab)
	if err != nil {
		return fmt.Errorf("parsing clab topo failed: %s", err.Error())
	}

	for _, o := range clab.GetNodes(ctx) {
		r.client.Create(ctx, o)

		n := &infrabev1alpha1.Node{}
		_ = r.client.Get(ctx, types.NamespacedName{Namespace: "default", Name: o.GetName()}, n)
		n.Status.SystemID = ptr.To[string]("00:01:02:03:04:05")
		_ = r.client.Update(ctx, n)

	}
	for _, o := range clab.GetLinks(ctx) {
		r.client.Create(ctx, o)
	}
	for _, o := range clab.GetEndpoints(ctx) {
		r.client.Create(ctx, o)
	}
	return nil
}

func (r *testCtx) initializeIPIndex(ctx context.Context, path string) error {
	ipindex, err := getIPIndex(filepath.Join(path, ipindexFileName))
	if err != nil {
		return err
	}

	fmt.Println("ipindex", ipindex.Name)

	if err := r.ipindex(ctx, ipindex); err != nil {
		return err
	}

	for _, prefix := range ipindex.Spec.Prefixes {
		claim, err := ipindex.GetClaim(prefix)
		if err != nil {
			return err
		}
		if err := r.ipambe.Claim(ctx, claim); err != nil {
			return err
		}
	}
	return nil
}

func (r *testCtx) initializeNetworkDesign(ctx context.Context, path string) error {
	var err error
	r.nd, err = getNetworkDesign(filepath.Join(path, networkDesignFileName))
	if err != nil {
		return err
	}

	index := r.nd.GetASIndex()
	if err := r.asindex(ctx, index); err != nil {
		return err
	}

	for _, claim := range r.nd.GetASClaims() {
		if err := r.asclaim(ctx, claim); err != nil {
			return nil
		}
	}
	for _, claim := range r.nd.GetIPClaims() {
		if err := r.ipclaim(ctx, claim); err != nil {
			return nil
		}
	}
	return nil
}

func (r *testCtx) ipindex(ctx context.Context, index *ipambev1alpha1.IPIndex) error {
	if err := r.client.Create(ctx, index); err != nil {
		return err
	}
	if err := r.ipambe.CreateIndex(ctx, index); err != nil {
		return err
	}
	index.SetConditions(conditionv1alpha1.Ready())
	if err := r.client.Update(ctx, index); err != nil {
		return err
	}
	return nil
}

func (r *testCtx) ipclaim(ctx context.Context, claim *ipambev1alpha1.IPClaim) error {
	if err := r.client.Create(ctx, claim); err != nil {
		fmt.Println("ipclaim client create", claim.Name, err)
		return err
	}
	if err := r.ipambe.Claim(ctx, claim); err != nil {
		fmt.Println("ipclaim be claim", claim.Name, err)
		return err
	}
	claim.SetConditions(conditionv1alpha1.Ready())
	if err := r.client.Update(ctx, claim); err != nil {
		fmt.Println("ipclaim client update", claim.Name, err)
		return err
	}
	return nil
}

func (r *testCtx) asindex(ctx context.Context, index *asbev1alpha1.ASIndex) error {
	if err := r.client.Create(ctx, index); err != nil {
		return err
	}
	if err := r.asbe.CreateIndex(ctx, index); err != nil {
		return err
	}
	index.SetConditions(conditionv1alpha1.Ready())
	if err := r.client.Update(ctx, index); err != nil {
		return err
	}
	return nil
}

func (r *testCtx) asclaim(ctx context.Context, claim *asbev1alpha1.ASClaim) error {
	if err := r.client.Create(ctx, claim); err != nil {
		return err
	}
	if err := r.asbe.Claim(ctx, claim); err != nil {
		return err
	}
	claim.SetConditions(conditionv1alpha1.Ready())
	if err := r.client.Update(ctx, claim); err != nil {
		return err
	}
	return nil
}

func (r *testCtx) initializeDefaultNetworkParameters(ctx context.Context, network *netwv1alpha1.Network) error {
	// for each node claim an ip
	nodeList := &infrabev1alpha1.NodeList{}
	if err := r.client.List(ctx, nodeList); err != nil {
		return err
	}

	for _, n := range nodeList.Items {
		if n.Spec.NodeGroup == network.Spec.Topology {
			for _, ipclaim := range r.nd.GetNodeIPClaims(network, &n) {
				if err := r.ipclaim(ctx, ipclaim); err != nil {
					return err
				}
			}

			if asclaim := r.nd.GetNodeASClaim(network, &n); asclaim != nil {
				if err := r.asclaim(ctx, asclaim); err != nil {
					return err
				}
			}
		}
	}
	linkList := &infrabev1alpha1.LinkList{}
	if err := r.client.List(ctx, linkList); err != nil {
		return err
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
			if ep.NodeGroup != network.Spec.Topology {
				continue
			}
		}
		for _, ipclaim := range r.nd.GetLinkIPClaims(network, &l) {
			if err := r.ipclaim(ctx, ipclaim); err != nil {
				return err
			}
		}
	}
	return nil
}

func getTopology(path string) (*topov1alpha1.Topology, error) {
	addToScheme := topov1alpha1.AddToScheme
	obj := &topov1alpha1.Topology{}
	gvk := topov1alpha1.SchemeGroupVersion.WithKind(reflect.TypeOf(obj).Name())
	// build object from file
	if err := testhelper.GetKRMResource(path, obj, gvk, addToScheme); err != nil {
		return nil, err
	}
	return obj, nil
}

func getNetworkDesign(path string) (*netwv1alpha1.NetworkDesign, error) {
	addToScheme := netwv1alpha1.AddToScheme
	obj := &netwv1alpha1.NetworkDesign{}
	gvk := netwv1alpha1.SchemeGroupVersion.WithKind(reflect.TypeOf(obj).Name())
	// build object from file
	if err := testhelper.GetKRMResource(path, obj, gvk, addToScheme); err != nil {
		return nil, err
	}
	return obj, nil
}

func getIPIndex(path string) (*ipambev1alpha1.IPIndex, error) {
	addToScheme := ipambev1alpha1.AddToScheme
	obj := &ipambev1alpha1.IPIndex{}
	gvk := netwv1alpha1.SchemeGroupVersion.WithKind(reflect.TypeOf(obj).Name())
	// build object from file
	if err := testhelper.GetKRMResource(path, obj, gvk, addToScheme); err != nil {
		return nil, err
	}
	return obj, nil
}

/*
func getNodeModel(path string) (*netwv1alpha1., error) {
	addToScheme := netwv1alpha1.AddToScheme
	obj := &netwv1alpha1.Topology{}
	gvk := SchemeGroupVersion.WithKind(reflect.TypeOf(obj).Name())
	// build object from file
	if err := testhelper.GetKRMResource(path, obj, gvk, addToScheme); err != nil {
		return nil, err
	}
	return obj, nil
}
*/
