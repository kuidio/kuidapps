package devbuilder

import (
	"context"
	"fmt"

	"github.com/henderiw/iputil"
	"github.com/kuidio/kuid/apis/backend"
	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	netwv1alpha1 "github.com/kuidio/kuidapps/apis/network/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	VXLANInterfaceName       = "vxlan0"
	IRBInterfaceName         = "irb0"
	SystemInterfaceName      = "system0"
	BGPUnderlayPeerGroupName = "underlay"
	BGPOverlayPeerGroupName  = "overlay"
)

func New(client client.Client, nsn types.NamespacedName) *DeviceBuilder {
	return &DeviceBuilder{
		Client:  client,
		devices: NewDevices(nsn),
		nsn:     nsn,
	}
}

type DeviceBuilder struct {
	client.Client
	devices *Devices
	nsn     types.NamespacedName
}

func (r *DeviceBuilder) GetNetworkDeviceConfigs() []*netwv1alpha1.NetworkDevice {
	return r.devices.GetNetworkDeviceConfigs()
}

func (r *DeviceBuilder) Build(ctx context.Context, cr *netwv1alpha1.Network, nc *netwv1alpha1.NetworkConfig) error {
	networkName := cr.GetNetworkName()
	if cr.IsDefaultNetwork() {
		niName := networkName
		nodes, err := r.GetNodes(ctx, cr)
		if err != nil {
			return err
		}

		links, err := r.GetLinks(ctx, cr)
		if err != nil {
			return err
		}

		for _, n := range nodes {
			nodeName := n.Spec.NodeID.Node
			//nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
			//nodeName := nodeID.Node

			r.devices.AddProvider(nodeName, n.Spec.Provider)
			// TODO check encap
			if nc.IsVXLANEnabled() {
				r.devices.AddTunnelInterface(nodeName, &netwv1alpha1.NetworkDeviceTunnelInterface{
					Name: VXLANInterfaceName,
				})
			}
			if nc.ISBGPEVPNEnabled() {
				r.devices.AddSystemProtocolsBGPVPN(nodeName, &netwv1alpha1.NetworkDeviceSystemProtocolsBGPVPN{})
			}

			r.UpdateNetworkInstance(ctx, nodeName, niName, netwv1alpha1.NetworkInstanceType_DEFAULT)
			r.UpdateRoutingPolicies(ctx, nodeName, cr.IsDefaultNetwork(), nc)
			if err := r.UpdateNodeAS(ctx, nodeName, niName, nc); err != nil {
				return err
			}
			if err := r.UpdateNodeIP(ctx, nodeName, niName, nc); err != nil {
				return err
			}
		}

		if err := r.UpdateInterfaces(ctx, niName, nc, links); err != nil {
			return err
		}

		for _, n := range nodes {
			nodeName := n.Spec.NodeID.Node
			// this is waiting for information from the previous steps
			// e.g. routerID, AS numbers, etc
			if err := r.UpdateProtocols(ctx, nodeName, niName, nc, n.Spec.UserDefinedLabels.Labels); err != nil {
				return err
			}
		}
	} else {
		for _, bd := range cr.Spec.BridgeDomains {
			// add bridge domain
			bdName := fmt.Sprintf("%s.%s", networkName, bd.Name)
			id := uint32(bd.NetworkID)
			for _, itfce := range bd.Interfaces {
				if !itfce.IsDynamic() {
					nodeName := itfce.Node
					// static interface
					// check if the endpoint exists in the inventory
					ep, err := r.getEndpoint(ctx, types.NamespacedName{
						Namespace: r.nsn.Namespace,
						Name:      fmt.Sprintf("%s.%s", cr.Spec.Topology, itfce.EndpointID.KuidString()),
					})
					if err != nil {
						return err
					}
					r.devices.AddProvider(itfce.Node, ep.Spec.Provider)
					r.devices.AddSubInterface(nodeName, ep.Spec.Endpoint, &netwv1alpha1.NetworkDeviceInterfaceSubInterface{
						PeerName: "customer",
						ID:       id,
						Type:     netwv1alpha1.SubInterfaceType_Bridged,
						VLAN:     ptr.To[uint32](id),
					}, nil, nil)

					r.devices.AddNetworkInstance(nodeName, &netwv1alpha1.NetworkDeviceNetworkInstance{
						Name: bdName,
						Type: netwv1alpha1.NetworkInstanceType_MACVRF,
					})
					r.devices.AddNetworkInstanceSubInterface(nodeName, bdName, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
						Name: ep.Spec.Endpoint,
						ID:   id,
					})
					if nc.IsVXLANEnabled() {
						r.devices.AddTunnelSubInterface(nodeName, VXLANInterfaceName, &netwv1alpha1.NetworkDeviceTunnelInterfaceSubInterface{
							ID:   id,
							Type: netwv1alpha1.SubInterfaceType_Bridged,
						})
						r.devices.AddNetworkInstanceSubInterfaceVXLAN(nodeName, bdName, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
							Name: VXLANInterfaceName,
							ID:   id,
						})
					}

					r.devices.AddBGPEVPN(nodeName, bdName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPEVPN{
						EVI:            id,
						ECMP:           2,
						VXLANInterface: VXLANInterfaceName,
					})
					r.devices.AddBGPVPN(nodeName, bdName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPVPN{
						ImportRouteTarget: fmt.Sprintf("target:%d:%d", nc.GetIBGPAS(), id),
						ExportRouteTarget: fmt.Sprintf("target:%d:%d", nc.GetIBGPAS(), id),
					})

					continue
				}
			}
		}
	}

	return nil
}

