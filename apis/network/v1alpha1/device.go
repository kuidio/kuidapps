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

package v1alpha1

import (
	"fmt"
	"sort"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func NewDevice(nsn types.NamespacedName, nodeName string) *Device {
	parts := strings.Split(nsn.Name, ".")
	return &Device{
		nd: BuildNetworkDevice(
			v1.ObjectMeta{
				Name:      fmt.Sprintf("%s.%s", nsn.Name, nodeName),
				Namespace: nsn.Namespace,
			},
			&NetworkDeviceSpec{
				Topology: parts[0],
			},
			nil,
		),
	}
}

type Device struct {
	nd *NetworkDevice
}

func (r *Device) AddProvider(provider string) {
	r.nd.Spec.Provider = provider
}

func (r *Device) GetNetworkDevice() *NetworkDevice {
	return r.nd
}

func (r *Device) AddOrUpdateInterface(new *NetworkDeviceInterface) {
	if r.nd.Spec.Interfaces == nil {
		r.nd.Spec.Interfaces = []*NetworkDeviceInterface{}
	}
	if new == nil {
		return
	}
	if new.Name == "" {
		return
	}
	x := r.GetOrCreateInterface(new.Name)
	x.LAGMember = new.LAGMember
	x.Speed = new.Speed
	x.VLANTagging = new.VLANTagging
}

func (r *Device) GetOrCreateInterface(name string) *NetworkDeviceInterface {
	for _, itfce := range r.nd.Spec.Interfaces {
		if itfce.Name == name {
			return itfce
		}
	}
	newItfce := &NetworkDeviceInterface{
		Name: name,
	}
	r.nd.Spec.Interfaces = append(r.nd.Spec.Interfaces, newItfce)
	sort.SliceStable(r.nd.Spec.Interfaces, func(i, j int) bool {
		return r.nd.Spec.Interfaces[i].Name < r.nd.Spec.Interfaces[j].Name
	})
	return newItfce
}

func (r *Device) AddOrUpdateTunnelInterface(new *NetworkDeviceTunnelInterface) {
	if r.nd.Spec.TunnelInterfaces == nil {
		r.nd.Spec.TunnelInterfaces = []*NetworkDeviceTunnelInterface{}
	}
	if new == nil {
		return
	}
	if new.Name == "" {
		return
	}
	_ = r.GetOrCreateTunnelInterface(new.Name)
}

func (r *Device) GetOrCreateTunnelInterface(name string) *NetworkDeviceTunnelInterface {
	for _, tunn := range r.nd.Spec.TunnelInterfaces {
		if tunn.Name == name {
			return tunn
		}
	}
	newTun := &NetworkDeviceTunnelInterface{
		Name: name,
	}
	r.nd.Spec.TunnelInterfaces = append(r.nd.Spec.TunnelInterfaces, newTun)
	sort.SliceStable(r.nd.Spec.TunnelInterfaces, func(i, j int) bool {
		return r.nd.Spec.TunnelInterfaces[i].Name < r.nd.Spec.TunnelInterfaces[j].Name
	})
	return newTun
}

func (r *NetworkDeviceInterface) AddOrUpdateInterfaceSubInterface(new *NetworkDeviceInterfaceSubInterface) {
	if r.SubInterfaces == nil {
		r.SubInterfaces = []*NetworkDeviceInterfaceSubInterface{}
	}
	x := r.GetOrCreateInterfaceSubInterface(new.ID)
	x.PeerName = new.PeerName
	x.VLAN = new.VLAN
	x.Type = new.Type
}

func (r *NetworkDeviceInterface) GetOrCreateInterfaceSubInterface(id uint32) *NetworkDeviceInterfaceSubInterface {
	for _, si := range r.SubInterfaces {
		if si.ID == id {
			return si
		}
	}
	newSI := &NetworkDeviceInterfaceSubInterface{
		ID: id,
	}
	r.SubInterfaces = append(r.SubInterfaces, newSI)
	sort.SliceStable(r.SubInterfaces, func(i, j int) bool {
		return r.SubInterfaces[i].ID < r.SubInterfaces[j].ID
	})
	return newSI
}

func (r *NetworkDeviceTunnelInterface) AddOrUpdateTunnelInterfaceSubInterface(new *NetworkDeviceTunnelInterfaceSubInterface) {
	if r.SubInterfaces == nil {
		r.SubInterfaces = []*NetworkDeviceTunnelInterfaceSubInterface{}
	}
	x := r.GetOrCreateTunnelInterfaceSubInterface(new.ID)
	x.Type = new.Type
}

func (r *NetworkDeviceTunnelInterface) GetOrCreateTunnelInterfaceSubInterface(id uint32) *NetworkDeviceTunnelInterfaceSubInterface {
	for _, si := range r.SubInterfaces {
		if si.ID == id {
			return si
		}
	}
	newSI := &NetworkDeviceTunnelInterfaceSubInterface{
		ID: id,
	}
	r.SubInterfaces = append(r.SubInterfaces, newSI)
	sort.SliceStable(r.SubInterfaces, func(i, j int) bool {
		return r.SubInterfaces[i].ID < r.SubInterfaces[j].ID
	})
	return newSI
}

func (r *NetworkDeviceInterfaceSubInterface) GetOrCreateIPv4() *NetworkDeviceInterfaceSubInterfaceIPv4 {
	if r.IPv4 == nil {
		r.IPv4 = &NetworkDeviceInterfaceSubInterfaceIPv4{}
	}
	return r.IPv4
}

func (r *NetworkDeviceInterfaceSubInterface) GetOrCreateIPv6() *NetworkDeviceInterfaceSubInterfaceIPv6 {
	if r.IPv6 == nil {
		r.IPv6 = &NetworkDeviceInterfaceSubInterfaceIPv6{}
	}
	return r.IPv6
}

func (r *Device) AddNetworkInstance(name string, niType NetworkInstanceType, interfaces []*NetworkDeviceNetworkInstanceInterface, vxlanItfce *NetworkDeviceNetworkInstanceInterface) {
	if r.nd.Spec.NetworkInstances == nil {
		r.nd.Spec.NetworkInstances = []*NetworkDeviceNetworkInstance{}
	}
	ni := r.GetOrCreateNetworkInstance(name)
	ni.Type = niType
	ni.VXLANInterface = vxlanItfce
	ni.Interfaces = interfaces // This might need to be optimized going further
}

func (r *Device) GetOrCreateNetworkInstance(name string) *NetworkDeviceNetworkInstance {
	for _, ni := range r.nd.Spec.NetworkInstances {
		if ni.Name == name {
			return ni
		}
	}
	newNI := &NetworkDeviceNetworkInstance{
		Name: name,
	}
	r.nd.Spec.NetworkInstances = append(r.nd.Spec.NetworkInstances, newNI)
	sort.SliceStable(r.nd.Spec.NetworkInstances, func(i, j int) bool {
		return r.nd.Spec.NetworkInstances[i].Name < r.nd.Spec.NetworkInstances[j].Name
	})
	return newNI
}

func (r *NetworkDeviceNetworkInstance) GetOrCreateNetworkInstanceProtocols() *NetworkDeviceNetworkInstanceProtocols {
	if r.Protocols == nil {
		r.Protocols = &NetworkDeviceNetworkInstanceProtocols{}
	}
	return r.Protocols
}

func (r *NetworkDeviceNetworkInstanceProtocols) GetOrCreateNetworkInstanceProtocolsBGP() *NetworkDeviceNetworkInstanceProtocolBGP {
	if r.BGP == nil {
		r.BGP = &NetworkDeviceNetworkInstanceProtocolBGP{}
	}
	return r.BGP
}

func (r *NetworkDeviceNetworkInstanceProtocols) GetOrCreateNetworkInstanceProtocolsBGPEVPN() *NetworkDeviceNetworkInstanceProtocolBGPEVPN {
	if r.BGPEVPN == nil {
		r.BGPEVPN = &NetworkDeviceNetworkInstanceProtocolBGPEVPN{}
	}
	return r.BGPEVPN
}

func (r *NetworkDeviceNetworkInstanceProtocols) GetOrCreateNetworkInstanceProtocolsBGPVPN() *NetworkDeviceNetworkInstanceProtocolBGPVPN {
	if r.BGPVPN == nil {
		r.BGPVPN = &NetworkDeviceNetworkInstanceProtocolBGPVPN{}
	}
	return r.BGPVPN
}

func (r *NetworkDeviceNetworkInstanceProtocolBGP) AddOrUpdateNetworkInstanceProtocolBGPPeerGroup(new *NetworkDeviceNetworkInstanceProtocolBGPPeerGroup) {
	if r.PeerGroups == nil {
		r.PeerGroups = []*NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{}
	}
	if new == nil {
		return
	}
	if new.Name == "" {
		return
	}
	x := r.GetOrCreateNetworkInstanceProtocolBGPPeerGroup(new.Name)
	x.AddressFamilies = new.AddressFamilies
	x.RouteReflector = new.RouteReflector
}

func (r *NetworkDeviceNetworkInstanceProtocolBGP) GetOrCreateNetworkInstanceProtocolBGPPeerGroup(name string) *NetworkDeviceNetworkInstanceProtocolBGPPeerGroup {
	for _, peerGroup := range r.PeerGroups {
		if peerGroup.Name == name {
			return peerGroup
		}
	}
	newPeerGroup := &NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
		Name: name,
	}
	r.PeerGroups = append(r.PeerGroups, newPeerGroup)
	sort.SliceStable(r.PeerGroups, func(i, j int) bool {
		return r.PeerGroups[i].Name < r.PeerGroups[j].Name
	})
	return newPeerGroup
}

