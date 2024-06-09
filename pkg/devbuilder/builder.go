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
	"strings"

	"github.com/henderiw/iputil"
	"github.com/henderiw/logger/log"
	"github.com/kuidio/kuid/apis/backend"
	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	netwv1alpha1 "github.com/kuidio/kuidapps/apis/network/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func (r *DeviceBuilder) BuildNew(ctx context.Context, network *netwv1alpha1.Network, networkDesign *netwv1alpha1.NetworkDesign) error {
	if network.IsDefaultNetwork() {
		return r.BuildUnderlay(ctx, network, networkDesign)
	}
	return r.BuildOverlay(ctx, network, networkDesign)
}

func (r *DeviceBuilder) BuildUnderlay(ctx context.Context, network *netwv1alpha1.Network, networkDesign *netwv1alpha1.NetworkDesign) error {
	links, err := r.GetLinks(ctx, network)
	if err != nil {
		return err
	}
	nodes := sets.New[string]()
	for _, link := range links {
		l, err := r.gatherLinkInfo(ctx, networkDesign, link)
		if err != nil {
			return nil
		}
		r.updateUnderlayDeviceConfig(ctx, network.GetNetworkName(), networkDesign, l)
	}
}

func (r *DeviceBuilder) gatherLinkInfo(ctx context.Context, networkDesign *netwv1alpha1.NetworkDesign, link *infrabev1alpha1.Link) (*link, error) {
	l := newLink(link)
	// gather IP and if EBGP is enabled AS numbers
	for i := uint(0); i < 2; i++ {
		nodeID := link.Spec.Endpoints[i].NodeID
		nodeName := nodeID.Node
		epName := link.Spec.Endpoints[i].Endpoint
		l.addNodeID(i, nodeID)
		l.addNodeName(i, nodeName)
		l.addEPName(i, epName)
		if networkDesign.IsEBGPEnabled() {
			as, err := r.getASClaim(ctx, types.NamespacedName{
				Namespace: r.nsn.Namespace,
				Name:      fmt.Sprintf("%s.%s", r.nsn.Name, nodeName),
			})
			if err != nil {
				return l, err
			}
			l.addAS(i, as)
		}
		if networkDesign.IsUnderlayIPv4Numbered() {
			localEPNodeName := fmt.Sprintf("%s.%s.%s.ipv4", r.nsn.Name, nodeName, epName)

			ipv4, err := r.getIPClaim(ctx, types.NamespacedName{
				Namespace: r.nsn.Namespace,
				Name:      localEPNodeName,
			})
			if err != nil {
				return l, err
			}
			l.addIpv4(i, ipv4)
		}
		if networkDesign.IsUnderlayIPv6Numbered() {
			localEPNodeName := fmt.Sprintf("%s.%s.%s.ipv6", r.nsn.Name, nodeName, epName)

			ipv6, err := r.getIPClaim(ctx, types.NamespacedName{
				Namespace: r.nsn.Namespace,
				Name:      localEPNodeName,
			})
			if err != nil {
				return l, err
			}
			l.addIpv6(i, ipv6)
		}
	}
	return l, nil
}

func (r *DeviceBuilder) updateUnderlayDeviceConfig(ctx context.Context, niName string, networkDesign *netwv1alpha1.NetworkDesign, l *link) error {
	for i := uint(0); i < 2; i++ {
		//nodeID := l.getNodeID(i)
		nodeName := l.getNodeName(i)
		epName := l.getEPName(i)
		//j := i ^ 1
		//peerNodeID := l.getNodeID(j)
		//peerNodeName := l.getNodeName(j)
		//peerEPName := l.getEPName(i)

		r.devices.AddInterface(nodeName, &netwv1alpha1.NetworkDeviceInterface{
			Name: epName,
			//Speed: "100G", -> to be changed and look at the inventory
		})
		id := uint32(0)
		r.devices.AddSubInterface(nodeName, epName, l.getUnderlaySubInterface(i, networkDesign, id))
		r.devices.AddNetworkInstanceSubInterface(nodeName, netwv1alpha1.DefaultNetwork, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
			Name: epName,
			ID:   id,
		})
		// BFD needs to be enabled on the protocols first
		// we ignore BFD enabled on underlay
		if networkDesign.IsBFDEnabled() {
			bfdParmas := l.getBFDLinkParameters(i, networkDesign)
			if bfdParmas.Enabled != nil && *bfdParmas.Enabled {
				r.devices.AddBFDInterface(nodeName, &netwv1alpha1.NetworkDeviceBFDInterface{
					SubInterfaceName: netwv1alpha1.NetworkDeviceNetworkInstanceInterface{Name: epName, ID: id},
					BFD:              *bfdParmas,
				})
			}
		}
		if networkDesign.IsOSPFEnabled() {
			r.updateUnderlayOSPFDeviceConfig(i, nodeName, epName, niName, id, networkDesign, l)
		}
		if networkDesign.IsISISEnabled() {
			r.updateUnderlayISISDeviceConfig(i, nodeName, epName, niName, id, networkDesign, l)
		}

	}
	return nil
}

