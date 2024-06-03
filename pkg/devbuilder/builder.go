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

const (
	VXLANInterfaceName       = "vxlan0"
	IRBInterfaceName         = "irb0"
	SystemInterfaceName      = "system0"
	BGPUnderlayPeerGroupName = "underlay"
	BGPOverlayPeerGroupName  = "overlay"
	DefaultIGPINstance       = "i1"
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

func (r *DeviceBuilder) Build(ctx context.Context, cr *netwv1alpha1.Network, nc *netwv1alpha1.NetworkDesign) error {
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
			if nc.IsVXLANEnabled() {
				r.devices.AddTunnelInterface(nodeName, &netwv1alpha1.NetworkDeviceTunnelInterface{
					Name: VXLANInterfaceName,
				})
			}
			if nc.ISBGPEVPNEnabled() {
				r.devices.AddSystemProtocolsBGPVPN(nodeName, &netwv1alpha1.NetworkDeviceSystemProtocolsBGPVPN{})
			}
			// add IRB interface
			r.devices.AddInterface(nodeName, &netwv1alpha1.NetworkDeviceInterface{
				Name: IRBInterfaceName,
			})

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
				if err = r.UpdateProtocolsEdge(ctx, nodeName, niName, nc, edgePeers); err != nil {
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
				if err := r.UpdateProtocolsRR(ctx, nodeName, niName, nc, edgePeers); err != nil {
					return err
				}
			}
		}
	} else {
		bd2NodeName := map[string]sets.Set[string]{}

		for _, bd := range cr.Spec.Bridges {
			bd2NodeName[bd.Name] = sets.New[string]()
			// add bridge domain
			bdName := fmt.Sprintf("%s.%s", networkName, bd.Name)
			id := uint32(bd.NetworkID)
			for _, itfce := range bd.Interfaces {
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
					bd2NodeName[bd.Name].Insert(nodeName)

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
					r.devices.AddNetworkInstanceProtocolsBGPEVPN(nodeName, bdName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPEVPN{
						EVI:            id,
						ECMP:           2,
						VXLANInterface: VXLANInterfaceName,
					})
					r.devices.AddNetworkInstanceProtocolsBGPVPN(nodeName, bdName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPVPN{
						ImportRouteTarget: fmt.Sprintf("target:%d:%d", nc.GetIBGPAS(), id),
						ExportRouteTarget: fmt.Sprintf("target:%d:%d", nc.GetIBGPAS(), id),
					})
					continue
				}
			}
		}
		for _, ni := range cr.Spec.Routers {
			niName := fmt.Sprintf("%s.%s", networkName, ni.Name)
			id := uint32(ni.NetworkID)
			for _, itfce := range ni.Interfaces {
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
					if nc.IsVXLANEnabled() {
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
						ImportRouteTarget: fmt.Sprintf("target:%d:%d", nc.GetIBGPAS(), id),
						ExportRouteTarget: fmt.Sprintf("target:%d:%d", nc.GetIBGPAS(), id),
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

func (r *DeviceBuilder) UpdateNodeIP(ctx context.Context, nodeName, niName string, nd *netwv1alpha1.NetworkDesign) error {
	//nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
	//nodeName := nodeID.Node
	//networkName := cr.GetNetworkName()
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
			Net:             nd.Spec.Protocols.ISIS.Net,
			MaxECMPPaths:    nd.Spec.Protocols.ISIS.MaxECMPPaths,
			AddressFamilies: nd.GetAddressFamilies(),
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

func (r *DeviceBuilder) UpdateInterfaces(ctx context.Context, niName string, nd *netwv1alpha1.NetworkDesign, links []*infrabev1alpha1.Link) error {
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
			if nd.IsISLIPv4Numbered() {
				localEPNodeName := fmt.Sprintf("%s.%s.%s.ipv4", r.nsn.Name, nodeName, epName)

				usedipv4[i], err = r.getIPClaim(ctx, types.NamespacedName{
					Namespace: r.nsn.Namespace,
					Name:      localEPNodeName,
				})
				if err != nil {
					return err
				}
			}
			if nd.IsISLIPv6Numbered() {
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
			if nd.IsISLIPv4Numbered() {
				si.IPv4 = &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv4{
					Addresses: []string{usedipv4[i]},
				}
			}
			if nd.IsISLIPv4UnNumbered() {
				si.IPv4 = &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv4{}
			}
			if nd.IsISLIPv6Numbered() {
				si.IPv6 = &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv6{
					Addresses: []string{usedipv6[i]},
				}
			}
			if nd.IsISLIPv6UnNumbered() {
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
					Net:             nd.Spec.Protocols.ISIS.Net,
					MaxECMPPaths:    nd.Spec.Protocols.ISIS.MaxECMPPaths,
					AddressFamilies: nd.GetAddressFamilies(),
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
				r.devices.AddAddNetworkInstanceprotocolsBGPPeerGroup(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
					Name:            BGPUnderlayPeerGroupName,
					AddressFamilies: nd.GetAddressFamilies(),
				})
			}
		}
	}
	return nil
}

func (r *DeviceBuilder) UpdateProtocolsDynamicNeighbors(ctx context.Context, nodeName, networkName string, nd *netwv1alpha1.NetworkDesign, nodeLabels map[string]string) error {
	log := log.FromContext(ctx)
	//nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
	//nodeName := nodeID.Node
	//networkName := cr.GetNetworkName()
	if nd.IsIBGPEnabled() {
		r.devices.AddAddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
			Name:            BGPOverlayPeerGroupName,
			AddressFamilies: nd.GetOverlayProtocols(),
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
				r.devices.AddAddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
					Name:            BGPOverlayPeerGroupName,
					AddressFamilies: nd.GetOverlayProtocols(),
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
		r.devices.AddAddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
			Name:            BGPOverlayPeerGroupName,
			AddressFamilies: nd.GetOverlayProtocols(),
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
				r.devices.AddAddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
					Name:            BGPOverlayPeerGroupName,
					AddressFamilies: nd.GetOverlayProtocols(),
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
		r.devices.AddAddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
			Name:            BGPOverlayPeerGroupName,
			AddressFamilies: nd.GetOverlayProtocols(),
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
