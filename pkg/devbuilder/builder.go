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

		for _, bd := range cr.Spec.BridgeDomains {
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
		for _, ni := range cr.Spec.RoutingTables {
			niName := fmt.Sprintf("%s.%s", networkName, ni.Name)
			id := uint32(ni.NetworkID)
			for _, itfce := range ni.Interfaces {
				if !itfce.IsDynamic() {
					if itfce.BridgeDomain == nil && itfce.EndPoint == nil {
						return fmt.Errorf("cannot create an rt interface w/o a bridgedomain or endpoint specified")
					}

					if itfce.NodeID == nil {
						return fmt.Errorf("unsupported for now")
						// use the nodes collected by the bridge domain logic
						// need to split the ip prefix per node
					}

					nodeName := itfce.Node
					if itfce.EndPoint == nil && itfce.BridgeDomain == nil {
						return fmt.Errorf("cannot create a interface w/o no endpoint and brdigedomain")
					}

					if itfce.BridgeDomain == nil {
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
					if itfce.BridgeDomain == nil {
						r.devices.AddSubInterface(nodeName, *itfce.EndPoint, &netwv1alpha1.NetworkDeviceInterfaceSubInterface{
							PeerName: "customer",
							ID:       id,
							Type:     netwv1alpha1.SubInterfaceType_Routed,
							VLAN:     vlan,
						}, ipv4, ipv6)
						r.devices.AddNetworkInstanceSubInterface(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
							Name: *itfce.EndPoint,
							ID:   id,
						})
					} else {
						if !cr.IsBridgeDomainPresent(*itfce.BridgeDomain) {
							return fmt.Errorf("cannot create an irb interface on a bridgedomain that is not present")
						}
						r.devices.AddSubInterface(nodeName, IRBInterfaceName, &netwv1alpha1.NetworkDeviceInterfaceSubInterface{
							PeerName: "customer",
							ID:       id,
							//Type:     netwv1alpha1.SubInterfaceType_Routed,// not type exepected
						}, ipv4, ipv6)
						r.devices.AddNetworkInstanceSubInterface(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
							Name: IRBInterfaceName,
							ID:   id,
						})
						// add the irb to the bridge domain as well
						bdName := fmt.Sprintf("%s.%s", networkName, *itfce.BridgeDomain)
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
					r.devices.AddAddNetworkInstanceprotocolsBGPNeighbor(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
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
					r.devices.AddAddNetworkInstanceprotocolsBGPNeighbor(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
						LocalAddress: pii.GetIPAddress().String(),
						PeerAddress:  pij.GetIPAddress().String(),
						PeerGroup:    BGPUnderlayPeerGroupName,
						LocalAS:      as[i],
						PeerAS:       as[j],
					})
					afs = append(afs, "ipv6-unicast")
				}
				r.devices.AddAddNetworkInstanceprotocolsBGPPeerGroup(nodeName, niName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
					Name:            BGPUnderlayPeerGroupName,
					AddressFamilies: afs,
				})
			}
		}
	}
	return nil
}