func (r *DeviceBuilder) updateUnderlayOSPFDeviceConfig(i uint, nodeName, epName, niName string, id uint32, networkDesign *netwv1alpha1.NetworkDesign, l *link) {
	instanceName := networkDesign.GetOSPFInstanceName()
	r.devices.AddNetworkInstanceProtocolsOSPFInstance(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolOSPFInstance{
		Name:    instanceName,
		Version: networkDesign.GetOSPFVersion(),
		//RouterID:     routerID,
		MaxECMPPaths: networkDesign.GetOSPFGetMaxECMPLPaths(),
		//ASBR: nd.Spec.Protocols.OSPF.ASBR, TODO
	})
	area := l.getOSPFArea(i, networkDesign)
	r.devices.AddNetworkInstanceProtocolsOSPFInstanceArea(nodeName, niName, instanceName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolOSPFInstanceArea{
		Name: area,
		NSSA: nil, // TODO
		Stub: nil, // TODO
	})
	r.devices.AddNetworkInstanceProtocolsOSPFInstanceAreaInterface(nodeName, niName, instanceName, area, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolOSPFInstanceAreaInterface{
		SubInterfaceName: netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
			Name: epName,
			ID:   id,
		},
		NetworkType: infrabev1alpha1.NetworkTypeP2P,
		Passive:     l.getOSPFPassive(i, networkDesign),
		BFD:         l.getOSPFBFD(i, networkDesign),
	})
}

func (r *DeviceBuilder) updateUnderlayISISDeviceConfig(i uint, nodeName, epName, niName string, id uint32, networkDesign *netwv1alpha1.NetworkDesign, l *link) {
	instanceName := networkDesign.GetISISInstanceName()
	r.devices.AddNetworkInstanceProtocolsISISInstance(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolISISInstance{
		Name:            instanceName,
		LevelCapability: networkDesign.GetISISLevel(),
		Net:             getISISNetIDs(networkDesign.GetISISAreas(), l.getSystemID(i)),
		MaxECMPPaths:    networkDesign.GetISISGetMaxECMPLPaths(),
		AddressFamilies: networkDesign.GetIGPAddressFamilies(),
	})
	r.devices.AddNetworkInstanceProtocolsISISInstanceInterface(nodeName, niName, instanceName, l.getISISInterface(i, networkDesign, id))
}

func (r *DeviceBuilder) BuildOverlay(ctx context.Context, network *netwv1alpha1.Network, networkDesign *netwv1alpha1.NetworkDesign) error {
	return nil
}