func (r *DeviceBuilder) UpdateNetworkInstance(ctx context.Context, nodeName, niName string, t netwv1alpha1.NetworkInstanceType) {
	r.devices.AddNetworkInstance(nodeName, &netwv1alpha1.NetworkDeviceNetworkInstance{
		Name: niName,
		Type: t,
	})
}

func (r *DeviceBuilder) UpdateNodeAS(ctx context.Context, nodeName, niName string, nc *netwv1alpha1.NetworkConfig) error {
	//nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
	//networkName := cr.GetNetworkName()
	if nc.IsEBGPEnabled() {
		as, err := r.getASClaim(ctx, types.NamespacedName{
			Namespace: r.nsn.Namespace,
			Name:      fmt.Sprintf("%s.%s", r.nsn.Name, nodeName),
		})
		if err != nil {
			return err
		}
		r.devices.AddAS(nodeName, niName, as)
	} else {
		r.devices.AddAS(nodeName, niName, nc.GetIBGPAS())
	}
	return nil
}

func (r *DeviceBuilder) UpdateNodeIP(ctx context.Context, nodeName, niName string, nc *netwv1alpha1.NetworkConfig) error {
	//nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
	//nodeName := nodeID.Node
	//networkName := cr.GetNetworkName()
	ipv4 := []string{}
	ipv6 := []string{}
	if nc.IsIPv4Enabled() {
		ip, err := r.getIPClaim(ctx, types.NamespacedName{
			Namespace: r.nsn.Namespace,
			Name:      fmt.Sprintf("%s.%s.ipv4", r.nsn.Name, nodeName),
		})
		if err != nil {
			return err
		}
		ipv4 = append(ipv4, ip)

		pi, _ := iputil.New(ip)
		r.devices.AddRouterID(nodeName, niName, pi.GetIPAddress().String())
	} else {
		ip, err := r.getIPClaim(ctx, types.NamespacedName{
			Namespace: r.nsn.Namespace,
			Name:      fmt.Sprintf("%s.%s.routerid", r.nsn.Name, nodeName),
		})
		if err != nil {
			return err
		}
		r.devices.AddRouterID(nodeName, niName, ip)
	}
	if nc.IsIPv6Enabled() {
		ip, err := r.getIPClaim(ctx, types.NamespacedName{
			Namespace: r.nsn.Namespace,
			Name:      fmt.Sprintf("%s.%s.ipv6", r.nsn.Name, nodeName),
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
	r.devices.AddNetworkInstanceSubInterface(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
		Name: SystemInterfaceName,
		ID:   0,
	})
	return nil
}

func (r *DeviceBuilder) UpdateInterfaces(ctx context.Context, niName string, nc *netwv1alpha1.NetworkConfig, links []*infrabev1alpha1.Link) error {
	//networkName := cr.GetNetworkName()
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
					Namespace: r.nsn.Namespace,
					Name:      fmt.Sprintf("%s.%s", r.nsn.Name, nodeName),
				})
				if err != nil {
					return err
				}
			}
			if nc.IsIPv4Enabled() {
				localEPNodeName := fmt.Sprintf("%s.%s.%s.ipv4", r.nsn.Name, nodeName, epName)

				usedipv4[i], err = r.getIPClaim(ctx, types.NamespacedName{
					Namespace: r.nsn.Namespace,
					Name:      localEPNodeName,
				})
				if err != nil {
					return err
				}
			}
			if nc.IsIPv6Enabled() {
				localEPNodeName := fmt.Sprintf("%s.%s.%s.ipv6", r.nsn.Name, nodeName, epName)

				usedipv6[i], err = r.getIPClaim(ctx, types.NamespacedName{
					Namespace: r.nsn.Namespace,
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
				PeerName: fmt.Sprintf("%s.%s", peerNodeName, peerEPName),
				ID:       0,
				Type:     netwv1alpha1.SubInterfaceType_Routed,
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
					r.devices.AddBGPNeighbor(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
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
					r.devices.AddBGPNeighbor(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
						LocalAddress: pii.GetIPAddress().String(),
						PeerAddress:  pij.GetIPAddress().String(),
						PeerGroup:    BGPUnderlayPeerGroupName,
						LocalAS:      as[i],
						PeerAS:       as[j],
					})
					afs = append(afs, "ipv6-unicast")
				}
				r.devices.AddBGPPeerGroup(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
					Name:            BGPUnderlayPeerGroupName,
					AddressFamilies: afs,
				})
			}
		}
	}
	return nil
}

func (r *DeviceBuilder) UpdateProtocols(ctx context.Context, nodeName, networkName string, nc *netwv1alpha1.NetworkConfig, nodeLabels map[string]string) error {
	//nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
	//nodeName := nodeID.Node
	//networkName := cr.GetNetworkName()
	if nc.IsIBGPEnabled() {
		r.devices.AddBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
			Name:            BGPOverlayPeerGroupName,
			AddressFamilies: []string{"evpn"},
		})

		for _, rrNodeName := range nc.Spec.Protocols.IBGP.RouteReflectors {
			peerIP, err := r.getIPClaim(ctx, types.NamespacedName{
				Namespace: r.nsn.Namespace,
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

			netwworkDeviceType, ok := nodeLabels[backend.KuidINVNetworkDeviceType]
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

func (r *DeviceBuilder) UpdateRoutingPolicies(ctx context.Context, nodeName string, isDefaultNetwork bool, nc *netwv1alpha1.NetworkConfig) error {
	if isDefaultNetwork {
		ipv4, ipv6 := nc.GetLoopbackPrefixes()
		r.devices.AddRoutingPolicy(nodeName, "underlay", ipv4, ipv6)
		r.devices.AddRoutingPolicy(nodeName, "overlay", nil, nil)
	}
	return nil
}