func (r *DeviceBuilder) UpdateProtocolsDynamicNeighbors(ctx context.Context, nodeName, networkName string, nc *netwv1alpha1.NetworkConfig, nodeLabels map[string]string) error {
	log := log.FromContext(ctx)
	//nodeID := infrabev1alpha1.String2NodeGroupNodeID(n.GetName())
	//nodeName := nodeID.Node
	//networkName := cr.GetNetworkName()
	if nc.IsIBGPEnabled() {
		r.devices.AddAddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
			Name:            BGPOverlayPeerGroupName,
			AddressFamilies: nc.GetOverlayProtocols(),
		})

		fullNodeName := fmt.Sprintf("%s.%s", r.nsn.Name, nodeName)
		fullNodeNameIPv4 := fmt.Sprintf("%s.ipv4", fullNodeName)
		fullNodeNameIPv6 := fmt.Sprintf("%s.ipv6", fullNodeName)

		for _, rrNodeNameAF := range nc.Spec.Protocols.IBGP.RouteReflectors {
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
				r.devices.AddAddNetworkInstanceprotocolsBGPNeighbor(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
					LocalAddress: localAddress,
					PeerAddress:  peerPrefix.GetIPAddress().String(),
					PeerGroup:    BGPOverlayPeerGroupName,
					LocalAS:      nc.GetIBGPAS(),
					PeerAS:       nc.GetIBGPAS(),
				})
			}

			log.Info("rrNodeName", "rrNodeNameAF", rrNodeNameAF, "ipv4", fullNodeNameIPv4)

			if rrNodeNameAF == fullNodeNameIPv4 || rrNodeNameAF == fullNodeNameIPv6 {
				// this is the rr - we add the peer group again as this cannot harm
				r.devices.AddAddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
					Name:            BGPOverlayPeerGroupName,
					AddressFamilies: nc.GetOverlayProtocols(),
					RouteReflector: &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroupRouteReflector{
						ClusterID: rrIP,
					},
				})
				r.devices.AddAddNetworkInstanceprotocolsBGPDynamicNeighbor(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighbors{
					PeerPrefixes: nc.GetLoopbackPrefixes(),
					PeerAS:       nc.GetIBGPAS(),
					PeerGroup:    BGPOverlayPeerGroupName,
				})
			}
		}
	}
	return nil
}

func (r *DeviceBuilder) UpdateProtocolsRR(ctx context.Context, nodeName, networkName string, nc *netwv1alpha1.NetworkConfig, edgePeers sets.Set[string]) error {
	//log := log.FromContext(ctx)
	if nc.IsIBGPEnabled() {
		r.devices.AddAddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
			Name:            BGPOverlayPeerGroupName,
			AddressFamilies: nc.GetOverlayProtocols(),
		})

		fullNodeName := fmt.Sprintf("%s.%s", r.nsn.Name, nodeName)
		fullNodeNameIPv4 := fmt.Sprintf("%s.ipv4", fullNodeName)
		fullNodeNameIPv6 := fmt.Sprintf("%s.ipv6", fullNodeName)

		for _, rrNodeNameAF := range nc.Spec.Protocols.IBGP.RouteReflectors {
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
					AddressFamilies: nc.GetOverlayProtocols(),
					RouteReflector: &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroupRouteReflector{
						ClusterID: peerPrefix.GetIPAddress().String(),
					},
				})

				for _, peerIP := range edgePeers.UnsortedList() {
					r.devices.AddAddNetworkInstanceprotocolsBGPNeighbor(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
						LocalAddress: localAddress,
						PeerAddress:  peerIP,
						PeerGroup:    BGPOverlayPeerGroupName,
						LocalAS:      nc.GetIBGPAS(),
						PeerAS:       nc.GetIBGPAS(),
					})
				}
			}
		}
	}
	return nil
}

func (r *DeviceBuilder) UpdateProtocolsEdge(ctx context.Context, nodeName, networkName string, nc *netwv1alpha1.NetworkConfig, edgePeers sets.Set[string]) error {
	if nc.IsIBGPEnabled() {
		r.devices.AddAddNetworkInstanceprotocolsBGPPeerGroup(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
			Name:            BGPOverlayPeerGroupName,
			AddressFamilies: nc.GetOverlayProtocols(),
		})

		for _, rrNodeNameAF := range nc.Spec.Protocols.IBGP.RouteReflectors {
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

			r.devices.AddAddNetworkInstanceprotocolsBGPNeighbor(nodeName, networkName, &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
				LocalAddress: localAddress,
				PeerAddress:  peerPrefix.GetIPAddress().String(),
				PeerGroup:    BGPOverlayPeerGroupName,
				LocalAS:      nc.GetIBGPAS(),
				PeerAS:       nc.GetIBGPAS(),
			})
		}
	}
	return nil
}

func (r *DeviceBuilder) UpdateRoutingPolicies(ctx context.Context, nodeName string, isDefaultNetwork bool, nc *netwv1alpha1.NetworkConfig) error {
	if isDefaultNetwork {
		ipv4, ipv6 := nc.GetLoopbackPrefixesPerAF()
		r.devices.AddRoutingPolicy(nodeName, "underlay", ipv4, ipv6)
		r.devices.AddRoutingPolicy(nodeName, "overlay", nil, nil)
	}
	return nil
}