func (r *DeviceBuilder) Build(ctx context.Context, cr *netwv1alpha1.Network, nd *netwv1alpha1.NetworkDesign) error {
	//log := log.FromContext(ctx)
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
			r.devices.AddProvider(nodeName, n.Spec.Provider)
			if nd.IsVXLANEnabled() {
				r.devices.AddTunnelInterface(nodeName, &netwv1alpha1.NetworkDeviceTunnelInterface{
					Name: VXLANInterfaceName,
				})
			}
			if nd.IsBGPEVPNEnabled() {
				r.devices.AddSystemProtocolsBGPVPN(nodeName, &netwv1alpha1.NetworkDeviceSystemProtocolsBGPVPN{})
			}
			// add IRB interface
			r.devices.AddInterface(nodeName, &netwv1alpha1.NetworkDeviceInterface{
				Name: IRBInterfaceName,
			})

			r.UpdateNetworkInstance(ctx, nodeName, niName, netwv1alpha1.NetworkInstanceType_DEFAULT)
			r.UpdateRoutingPolicies(ctx, nodeName, cr.IsDefaultNetwork(), nd)
			r.devices.AddNetworkInstanceProtocolsBGPAddressFamilies(nodeName, niName, nd.GetAllEnabledAddressFamilies())
			if err := r.UpdateNodeAS(ctx, nodeName, niName, nd); err != nil {
				return err
			}

			if n.Status.SystemID == nil {
				return fmt.Errorf("cannot update node parameters without a systemID")
			}

			if err := r.UpdateNodeIP(ctx, nodeName, niName, *n.Status.SystemID, nd); err != nil {
				return err
			}
		}

		if err := r.UpdateInterfaces(ctx, niName, nd, links, nodes); err != nil {
			return err
		}

		// to do the peer mesh we need to collect all edge peers for the rr to
		// know all the peers
		edgePeers := sets.New[string]()
		for _, n := range nodes {
			nodeName := n.Spec.NodeID.Node
			// this is waiting for information from the previous steps
			// e.g. routerID, AS numbers, etc
			netwworkDeviceType, ok := n.Spec.UserDefinedLabels.Labels[backend.KuidINVNetworkDeviceType]
			if !ok {
				return nil
			}

			if netwworkDeviceType == "edge" || netwworkDeviceType == "pe" {
				var err error
				if err = r.UpdateProtocolsEdge(ctx, nodeName, niName, nd, edgePeers); err != nil {
					return err
				}
			}
		}
		for _, n := range nodes {
			nodeName := n.Spec.NodeID.Node
			// this is waiting for information from the previous steps
			// e.g. routerID, AS numbers, etc
			netwworkDeviceType, ok := n.Spec.UserDefinedLabels.Labels[backend.KuidINVNetworkDeviceType]
			if !ok {
				return nil
			}
			if edgePeers.Len() != 0 && netwworkDeviceType != "edge" && netwworkDeviceType != "pe" {
				if err := r.UpdateProtocolsRR(ctx, nodeName, niName, nd, edgePeers); err != nil {
					return err
				}
			}
		}
	} else {
		bd2NodeName := map[string]sets.Set[string]{}
		for _, bridge := range cr.Spec.Bridges {
			bd2NodeName[bridge.Name] = sets.New[string]()
			// add bridge domain
			bdName := fmt.Sprintf("%s.%s", networkName, bridge.Name)
			id := uint32(bridge.NetworkID)
			for _, itfce := range bridge.Interfaces {
				if !itfce.IsDynamic() {
					// static interface
					// check if the endpoint exists in the inventory
					if itfce.EndPoint == nil {
						return fmt.Errorf("cannot have a bridge domain without a specific endpoint defined")
					}
					if itfce.NodeID == nil {
						return fmt.Errorf("cannot have a bridge domain without a specific node id defined")
					}
					nodeName := itfce.Node
					endpointID := infrabev1alpha1.EndpointID{
						Endpoint: *itfce.EndPoint,
						NodeID:   *itfce.NodeID,
					}
					ep, err := r.getEndpoint(ctx, types.NamespacedName{
						Namespace: r.nsn.Namespace,
						Name:      fmt.Sprintf("%s.%s", cr.Spec.Topology, endpointID.KuidString()),
					})
					if err != nil {
						return err
					}
					bd2NodeName[bridge.Name].Insert(nodeName)

					r.devices.AddProvider(itfce.Node, ep.Spec.Provider)

					vlan := ptr.To[uint32](id)
					if itfce.VLANID != nil {
						*vlan = *itfce.VLANID
					}
					r.devices.AddSubInterface(nodeName, ep.Spec.Endpoint, &netwv1alpha1.NetworkDeviceInterfaceSubInterface{
						PeerName: "customer",
						ID:       id,
						Type:     netwv1alpha1.SubInterfaceType_Bridged,
						VLAN:     vlan,
						// no ip addresses required since this is bridged
					})

					r.devices.AddNetworkInstance(nodeName, &netwv1alpha1.NetworkDeviceNetworkInstance{
						Name: bdName,
						Type: netwv1alpha1.NetworkInstanceType_MACVRF,
					})
					r.devices.AddNetworkInstanceSubInterface(nodeName, bdName, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
						Name: ep.Spec.Endpoint,
						ID:   id,
					})
					if nd.IsVXLANEnabled() {
						r.devices.AddTunnelSubInterface(nodeName, VXLANInterfaceName, &netwv1alpha1.NetworkDeviceTunnelInterfaceSubInterface{
							ID:   id,
							Type: netwv1alpha1.SubInterfaceType_Bridged,
						})
						r.devices.AddNetworkInstanceSubInterfaceVXLAN(nodeName, bdName, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
							Name: VXLANInterfaceName,
							ID:   id,
						})
					}
					r.devices.AddNetworkInstanceProtocolsBGPEVPN(nodeName, bdName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPEVPN{
						EVI:            id,
						ECMP:           2,
						VXLANInterface: VXLANInterfaceName,
					})
					r.devices.AddNetworkInstanceProtocolsBGPVPN(nodeName, bdName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPVPN{
						ImportRouteTarget: fmt.Sprintf("target:%d:%d", nd.GetIBGPAS(), id),
						ExportRouteTarget: fmt.Sprintf("target:%d:%d", nd.GetIBGPAS(), id),
					})
					continue
				}
			}
		}
		for _, router := range cr.Spec.Routers {
			niName := fmt.Sprintf("%s.%s", networkName, router.Name)
			id := uint32(router.NetworkID)
			for _, itfce := range router.Interfaces {
				if !itfce.IsDynamic() {
					if itfce.Bridge == nil && itfce.EndPoint == nil {
						return fmt.Errorf("cannot create an rt interface w/o a bridgedomain or endpoint specified")
					}

					if itfce.NodeID == nil {
						return fmt.Errorf("unsupported for now")
						// use the nodes collected by the bridge domain logic
						// need to split the ip prefix per node
					}

					nodeName := itfce.Node
					if itfce.EndPoint == nil && itfce.Bridge == nil {
						return fmt.Errorf("cannot create a interface w/o no endpoint and brdigedomain")
					}

					if itfce.Bridge == nil {
						// static interface
						// check if the endpoint exists in the inventory
						endpointID := infrabev1alpha1.EndpointID{
							Endpoint: *itfce.EndPoint,
							NodeID:   *itfce.NodeID,
						}
						ep, err := r.getEndpoint(ctx, types.NamespacedName{
							Namespace: r.nsn.Namespace,
							Name:      fmt.Sprintf("%s.%s", cr.Spec.Topology, endpointID.KuidString()),
						})
						if err != nil {
							return err
						}
						r.devices.AddProvider(itfce.Node, ep.Spec.Provider)
					} else {
						n, err := r.getNode(ctx, types.NamespacedName{
							Namespace: r.nsn.Namespace,
							Name:      fmt.Sprintf("%s.%s", cr.Spec.Topology, itfce.NodeID.KuidString()),
						})
						if err != nil {
							return err
						}
						r.devices.AddProvider(itfce.Node, n.Spec.Provider)
					}

					r.devices.AddNetworkInstance(nodeName, &netwv1alpha1.NetworkDeviceNetworkInstance{
						Name: niName,
						Type: netwv1alpha1.NetworkInstanceType_IPVRF,
					})
					if nd.IsVXLANEnabled() {
						r.devices.AddTunnelSubInterface(nodeName, VXLANInterfaceName, &netwv1alpha1.NetworkDeviceTunnelInterfaceSubInterface{
							ID:   id,
							Type: netwv1alpha1.SubInterfaceType_Routed,
						})
						r.devices.AddNetworkInstanceSubInterfaceVXLAN(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
							Name: VXLANInterfaceName,
							ID:   id,
						})
					}
					r.devices.AddNetworkInstanceProtocolsBGPEVPN(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPEVPN{
						EVI:            id,
						ECMP:           2,
						VXLANInterface: VXLANInterfaceName,
					})
					r.devices.AddNetworkInstanceProtocolsBGPVPN(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPVPN{
						ImportRouteTarget: fmt.Sprintf("target:%d:%d", nd.GetIBGPAS(), id),
						ExportRouteTarget: fmt.Sprintf("target:%d:%d", nd.GetIBGPAS(), id),
					})

					vlan := ptr.To[uint32](id)
					if itfce.VLANID != nil {
						*vlan = *itfce.VLANID
					}

					ipv4 := []string{}
					ipv6 := []string{}
					for _, address := range itfce.Addresses {
						pi, err := iputil.New(address.Address)
						if err != nil {
							continue
						}
						if pi.IsIpv4() {
							ipv4 = append(ipv4, address.Address)
						} else {
							ipv6 = append(ipv6, address.Address)
						}
					}
					if itfce.Bridge == nil {
						r.devices.AddSubInterface(nodeName, *itfce.EndPoint, &netwv1alpha1.NetworkDeviceInterfaceSubInterface{
							PeerName: "customer",
							ID:       id,
							Type:     netwv1alpha1.SubInterfaceType_Routed,
							VLAN:     vlan,
							IPv4: &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv4{
								Addresses: ipv4,
							},
							IPv6: &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv6{
								Addresses: ipv6,
							},
						})
						r.devices.AddNetworkInstanceSubInterface(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
							Name: *itfce.EndPoint,
							ID:   id,
						})
					} else {
						// irb
						if !cr.IsBridgePresent(*itfce.Bridge) {
							return fmt.Errorf("cannot create an irb interface on a bridgedomain that is not present")
						}
						r.devices.AddSubInterface(nodeName, IRBInterfaceName, &netwv1alpha1.NetworkDeviceInterfaceSubInterface{
							PeerName: "customer",
							ID:       id,
							//Type:     netwv1alpha1.SubInterfaceType_Routed,// not type exepected
							IPv4: &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv4{
								Addresses: ipv4,
							},
							IPv6: &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv6{
								Addresses: ipv6,
							},
						})
						r.devices.AddNetworkInstanceSubInterface(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
							Name: IRBInterfaceName,
							ID:   id,
						})
						// add the irb to the bridge domain as well
						bdName := fmt.Sprintf("%s.%s", networkName, *itfce.Bridge)
						r.devices.AddNetworkInstanceSubInterface(nodeName, bdName, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
							Name: IRBInterfaceName,
							ID:   id,
						})
					}
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

