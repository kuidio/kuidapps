package devbuilder

import (
	"context"
	"fmt"

	"github.com/henderiw/iputil"
	"github.com/kuidio/kuid/apis/backend"
	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	netwv1alpha1 "github.com/kuidio/kuidapps/apis/network/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	IRBInterfaceName         = "irb0"
	SystemInterfaceName      = "system0"
	BGPUnderlayPeerGroupName = "underlay"
	BGPOverlayPeerGroupName  = "overlay"
)

func New(client client.Client, nsn types.NamespacedName) *DeviceBuilder {
	return &DeviceBuilder{
		Client:  client,
		devices: NewDevices(nsn),
	}
}

type DeviceBuilder struct {
	client.Client
	devices *Devices
}

func (r *DeviceBuilder) GetNetworkDeviceConfigs() []*netwv1alpha1.NetworkDevice {
	return r.devices.GetNetworkDeviceConfigs()
}

func (r *DeviceBuilder) Build(ctx context.Context, cr *netwv1alpha1.Network, nc *netwv1alpha1.NetworkConfig) error {
	nodes, err := r.GetNodes(ctx, cr)
	if err != nil {
		return err
	}
	var links []*infrabev1alpha1.Link
	if cr.IsDefaultNetwork() {
		links, err = r.GetLinks(ctx, cr)
		if err != nil {
			return err
		}
	}

	for _, n := range nodes {
		r.devices.AddProvider(n.Spec.Node, n.Spec.Provider)
		if cr.IsDefaultNetwork() {
			r.UpdateNetworkInstance(ctx, cr, n, netwv1alpha1.NetworkInstanceType_DEFAULT)

			r.UpdateRoutingPolicies(ctx, cr, nc, n)
		}

		if err := r.UpdateNodeAS(ctx, cr, nc, n); err != nil {
			return err
		}
		if err := r.UpdateNodeIP(ctx, cr, nc, n); err != nil {
			return err
		}
	}
	if cr.IsDefaultNetwork() {
		if err := r.UpdateInterfaces(ctx, cr, nc, links); err != nil {
			return err
		}
	}

	for _, n := range nodes {
		if err := r.UpdateProtocols(ctx, cr, nc, n); err != nil {
			return err
		}
	}

	return nil
}

func (r *DeviceBuilder) UpdateNetworkInstance(ctx context.Context, cr *netwv1alpha1.Network, n *infrabev1alpha1.Node, t netwv1alpha1.NetworkInstanceType) {
	nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
	nodeName := nodeID.Node
	networkName := cr.GetNetworkName()
	r.devices.AddNetworkInstance(nodeName, &netwv1alpha1.NetworkDeviceNetworkInstance{
		Name:                networkName,
		NetworkInstanceType: t,
	})
}

func (r *DeviceBuilder) UpdateNodeAS(ctx context.Context, cr *netwv1alpha1.Network, nc *netwv1alpha1.NetworkConfig, n *infrabev1alpha1.Node) error {
	nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
	networkName := cr.GetNetworkName()
	if nc.IsEBGPEnabled() {
		as, err := r.getASClaim(ctx, types.NamespacedName{
			Namespace: cr.GetNamespace(),
			Name:      fmt.Sprintf("%s.%s", cr.Name, nodeID.Node),
		})
		if err != nil {
			return err
		}
		r.devices.AddAS(nodeID.Node, networkName, as)
	} else {
		r.devices.AddAS(nodeID.Node, networkName, nc.GetIBGPAS())
	}
	return nil
}