func (r *NetworkDeviceNetworkInstanceProtocolBGP) AddOrUpdateNetworkInstanceProtocolBGNeighbor(new *NetworkDeviceNetworkInstanceProtocolBGPNeighbor) {
	if r.Neighbors == nil {
		r.Neighbors = []*NetworkDeviceNetworkInstanceProtocolBGPNeighbor{}
	}
	if new == nil {
		return
	}
	if new.PeerAddress == "" {
		return
	}
	x := r.GetOrCreateNetworkInstanceProtocolBGPNeighbor(new.PeerAddress)
	x.LocalAS = new.LocalAS
	x.LocalAddress = new.LocalAddress
	x.PeerAS = new.PeerAS
	x.PeerAddress = new.PeerAddress
	x.PeerGroup = new.PeerGroup

}

func (r *NetworkDeviceNetworkInstanceProtocolBGP) GetOrCreateNetworkInstanceProtocolBGPNeighbor(peerAddress string) *NetworkDeviceNetworkInstanceProtocolBGPNeighbor {
	for _, neighbor := range r.Neighbors {
		if neighbor.PeerAddress == peerAddress {
			return neighbor
		}
	}
	newNeighbor := &NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
		PeerAddress: peerAddress,
	}
	r.Neighbors = append(r.Neighbors, newNeighbor)
	sort.SliceStable(r.Neighbors, func(i, j int) bool {
		return r.Neighbors[i].PeerAddress < r.Neighbors[j].PeerAddress
	})
	return newNeighbor
}