func (r *DeviceBuilder) UpdateNodeAS(ctx context.Context, nodeName, niName string, nd *netwv1alpha1.NetworkDesign) error {
	//nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
	//networkName := cr.GetNetworkName()
	if nd.IsEBGPEnabled() {
		as, err := r.getASClaim(ctx, types.NamespacedName{
			Namespace: r.nsn.Namespace,
			Name:      fmt.Sprintf("%s.%s", r.nsn.Name, nodeName),
		})
		if err != nil {
			return err
		}
		r.devices.AddNetworkInstanceProtocolsBGPAS(nodeName, niName, as)
	} else {
		r.devices.AddNetworkInstanceProtocolsBGPAS(nodeName, niName, nd.GetIBGPAS())
	}
	return nil
}

func (r *DeviceBuilder) UpdateNodeIP(ctx context.Context, nodeName, niName, systemID string, nd *netwv1alpha1.NetworkDesign) error {
	ipv4 := []string{}
	ipv6 := []string{}
	routerID := ""
	if nd.IsLoopbackIPv4Enabled() {
		ip, err := r.getIPClaim(ctx, types.NamespacedName{
			Namespace: r.nsn.Namespace,
			Name:      fmt.Sprintf("%s.%s.ipv4", r.nsn.Name, nodeName),
		})
		if err != nil {
			return err
		}
		ipv4 = append(ipv4, ip)

		pi, _ := iputil.New(ip)
		routerID = pi.GetIPAddress().String()
		r.devices.AddNetworkInstanceProtocolsBGPRouterID(nodeName, niName, routerID)
	} else {
		ip, err := r.getIPClaim(ctx, types.NamespacedName{
			Namespace: r.nsn.Namespace,
			Name:      fmt.Sprintf("%s.%s.routerid", r.nsn.Name, nodeName),
		})
		if err != nil {
			return err
		}
		routerID = ip
		r.devices.AddNetworkInstanceProtocolsBGPRouterID(nodeName, niName, routerID)
	}
	if nd.IsLoopbackIPv6Enabled() {
		ip, err := r.getIPClaim(ctx, types.NamespacedName{
			Namespace: r.nsn.Namespace,
			Name:      fmt.Sprintf("%s.%s.ipv6", r.nsn.Name, nodeName),
		})
		if err != nil {
			return err
		}
		ipv6 = append(ipv6, ip)
	}
	// system interface
	r.devices.AddInterface(nodeName, &netwv1alpha1.NetworkDeviceInterface{
		Name: SystemInterfaceName,
	})
	r.devices.AddSubInterface(nodeName, SystemInterfaceName, &netwv1alpha1.NetworkDeviceInterfaceSubInterface{
		ID: 0,
		//SubInterfaceType: netwv1alpha1.SubInterfaceType_Routed, A type is not possible here
		IPv4: &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv4{Addresses: ipv4},
		IPv6: &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv6{Addresses: ipv6},
	})
	r.devices.AddNetworkInstanceSubInterface(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
		Name: SystemInterfaceName,
		ID:   0,
	})

	if nd.IsISISEnabled() {
		instanceName := DefaultIGPINstance
		if nd.Spec.Protocols.ISIS.Instance != nil {
			instanceName = *nd.Spec.Protocols.ISIS.Instance
		}

		r.devices.AddNetworkInstanceProtocolsISISInstance(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolISISInstance{
			Name:            instanceName,
			LevelCapability: nd.Spec.Protocols.ISIS.LevelCapability,
			Net:             getISISNetIDs(nd.Spec.Protocols.ISIS.Areas, systemID),
			MaxECMPPaths:    nd.Spec.Protocols.ISIS.MaxECMPPaths,
			AddressFamilies: nd.GetIGPAddressFamilies(),
		})
		isisItfce := &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolISISInstanceInterface{
			SubInterfaceName: netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
				Name: SystemInterfaceName,
				ID:   0,
			},
		}
		isisItfce.Level = 2
		if nd.Spec.Protocols.ISIS.LevelCapability == netwv1alpha1.NetworkDesignProtocolsISISLevelCapabilityL1 {
			isisItfce.Level = 1
		}
		isisItfce.Passive = true
		r.devices.AddNetworkInstanceProtocolsISISInstanceInterface(nodeName, niName, instanceName, isisItfce)
	}
	if nd.IsOSPFEnabled() {
		instanceName := DefaultIGPINstance
		if nd.Spec.Protocols.OSPF.Instance != nil {
			instanceName = *nd.Spec.Protocols.OSPF.Instance
		}
		r.devices.AddNetworkInstanceProtocolsOSPFInstance(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolOSPFInstance{
			Name:         instanceName,
			Version:      nd.Spec.Protocols.OSPF.Version,
			RouterID:     routerID,
			MaxECMPPaths: nd.Spec.Protocols.OSPF.MaxECMPPaths,
			//ASBR: nd.Spec.Protocols.OSPF.ASBR, TODO
		})

		area := nd.Spec.Protocols.OSPF.Area
		r.devices.AddNetworkInstanceProtocolsOSPFInstanceArea(nodeName, niName, instanceName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolOSPFInstanceArea{
			Name: area,
			NSSA: nil, // TODO
			Stub: nil, // TODO
			//ASBR: nd.Spec.Protocols.OSPF.ASBR, TODO
		})
		r.devices.AddNetworkInstanceProtocolsOSPFInstanceAreaInterface(nodeName, niName, instanceName, area, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolOSPFInstanceAreaInterface{
			SubInterfaceName: netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
				Name: SystemInterfaceName,
				ID:   0,
			},
			Passive: true,
			BFD:     nd.Spec.Interfaces.ISL.BFD,
		})

	}
	return nil
}