func (r *DeviceBuilder) UpdateNodeIP(ctx context.Context, cr *netwv1alpha1.Network, nc *netwv1alpha1.NetworkConfig, n *infrabev1alpha1.Node) error {
	nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
	nodeName := nodeID.Node
	networkName := cr.GetNetworkName()
	ipv4 := []string{}
	ipv6 := []string{}
	if nc.IsIPv4Enabled() {
		ip, err := r.getIPClaim(ctx, types.NamespacedName{
			Namespace: cr.GetNamespace(),
			Name:      fmt.Sprintf("%s.%s.ipv4", cr.Name, nodeName),
		})
		if err != nil {
			return err
		}
		ipv4 = append(ipv4, ip)

		pi, _ := iputil.New(ip)
		r.devices.AddRouterID(nodeID.Node, networkName, pi.GetIPAddress().String())
	} else {
		ip, err := r.getIPClaim(ctx, types.NamespacedName{
			Namespace: cr.GetNamespace(),
			Name:      fmt.Sprintf("%s.%s.routerid", cr.Name, nodeName),
		})
		if err != nil {
			return err
		}
		r.devices.AddRouterID(nodeID.Node, networkName, ip)
	}
	if nc.IsIPv6Enabled() {
		ip, err := r.getIPClaim(ctx, types.NamespacedName{
			Namespace: cr.GetNamespace(),
			Name:      fmt.Sprintf("%s.%s.ipv6", cr.Name, nodeName),
		})
		if err != nil {
			return err
		}
		ipv6 = append(ipv6, ip)
	}
	r.devices.AddInterface(nodeName, &netwv1alpha1.NetworkDeviceInterface{
		Name: SystemInterfaceName,
	})
	r.devices.AddSubInterface(nodeName, SystemInterfaceName, &netwv1alpha1.NetworkDeviceInterfaceSubInterface{
		ID: 0,
		//SubInterfaceType: netwv1alpha1.SubInterfaceType_Routed, A type is not possible here
	}, ipv4, ipv6)
	r.devices.AddNetworkInstanceSubInterface(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
		Name: SystemInterfaceName,
		ID:   0,
	})
	return nil
}

func (r *DeviceBuilder) UpdateInterfaces(ctx context.Context, cr *netwv1alpha1.Network, nc *netwv1alpha1.NetworkConfig, links []*infrabev1alpha1.Link) error {
	networkName := cr.GetNetworkName()
	for _, l := range links {
		as := make([]uint32, 2)
		usedipv4 := make([]string, 2)
		usedipv6 := make([]string, 2)

		// gather IP and if EBGP is enabled AS numbers
		for i := 0; i < 2; i++ {
			nodeID := l.Spec.Endpoints[i].NodeID
			nodeName := nodeID.Node
			epName := l.Spec.Endpoints[i].Endpoint
			var err error
			if nc.IsEBGPEnabled() {
				as[i], err = r.getASClaim(ctx, types.NamespacedName{
					Namespace: cr.GetNamespace(),
					Name:      fmt.Sprintf("%s.%s", cr.Name, nodeName),
				})
				if err != nil {
					return err
				}
			}
			if nc.IsIPv4Enabled() {
				localEPNodeName := fmt.Sprintf("%s.%s.%s.ipv4", cr.Name, nodeName, epName)

				usedipv4[i], err = r.getIPClaim(ctx, types.NamespacedName{
					Namespace: cr.GetNamespace(),
					Name:      localEPNodeName,
				})
				if err != nil {
					return err
				}
			}
			if nc.IsIPv6Enabled() {
				localEPNodeName := fmt.Sprintf("%s.%s.%s.ipv6", cr.Name, nodeName, epName)

				usedipv6[i], err = r.getIPClaim(ctx, types.NamespacedName{
					Namespace: cr.GetNamespace(),
					Name:      localEPNodeName,
				})
				if err != nil {
					return err
				}
			}
		}

		// apply the retrieved information to the spec
		for i := 0; i < 2; i++ {
			nodeID := l.Spec.Endpoints[i].NodeID
			nodeName := nodeID.Node
			epName := l.Spec.Endpoints[i].Endpoint
			j := i ^ 1
			peerNodeID := l.Spec.Endpoints[j].NodeID
			peerNodeName := peerNodeID.Node
			peerEPName := l.Spec.Endpoints[j].Endpoint

			var ipv4 []string
			var ipv6 []string
			if nc.IsIPv4Enabled() {
				ipv4 = []string{usedipv4[i]}
			}
			if nc.IsIPv4Enabled() {
				ipv6 = []string{usedipv6[i]}
			}

			r.devices.AddInterface(nodeName, &netwv1alpha1.NetworkDeviceInterface{
				Name: epName,
				//Speed: "100G", -> to be changed and look at the inventory
			})
			r.devices.AddSubInterface(nodeName, epName, &netwv1alpha1.NetworkDeviceInterfaceSubInterface{
				PeerName:         fmt.Sprintf("%s.%s", peerNodeName, peerEPName),
				ID:               0,
				SubInterfaceType: netwv1alpha1.SubInterfaceType_Routed,
			}, ipv4, ipv6)

			r.devices.AddNetworkInstanceSubInterface(nodeName, netwv1alpha1.DefaultNetwork, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
				Name: epName,
				ID:   0,
			})

			if nc.IsEBGPEnabled() {
				afs := []string{}
				if nc.IsIPv4Enabled() {
					pii, _ := iputil.New(usedipv4[i])
					pij, _ := iputil.New(usedipv4[j])
					r.devices.AddBGPNeighbor(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
						LocalAddress: pii.GetIPAddress().String(),
						PeerAddress:  pij.GetIPAddress().String(),
						PeerGroup:    BGPUnderlayPeerGroupName,
						LocalAS:      as[i],
						PeerAS:       as[j],
					})
					afs = append(afs, "ipv4-unicast")
				}
				if nc.IsIPv6Enabled() {
					pii, _ := iputil.New(usedipv6[i])
					pij, _ := iputil.New(usedipv6[j])
					r.devices.AddBGPNeighbor(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
						LocalAddress: pii.GetIPAddress().String(),
						PeerAddress:  pij.GetIPAddress().String(),
						PeerGroup:    BGPUnderlayPeerGroupName,
						LocalAS:      as[i],
						PeerAS:       as[j],
					})
					afs = append(afs, "ipv6-unicast")
				}
				r.devices.AddBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
					Name:            BGPUnderlayPeerGroupName,
					AddressFamilies: afs,
				})
			}
		}
	}
	return nil
}