func (r *NetworkDeviceNetworkInstanceProtocolBGP) GetOrCreateNetworkInstanceProtocolBGPDynamicNeighbors() *NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighbors {
	if r.DynamicNeighbors == nil {
		r.DynamicNeighbors = &NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighbors{}
	}
	return r.DynamicNeighbors
}

func (r *NetworkDeviceNetworkInstanceProtocolBGP) AddOrCreateNetworkInstanceProtocolBGPDynamicNeighbors(new *NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighbors) {
	x := r.GetOrCreateNetworkInstanceProtocolBGPDynamicNeighbors()
	x.PeerAS = new.PeerAS
	x.PeerGroup = new.PeerGroup
	x.PeerPrefixes = new.PeerPrefixes

}

func (r *Device) AddOrUpdateRoutingPolicy(new *NetworkDeviceRoutingPolicy) {
	if r.nd.Spec.RoutingPolicies == nil {
		r.nd.Spec.RoutingPolicies = []*NetworkDeviceRoutingPolicy{}
	}
	if new == nil {
		return
	}
	if new.Name == "" {
		return
	}
	x := r.GetOrCreateRoutingPolicy(new.Name)
	x.IPv4Prefixes = new.IPv4Prefixes
	x.IPv6Prefixes = new.IPv6Prefixes
}

func (r *Device) GetOrCreateRoutingPolicy(name string) *NetworkDeviceRoutingPolicy {
	for _, rp := range r.nd.Spec.RoutingPolicies {
		if rp.Name == name {
			return rp
		}
	}
	newrp := &NetworkDeviceRoutingPolicy{
		Name: name,
	}
	r.nd.Spec.RoutingPolicies = append(r.nd.Spec.RoutingPolicies, newrp)
	sort.SliceStable(r.nd.Spec.RoutingPolicies, func(i, j int) bool {
		return r.nd.Spec.RoutingPolicies[i].Name < r.nd.Spec.RoutingPolicies[j].Name
	})
	return newrp
}

func (r *Device) GetOrCreateSystem() *NetworkDeviceSystem {
	if r.nd.Spec.System == nil {
		r.nd.Spec.System = &NetworkDeviceSystem{}
	}
	return r.nd.Spec.System
}

func (r *NetworkDeviceSystem) GetOrCreateSystemProtocols() *NetworkDeviceSystemProtocols {
	if r.Protocols == nil {
		r.Protocols = &NetworkDeviceSystemProtocols{}
	}
	return r.Protocols
}

func (r *NetworkDeviceSystemProtocols) GetOrCreateSystemProtocolsBGPEVPN() *NetworkDeviceSystemProtocolsBGPEVPN {
	if r.BGPEVPN == nil {
		r.BGPEVPN = &NetworkDeviceSystemProtocolsBGPEVPN{}
	}
	return r.BGPEVPN
}

func (r *NetworkDeviceSystemProtocols) GetOrCreateSystemProtocolsBGPVPN() *NetworkDeviceSystemProtocolsBGPVPN {
	if r.BGPVPN == nil {
		r.BGPVPN = &NetworkDeviceSystemProtocolsBGPVPN{}
	}
	return r.BGPVPN
}