func getISISNetIDs(areas []string, systemID string) []string {
	nets := make([]string, 0, len(areas))
	// Split the MAC address into parts
	parts := strings.Split(systemID, ":")
	if len(parts) != 6 {
		return nets
	}
	for _, area := range areas {
		nets = append(nets, fmt.Sprintf("%s.%s.00", area, fmt.Sprintf("%s%s.%s%s.%s%s", parts[0], parts[1], parts[2], parts[3], parts[4], parts[5])))
	}
	return nets
}

func (r *DeviceBuilder) UpdateInterfaces(ctx context.Context, niName string, nd *netwv1alpha1.NetworkDesign, links []*infrabev1alpha1.Link, nodes []*infrabev1alpha1.Node) error {
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
			if nd.IsEBGPEnabled() {
				as[i], err = r.getASClaim(ctx, types.NamespacedName{
					Namespace: r.nsn.Namespace,
					Name:      fmt.Sprintf("%s.%s", r.nsn.Name, nodeName),
				})
				if err != nil {
					return err
				}
			}
			if nd.IsUnderlayIPv4Numbered() {
				localEPNodeName := fmt.Sprintf("%s.%s.%s.ipv4", r.nsn.Name, nodeName, epName)

				usedipv4[i], err = r.getIPClaim(ctx, types.NamespacedName{
					Namespace: r.nsn.Namespace,
					Name:      localEPNodeName,
				})
				if err != nil {
					return err
				}
			}
			if nd.IsUnderlayIPv6Numbered() {
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

			r.devices.AddInterface(nodeName, &netwv1alpha1.NetworkDeviceInterface{
				Name: epName,
				//Speed: "100G", -> to be changed and look at the inventory
			})
			si := &netwv1alpha1.NetworkDeviceInterfaceSubInterface{
				PeerName: fmt.Sprintf("%s.%s", peerNodeName, peerEPName),
				ID:       0,
				Type:     netwv1alpha1.SubInterfaceType_Routed,
			}
			if nd.IsUnderlayIPv4Numbered() {
				si.IPv4 = &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv4{
					Addresses: []string{usedipv4[i]},
				}
			}
			if nd.IsUnderlayIPv4UnNumbered() {
				si.IPv4 = &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv4{}
			}
			if nd.IsUnderlayIPv6Numbered() {
				si.IPv6 = &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv6{
					Addresses: []string{usedipv6[i]},
				}
			}
			if nd.IsUnderlayIPv6UnNumbered() {
				si.IPv6 = &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv6{}
			}
			r.devices.AddSubInterface(nodeName, epName, si)

			r.devices.AddNetworkInstanceSubInterface(nodeName, netwv1alpha1.DefaultNetwork, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
				Name: epName,
				ID:   0,
			})

			if nd.IsOSPFEnabled() {
				instanceName := DefaultIGPINstance
				if nd.Spec.Protocols.OSPF.Instance != nil {
					instanceName = *nd.Spec.Protocols.OSPF.Instance
				}
				/*
					r.devices.AddNetworkInstanceProtocolsOSPFInstance(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolOSPFInstance{
						Name:    instanceName,
						Version: nd.Spec.Protocols.OSPF.Version,
						//RouterID:     routerID,
						MaxECMPPaths: nd.Spec.Protocols.OSPF.MaxECMPPaths,
						//ASBR: nd.Spec.Protocols.OSPF.ASBR, TODO
					})
				*/
				area := nd.Spec.Protocols.OSPF.Area
				r.devices.AddNetworkInstanceProtocolsOSPFInstanceArea(nodeName, niName, instanceName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolOSPFInstanceArea{
					Name: area,
					NSSA: nil, // TODO
					Stub: nil, // TODO
					//ASBR: nd.Spec.Protocols.OSPF.ASBR, TODO
				})
				r.devices.AddNetworkInstanceProtocolsOSPFInstanceAreaInterface(nodeName, niName, instanceName, area, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolOSPFInstanceAreaInterface{
					SubInterfaceName: netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
						Name: epName,
						ID:   0,
					},
					Type: ptr.To[netwv1alpha1.OSPFInterfaceType](netwv1alpha1.OSPFInterfaceTypeP2P),
					BFD:  nd.Spec.Interfaces.ISL.BFD,
				})
			}
			if nd.IsISISEnabled() {
				instanceName := DefaultIGPINstance
				if nd.Spec.Protocols.ISIS.Instance != nil {
					instanceName = *nd.Spec.Protocols.ISIS.Instance
				}
				r.devices.AddNetworkInstanceProtocolsISISInstance(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolISISInstance{
					Name:            instanceName,
					LevelCapability: nd.Spec.Protocols.ISIS.LevelCapability,
					Net:             getISISNetIDs(nd.Spec.Protocols.ISIS.Areas, getNodeSystemID(nodes, nodeName)),
					MaxECMPPaths:    nd.Spec.Protocols.ISIS.MaxECMPPaths,
					AddressFamilies: nd.GetIGPAddressFamilies(),
				})
				isisItfce := &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolISISInstanceInterface{
					SubInterfaceName: netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
						Name: epName,
						ID:   0,
					},
					//Type: netwv1alpha1.ISISInterfaceTypeP2P,
					Type: ptr.To[netwv1alpha1.ISISInterfaceType](netwv1alpha1.ISISInterfaceTypeP2P),
				}
				isisItfce.Level = 2
				if nd.Spec.Protocols.ISIS.LevelCapability == netwv1alpha1.NetworkDesignProtocolsISISLevelCapabilityL1 {
					isisItfce.Level = 1
				}
				if nd.IsISLIPv4Enabled() {
					isisItfce.IPv4 = &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolISISInstanceInterfaceIPv4{
						BFD: nd.Spec.Interfaces.ISL.BFD,
					}
				}
				if nd.IsISLIPv6Enabled() {
					isisItfce.IPv6 = &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolISISInstanceInterfaceIPv6{
						BFD: nd.Spec.Interfaces.ISL.BFD,
					}
				}

				r.devices.AddNetworkInstanceProtocolsISISInstanceInterface(nodeName, niName, instanceName, isisItfce)
			}

			if nd.IsEBGPEnabled() {
				if nd.IsISLIPv4Enabled() {
					if nd.IsISLIPv4Numbered() {
						pii, _ := iputil.New(usedipv4[i])
						pij, _ := iputil.New(usedipv4[j])
						r.devices.AddNetworkInstanceprotocolsBGPNeighbor(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
							LocalAddress: pii.GetIPAddress().String(),
							PeerAddress:  pij.GetIPAddress().String(),
							PeerGroup:    BGPUnderlayPeerGroupName,
							LocalAS:      as[i],
							PeerAS:       as[j],
						})
					} else {
						r.devices.AddNetworkInstanceprotocolsBGPDynamicNeighbor(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighborsInterface{
							SubInterfaceName: netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
								Name: epName,
								ID:   0,
							},
							PeerAS:    as[j],
							PeerGroup: BGPUnderlayPeerGroupName,
						})
					}
				}
				if nd.IsISLIPv6Enabled() {
					if nd.IsISLIPv6Numbered() {
						pii, _ := iputil.New(usedipv6[i])
						pij, _ := iputil.New(usedipv6[j])
						r.devices.AddNetworkInstanceprotocolsBGPNeighbor(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
							LocalAddress: pii.GetIPAddress().String(),
							PeerAddress:  pij.GetIPAddress().String(),
							PeerGroup:    BGPUnderlayPeerGroupName,
							LocalAS:      as[i],
							PeerAS:       as[j],
						})

					} else {
						r.devices.AddNetworkInstanceprotocolsBGPDynamicNeighbor(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighborsInterface{
							SubInterfaceName: netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
								Name: epName,
								ID:   0,
							},
							PeerAS:    as[j],
							PeerGroup: BGPUnderlayPeerGroupName,
						})
					}
				}
				r.devices.AddNetworkInstanceprotocolsBGPPeerGroup(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
					Name:            BGPUnderlayPeerGroupName,
					AddressFamilies: nd.GetUnderlayAddressFamiliesToBeDisabled(),
				})
			}
		}
	}
	return nil
}