func (r *DeviceBuilder) UpdateProtocols(ctx context.Context, cr *netwv1alpha1.Network, nc *netwv1alpha1.NetworkConfig, n *infrabev1alpha1.Node) error {
	nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
	nodeName := nodeID.Node
	networkName := cr.GetNetworkName()
	if nc.IsIBGPEnabled() {
		r.devices.AddBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
			Name:            BGPOverlayPeerGroupName,
			AddressFamilies: []string{"evpn"},
		})

		for _, rrNodeName := range nc.Spec.Protocols.IBGP.RouteReflectors {
			peerIP, err := r.getIPClaim(ctx, types.NamespacedName{
				Namespace: cr.GetNamespace(),
				Name:      rrNodeName,
			})
			if err != nil {
				return err
			}

			peerPrefix, _ := iputil.New(peerIP)

			localAddress := ""
			systemPrefix := r.devices.GetSystemIP(nodeName, SystemInterfaceName, 0, peerPrefix.IsIpv4())
			if systemPrefix != "" {
				pi, _ := iputil.New(systemPrefix)
				localAddress = pi.GetIPAddress().String()
			}

			netwworkDeviceType, ok := n.Spec.UserDefinedLabels.Labels[backend.KuidINVNetworkDeviceType]
			if !ok {
				return nil
			}
			if netwworkDeviceType == "edge" || netwworkDeviceType == "pe" {
				r.devices.AddBGPNeighbor(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
					LocalAddress: localAddress,
					PeerAddress:  peerPrefix.GetIPAddress().String(),
					PeerGroup:    BGPOverlayPeerGroupName,
					LocalAS:      *nc.Spec.Protocols.IBGP.AS,
					PeerAS:       *nc.Spec.Protocols.IBGP.AS,
				})
			}
		}
	}
	return nil
}

func (r *DeviceBuilder) UpdateRoutingPolicies(ctx context.Context, cr *netwv1alpha1.Network, nc *netwv1alpha1.NetworkConfig, n *infrabev1alpha1.Node) error {
	nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
	nodeName := nodeID.Node
	if cr.IsDefaultNetwork() {
		ipv4, ipv6 := nc.GetLoopbackPrefixes()
		r.devices.AddRoutingPolicy(nodeName, "underlay", ipv4, ipv6)
		r.devices.AddRoutingPolicy(nodeName, "overlay", nil, nil)
	}
	return nil
}
