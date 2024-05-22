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
			//return reconcile.Result{Requeue: true}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
			return reconcile.Result{Requeue: true}, nil
		}

		if err := r.finalizer.RemoveFinalizer(ctx, cr); err != nil {
			r.handleError(ctx, cr, "cannot remove finalizer", err)
			//return ctrl.Result{Requeue: true}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
			return ctrl.Result{Requeue: true}, nil
		}
		log.Debug("Successfully deleted resource")
		return ctrl.Result{}, nil
	}

	if err := r.finalizer.AddFinalizer(ctx, cr); err != nil {
		r.handleError(ctx, cr, "cannot add finalizer", err)
		//return ctrl.Result{Requeue: true}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		return ctrl.Result{Requeue: true}, nil
	}

	if cr.GetCondition(conditionv1alpha1.ConditionTypeReady).Status == metav1.ConditionFalse {
		//return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
		return ctrl.Result{Requeue: true}, nil
	}

	defaultNetwork := isDefaultNetwork(cr.Name)
	nc, err := r.getNetworkConfig(ctx, cr)
	if err != nil {
		if defaultNetwork {
			// we need networkconfig for the default network
			// we do not release resources at this stage -> decision do far is no
			r.handleError(ctx, cr, "network config not provided, needed for a default network", nil)
			//return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		if nc.GetCondition(conditionv1alpha1.ConditionTypeReady).Status == metav1.ConditionFalse {
			r.handleError(ctx, cr, "network config not ready", nil)
			//return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
			return ctrl.Result{Requeue: true}, nil
		}
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

	cr.SetConditions(conditionv1alpha1.Ready())
	r.recorder.Eventf(cr, corev1.EventTypeNormal, crName, "ready")
	return ctrl.Result{}, perrors.Wrap(r.Client.Status().Update(ctx, cr), errUpdateStatus)
}

func (r *reconciler) handleError(ctx context.Context, cr *netwv1alpha1.Network, msg string, err error) {
	log := log.FromContext(ctx)
	if err == nil {
		//cr.SetConditions(conditionv1alpha1.Failed(msg))
		log.Error(msg)
		r.recorder.Eventf(cr, corev1.EventTypeWarning, crName, msg)
	} else {
		//cr.SetConditions(conditionv1alpha1.Failed(err.Error()))
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

	var links []*infrabev1alpha1.Link
	var err error
	if isDefaultNetwork(cr.Name) {
		links, err = r.GetLinks(ctx, cr)
		if err != nil {
			return err
		}
	}

	nodes, err := r.GetNodes(ctx, cr)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		nd, err := r.getNetworkDeviceConfig(ctx, cr, nc, n, links)
		if err != nil {
			return err
		}
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

func getNetworkName(crName string) string {
	parts := strings.Split(crName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
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
/*
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
*/

func (r *reconciler) getNetworkDeviceConfig(
	ctx context.Context,
	cr *netwv1alpha1.Network,
	nc *netwv1alpha1.NetworkConfig,
	n *infrabev1alpha1.Node,
	links []*infrabev1alpha1.Link,
) (*netwv1alpha1.NetworkDevice, error) {

	deviceSpec := &netwv1alpha1.NetworkDeviceSpec{}
	nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())

	name := getNetworkName(cr.Name)
	if name == netwv1alpha1.DefaultNetwork {
		nodeIPClaimName := fmt.Sprintf("%s.%s.ipv4", cr.Name, nodeID.Node)
		if !nc.IsIPv4Enabled() {
			nodeIPClaimName = fmt.Sprintf("%s.%s.routerid", cr.Name, nodeID.Node)
		}
		routerID, err := r.getIPClaim(ctx, types.NamespacedName{
			Name:      nodeIPClaimName,
			Namespace: cr.GetNamespace()})
		if err != nil {
			return nil, err
		}

		links, err := r.getInterfaces(ctx, cr, nc, n, links)
		if err != nil {
			return nil, err
		}
		deviceSpec.Interfaces = make([]*netwv1alpha1.NetworkDeviceInterface, 0, len(links))

		interfaces := make([]string, 0, len(links))
		for _, l := range links {
			var ipv4 []*netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv4
			if l.ipv4 != nil {
				ipv4 = append(ipv4, &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv4{
					Prefix: l.ipv4.localIP,
				})
			}
			var ipv6 []*netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv6
			if l.ipv6 != nil {
				ipv6 = append(ipv6, &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv6{
					Prefix: l.ipv6.localIP,
				})
			}
			deviceSpec.Interfaces = append(deviceSpec.Interfaces, &netwv1alpha1.NetworkDeviceInterface{
				Name:          l.epName,
				InterfaceType: "regular",
				SubInterfaces: []netwv1alpha1.NetworkDeviceInterfaceSubInterface{
					{
						ID:               l.id,
						SubInterfaceType: "routed",
						IPv4:             ipv4,
						IPv6:             ipv6,
					},
				},
			})
			interfaces = append(interfaces, l.epName)
		}

		var protocols *netwv1alpha1.NetworkDeviceNetworkInstanceProtocols
		if nc.Spec.Protocols != nil {
			protocols = &netwv1alpha1.NetworkDeviceNetworkInstanceProtocols{}
			var bgp *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGP

			as, err := r.getASClaim(ctx, types.NamespacedName{
				Namespace: cr.GetNamespace(),
				Name:      fmt.Sprintf("%s.%s", cr.Name, nodeID.Node),
			})
			if err != nil {
				return nil, err
			}

			neighbors := []*netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{}
			for _, l := range links {
				if l.ipv4 != nil {
					neighbors = append(neighbors, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
						PeerAddress:  l.ipv6.peerIP,
						LocalAddress: l.ipv6.localIP,
						PeerAS:       l.peerAS,
						PeerGroup:    "underlay",
					})
				}
				if l.ipv6 != nil {
					neighbors = append(neighbors, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
						PeerAddress:  l.ipv6.peerIP,
						LocalAddress: l.ipv6.localIP,
						PeerAS:       l.peerAS,
						PeerGroup:    "underlay",
					})
				}
			}
			if nc.Spec.Protocols.IBGP != nil {
				if strings.Contains(nodeID.Node, "edge") {
					for _, rrName := range nc.Spec.Protocols.IBGP.RouteReflectors {
						address, err := r.getIPClaim(ctx, types.NamespacedName{
							Namespace: cr.GetNamespace(),
							Name:      rrName,
						})
						if err != nil {
							return nil, err
						}
						neighbors = append(neighbors, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
							PeerAddress:  address,
							LocalAddress: routerID,
							PeerAS:       *nc.Spec.Protocols.IBGP.AS,
							PeerGroup:    "overlay",
						})
					}
				}
			}

			bgp = &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGP{
				RouterID: routerID,
				AS:       as,
				PeerGroups: []*netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
					{
						Name:            "underlay",
						AddressFamilies: []string{"ipv4", "ipv6"},
					},
					{
						Name:            "overlay",
						AddressFamilies: []string{"evpn"},
					},
				},
				Neighbors: neighbors,
			}

			protocols.BGP = bgp
		}

		deviceSpec.NetworkInstances = []*netwv1alpha1.NetworkDeviceNetworkInstance{
			{
				Name:                netwv1alpha1.DefaultNetwork,
				NetworkInstanceType: netwv1alpha1.NetworkInstanceType_DEFAULT,
				Protocols:           protocols,
				Interfaces:          interfaces,
			},
		}

		return netwv1alpha1.BuildNetworkDevice(
			metav1.ObjectMeta{
				Namespace: cr.GetNamespace(),
				Name:      fmt.Sprintf("%s.%s", cr.Name, nodeID.Node),
			},
			deviceSpec,
			nil,
		), nil
	}

	return nil, fmt.Errorf("not supported")
}

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