func getNodeSystemID(nodes []*infrabev1alpha1.Node, nodeName string) string {
	for _, node := range nodes {
		if node.Spec.NodeID.Node == nodeName {
			if node.Status.SystemID != nil {
				return *node.Status.SystemID
			}
		}
	}
	return ""
}

func (r *DeviceBuilder) UpdateProtocolsDynamicNeighbors(ctx context.Context, nodeName, networkName string, nd *netwv1alpha1.NetworkDesign, nodeLabels map[string]string) error {
	log := log.FromContext(ctx)
	//nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
	//nodeName := nodeID.Node
	//networkName := cr.GetNetworkName()
	if nd.IsIBGPEnabled() {
		r.devices.AddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
			Name:            BGPOverlayPeerGroupName,
			AddressFamilies: nd.GetOverlayAddressFamiliesToBeDisabled(),
		})

		fullNodeName := fmt.Sprintf("%s.%s", r.nsn.Name, nodeName)
		fullNodeNameIPv4 := fmt.Sprintf("%s.ipv4", fullNodeName)
		fullNodeNameIPv6 := fmt.Sprintf("%s.ipv6", fullNodeName)

		for _, rrNodeNameAF := range nd.Spec.Protocols.IBGP.RouteReflectors {
			rrIP, err := r.getIPClaim(ctx, types.NamespacedName{
				Namespace: r.nsn.Namespace,
				Name:      rrNodeNameAF,
			})
			if err != nil {
				return err
			}

			peerPrefix, _ := iputil.New(rrIP)

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
				r.devices.AddNetworkInstanceprotocolsBGPNeighbor(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
					LocalAddress: localAddress,
					PeerAddress:  peerPrefix.GetIPAddress().String(),
					PeerGroup:    BGPOverlayPeerGroupName,
					LocalAS:      nd.GetIBGPAS(),
					PeerAS:       nd.GetIBGPAS(),
				})
			}

			log.Info("rrNodeName", "rrNodeNameAF", rrNodeNameAF, "ipv4", fullNodeNameIPv4)

			if rrNodeNameAF == fullNodeNameIPv4 || rrNodeNameAF == fullNodeNameIPv6 {
				// this is the rr - we add the peer group again as this cannot harm
				r.devices.AddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
					Name:            BGPOverlayPeerGroupName,
					AddressFamilies: nd.GetOverlayAddressFamiliesToBeDisabled(),
					RouteReflector: &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroupRouteReflector{
						ClusterID: rrIP,
					},
				})

				r.devices.AddNetworkInstanceprotocolsBGPDynamicNeighbor(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighborsInterface{
					SubInterfaceName: netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
						Name: SystemInterfaceName,
						ID:   0,
					},
					PeerAS:    nd.GetIBGPAS(),
					PeerGroup: BGPOverlayPeerGroupName,
				})
			}
		}
	}
	return nil
}

