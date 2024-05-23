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
	return newItfce
}

func (r *NetworkDeviceInterface) AddOrUpdateSubInterface(new *NetworkDeviceInterfaceSubInterface) {
	if r.SubInterfaces == nil {
		r.SubInterfaces = []*NetworkDeviceInterfaceSubInterface{}
	}
	x := r.GetOrCreateSubInterface(new.ID)
	x.PeerName = new.PeerName
	x.VLAN = new.VLAN
	x.SubInterfaceType = new.SubInterfaceType
}

func (r *NetworkDeviceInterface) GetOrCreateSubInterface(id uint32) *NetworkDeviceInterfaceSubInterface {
	for _, si := range r.SubInterfaces {
		if si.ID == id {
			return si
		}
	}
	newSI := &NetworkDeviceInterfaceSubInterface{
		ID: id,
	}
	r.SubInterfaces = append(r.SubInterfaces, newSI)
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

func (r *Device) AddNetworkInstance(name string, niType NetworkInstanceType, interfaces []*NetworkDeviceNetworkInstanceInterface, vxlanItfce *string) {
	if r.nd.Spec.NetworkInstances == nil {
		r.nd.Spec.NetworkInstances = []*NetworkDeviceNetworkInstance{}
	}
	ni := r.GetOrCreateNetworkInstance(name)
	ni.NetworkInstanceType = niType
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
	return newNI
}

func (r *NetworkDeviceNetworkInstance) GetOrCreateprotocols() *NetworkDeviceNetworkInstanceProtocols {
	if r.Protocols == nil {
		r.Protocols = &NetworkDeviceNetworkInstanceProtocols{}
	}
	return r.Protocols
}

func (r *NetworkDeviceNetworkInstanceProtocols) GetOrCreateBGP() *NetworkDeviceNetworkInstanceProtocolBGP {
	if r.BGP == nil {
		r.BGP = &NetworkDeviceNetworkInstanceProtocolBGP{}
	}
	return r.BGP
}

func (r *NetworkDeviceNetworkInstanceProtocolBGP) AddOrUpdatePeerGroup(new *NetworkDeviceNetworkInstanceProtocolBGPPeerGroup) {
	if r.PeerGroups == nil {
		r.PeerGroups = []*NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{}
	}
	if new == nil {
		return
	}
	if new.Name == "" {
		return
	}
	x := r.GetOrCreatePeerGroup(new.Name)
	x.AddressFamilies = new.AddressFamilies
}

func (r *NetworkDeviceNetworkInstanceProtocolBGP) GetOrCreatePeerGroup(name string) *NetworkDeviceNetworkInstanceProtocolBGPPeerGroup {
	for _, peerGroup := range r.PeerGroups {
		if peerGroup.Name == name {
			return peerGroup
		}
	}
	newPeerGroup := &NetworkDeviceNetworkInstanceProtocolBGPPeerGroup{
		Name: name,
	}
	r.PeerGroups = append(r.PeerGroups, newPeerGroup)
	return newPeerGroup
}

func (r *NetworkDeviceNetworkInstanceProtocolBGP) AddOrUpdateNeighbor(new *NetworkDeviceNetworkInstanceProtocolBGPNeighbor) {
	if r.Neighbors == nil {
		r.Neighbors = []*NetworkDeviceNetworkInstanceProtocolBGPNeighbor{}
	}
	if new == nil {
		return
	}
	if new.PeerAddress == "" {
		return
	}
	x := r.GetOrCreateNeighbor(new.PeerAddress)
	x.LocalAS = new.LocalAS
	x.LocalAddress = new.LocalAddress
	x.PeerAS = new.PeerAS
	x.PeerAddress = new.PeerAddress
	x.PeerGroup = new.PeerGroup

}

func (r *NetworkDeviceNetworkInstanceProtocolBGP) GetOrCreateNeighbor(peerAddress string) *NetworkDeviceNetworkInstanceProtocolBGPNeighbor {
	for _, neighbor := range r.Neighbors {
		if neighbor.PeerAddress == peerAddress {
			return neighbor
		}
	}
	newNeighbor := &NetworkDeviceNetworkInstanceProtocolBGPNeighbor{
		PeerAddress: peerAddress,
	}
	r.Neighbors = append(r.Neighbors, newNeighbor)
	return newNeighbor
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
	return newrp
}