type link struct {
	epName  string
	id      uint32
	localAS uint32
	peerAS  uint32
	ipv4    *linkIP
	ipv6    *linkIP
}

type linkIP struct {
	localIP string
	peerIP  string
}

func (r *reconciler) getInterfaces(
	ctx context.Context,
	cr *netwv1alpha1.Network,
	nc *netwv1alpha1.NetworkConfig,
	n *infrabev1alpha1.Node,
	links []*infrabev1alpha1.Link,
) ([]link, error) {
	nodeLinks := []link{}
	for _, l := range links {
		found := false
		localID := 0
		if l.Spec.Endpoints[0].NodeID == n.Spec.NodeID {
			found = true
			localID = 0
		}
		if l.Spec.Endpoints[1].NodeID == n.Spec.NodeID {
			found = true
			localID = 1
		}
		if found {
			link := link{
				epName: l.Spec.Endpoints[localID].Endpoint,
				id:     0,
			}
			remoteID := localID ^ 1

			var err error
			link.peerAS, err = r.getASClaim(ctx, types.NamespacedName{
				Namespace: cr.GetNamespace(),
				Name:      fmt.Sprintf("%s.%s", cr.Name, l.Spec.Endpoints[remoteID].Node),
			})
			if err != nil {
				return nil, err
			}
			link.localAS = 0

			if nc.IsIPv4Enabled() {
				link.ipv4 = &linkIP{}
				localEPNodeName := fmt.Sprintf("%s.%s.%s.ipv4", cr.Name, l.Spec.Endpoints[localID].Node, l.Spec.Endpoints[localID].Endpoint)

				link.ipv4.localIP, err = r.getIPClaim(ctx, types.NamespacedName{
					Namespace: cr.GetNamespace(),
					Name:      localEPNodeName,
				})
				if err != nil {
					return nil, err
				}
				peerEPNodeName := fmt.Sprintf("%s.%s.%s.ipv4", cr.Name, l.Spec.Endpoints[remoteID].Node, l.Spec.Endpoints[remoteID].Endpoint)
				link.ipv4.peerIP, err = r.getIPClaim(ctx, types.NamespacedName{
					Namespace: cr.GetNamespace(),
					Name:      peerEPNodeName,
				})
				if err != nil {
					return nil, err
				}
			}
			if nc.IsIPv6Enabled() {
				link.ipv6 = &linkIP{}
				localEPNodeName := fmt.Sprintf("%s.%s.%s.ipv6", cr.Name, l.Spec.Endpoints[localID].Node, l.Spec.Endpoints[localID].Endpoint)
				link.ipv6.localIP, err = r.getIPClaim(ctx, types.NamespacedName{
					Namespace: cr.GetNamespace(),
					Name:      localEPNodeName,
				})
				if err != nil {
					return nil, err
				}
				peerEPNodeName := fmt.Sprintf("%s.%s.%s.ipv6", cr.Name, l.Spec.Endpoints[remoteID].Node, l.Spec.Endpoints[remoteID].Endpoint)
				link.ipv6.peerIP, err = r.getIPClaim(ctx, types.NamespacedName{
					Namespace: cr.GetNamespace(),
					Name:      peerEPNodeName,
				})
				if err != nil {
					return nil, err
				}
			}
			nodeLinks = append(nodeLinks, link)
		}
	}
	return nodeLinks, nil
}

func (r *reconciler) getIPClaim(ctx context.Context, key types.NamespacedName) (string, error) {
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

func (r *reconciler) getASClaim(ctx context.Context, nsn types.NamespacedName) (uint32, error) {
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