func (r *DeviceBuilder) UpdateProtocolsRR(ctx context.Context, nodeName, networkName string, nd *netwv1alpha1.NetworkDesign, edgePeers sets.Set[string]) error {
	//log := log.FromContext(ctx)
	if nd.IsIBGPEnabled() {
		r.devices.AddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
			Name:            BGPOverlayPeerGroupName,
			AddressFamilies: nd.GetOverlayAddressFamiliesToBeDisabled(),
		})

		fullNodeName := fmt.Sprintf("%s.%s", r.nsn.Name, nodeName)
		fullNodeNameIPv4 := fmt.Sprintf("%s.ipv4", fullNodeName)
		fullNodeNameIPv6 := fmt.Sprintf("%s.ipv6", fullNodeName)

		for _, rrNodeNameAF := range nd.Spec.Protocols.IBGP.RouteReflectors {
			rrIP, err := r.getIPClaim(ctx, types.NamespacedName{
				Namespace: r.nsn.Namespace,
				Name:      rrNodeNameAF,
			})
			if err != nil {
				return err
			}

			peerPrefix, _ := iputil.New(rrIP)

			localAddress := ""
			systemPrefix := r.devices.GetSystemIP(nodeName, SystemInterfaceName, 0, peerPrefix.IsIpv4())
			if systemPrefix != "" {
				pi, _ := iputil.New(systemPrefix)
				localAddress = pi.GetIPAddress().String()
			}
			if rrNodeNameAF == fullNodeNameIPv4 || rrNodeNameAF == fullNodeNameIPv6 {
				// this is the rr - we add the peer group again as this cannot harm
				r.devices.AddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
					Name:            BGPOverlayPeerGroupName,
					AddressFamilies: nd.GetOverlayAddressFamiliesToBeDisabled(),
					RouteReflector: &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroupRouteReflector{
						ClusterID: peerPrefix.GetIPAddress().String(),
					},
				})

				for _, peerIP := range edgePeers.UnsortedList() {
					r.devices.AddNetworkInstanceprotocolsBGPNeighbor(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
						LocalAddress: localAddress,
						PeerAddress:  peerIP,
						PeerGroup:    BGPOverlayPeerGroupName,
						LocalAS:      nd.GetIBGPAS(),
						PeerAS:       nd.GetIBGPAS(),
					})
				}
			}
		}
	}
	return nil
}

func (r *DeviceBuilder) UpdateProtocolsEdge(ctx context.Context, nodeName, networkName string, nd *netwv1alpha1.NetworkDesign, edgePeers sets.Set[string]) error {
	if nd.IsIBGPEnabled() {
		r.devices.AddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
			Name:            BGPOverlayPeerGroupName,
			AddressFamilies: nd.GetOverlayAddressFamiliesToBeDisabled(),
		})

		for _, rrNodeNameAF := range nd.Spec.Protocols.IBGP.RouteReflectors {
			rrIP, err := r.getIPClaim(ctx, types.NamespacedName{
				Namespace: r.nsn.Namespace,
				Name:      rrNodeNameAF,
			})
			if err != nil {
				return err
			}

			peerPrefix, _ := iputil.New(rrIP)

			localAddress := ""
			systemPrefix := r.devices.GetSystemIP(nodeName, SystemInterfaceName, 0, peerPrefix.IsIpv4())
			if systemPrefix == "" {
				return fmt.Errorf("no local address configured")
			}
			pi, _ := iputil.New(systemPrefix)
			localAddress = pi.GetIPAddress().String()
			edgePeers.Insert(localAddress)

			r.devices.AddNetworkInstanceprotocolsBGPNeighbor(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
				LocalAddress: localAddress,
				PeerAddress:  peerPrefix.GetIPAddress().String(),
				PeerGroup:    BGPOverlayPeerGroupName,
				LocalAS:      nd.GetIBGPAS(),
				PeerAS:       nd.GetIBGPAS(),
			})
		}
	}
	return nil
}

func (r *DeviceBuilder) UpdateRoutingPolicies(ctx context.Context, nodeName string, isDefaultNetwork bool, nd *netwv1alpha1.NetworkDesign) error {
	if isDefaultNetwork {
		ipv4, ipv6 := nd.GetLoopbackPrefixesPerAF()
		r.devices.AddRoutingPolicy(nodeName, "underlay", ipv4, ipv6)
		r.devices.AddRoutingPolicy(nodeName, "overlay", nil, nil)
	}
	return nil
}
