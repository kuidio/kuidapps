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

func (r *Device) GetOrCreateBFD() *NetworkDeviceBFD {
	if r.nd.Spec.BFD == nil {
		r.nd.Spec.BFD = &NetworkDeviceBFD{}
	}
	return r.nd.Spec.BFD
}

func (r *NetworkDeviceBFD) AddOrUpdateBFDInterface(new *NetworkDeviceBFDInterface) {
	if r.Interfaces == nil {
		r.Interfaces = []*NetworkDeviceBFDInterface{}
	}
	if new == nil {
		return
	}
	if new.SubInterfaceName.Name == "" {
		return
	}
	x := r.GetOrCreateBFDInterface(new.SubInterfaceName)
	x.BFD = new.BFD
}

func (r *NetworkDeviceBFD) GetOrCreateBFDInterface(siName NetworkDeviceNetworkInstanceInterface) *NetworkDeviceBFDInterface {
	for _, itfce := range r.Interfaces {
		if itfce.SubInterfaceName.Name == siName.Name && itfce.SubInterfaceName.ID == siName.ID {
			return itfce
		}
	}
	newBFDItfce := &NetworkDeviceBFDInterface{
		SubInterfaceName: siName,
	}
	r.Interfaces = append(r.Interfaces, newBFDItfce)
	sort.SliceStable(r.Interfaces, func(i, j int) bool {
		return fmt.Sprintf("%s.%d", r.Interfaces[i].SubInterfaceName.Name, r.Interfaces[i].SubInterfaceName.ID) <
			fmt.Sprintf("%s.%d", r.Interfaces[j].SubInterfaceName.Name, r.Interfaces[j].SubInterfaceName.ID)
	})
	return newBFDItfce
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

func (r *NetworkDeviceNetworkInstanceProtocols) GetOrCreateNetworkInstanceProtocolsISIS() *NetworkDeviceNetworkInstanceProtocolISIS {
	if r.ISIS == nil {
		r.ISIS = &NetworkDeviceNetworkInstanceProtocolISIS{}
	}
	return r.ISIS
}

func (r *NetworkDeviceNetworkInstanceProtocols) GetOrCreateNetworkInstanceProtocolsOSPF() *NetworkDeviceNetworkInstanceProtocolOSPF {
	if r.OSPF == nil {
		r.OSPF = &NetworkDeviceNetworkInstanceProtocolOSPF{}
	}
	return r.OSPF
}

func (r *NetworkDeviceNetworkInstanceProtocolISIS) AddOrUpdateNetworkInstanceProtocolISISInstances(new *NetworkDeviceNetworkInstanceProtocolISISInstance) {
	if r.Instances == nil {
		r.Instances = []*NetworkDeviceNetworkInstanceProtocolISISInstance{}
	}
	x := r.GetOrCreateNetworkInstanceProtocolISISInstance(new.Name)
	x.AddressFamilies = new.AddressFamilies
	x.LevelCapability = new.LevelCapability
	x.Net = new.Net
	x.MaxECMPPaths = new.MaxECMPPaths
}

func (r *NetworkDeviceNetworkInstanceProtocolISIS) GetOrCreateNetworkInstanceProtocolISISInstance(name string) *NetworkDeviceNetworkInstanceProtocolISISInstance {
	for _, instance := range r.Instances {
		if instance.Name == name {
			return instance
		}
	}
	newInstance := &NetworkDeviceNetworkInstanceProtocolISISInstance{
		Name: name,
	}
	r.Instances = append(r.Instances, newInstance)
	sort.SliceStable(r.Instances, func(i, j int) bool {
		return r.Instances[i].Name < r.Instances[j].Name
	})
	return newInstance
}

func (r *NetworkDeviceNetworkInstanceProtocolISISInstance) AddOrUpdateNetworkInstanceProtocolISISInstanceLevel1(new *NetworkDeviceNetworkInstanceProtocolISISInstanceLevel) {
	r.Level1 = r.GetOrCreateNetworkInstanceProtocolISISInstanceLevel1()
	r.Level1.MetricStyle = new.MetricStyle
}

func (r *NetworkDeviceNetworkInstanceProtocolISISInstance) GetOrCreateNetworkInstanceProtocolISISInstanceLevel1() *NetworkDeviceNetworkInstanceProtocolISISInstanceLevel {
	if r.Level1 == nil {
		r.Level1 = &NetworkDeviceNetworkInstanceProtocolISISInstanceLevel{}
	}
	return r.Level1
}

func (r *NetworkDeviceNetworkInstanceProtocolISISInstance) AddOrUpdateNetworkInstanceProtocolISISInstanceLevel2(new *NetworkDeviceNetworkInstanceProtocolISISInstanceLevel) {
	r.Level2 = r.GetOrCreateNetworkInstanceProtocolISISInstanceLevel1()
	r.Level2.MetricStyle = new.MetricStyle
}

func (r *NetworkDeviceNetworkInstanceProtocolISISInstance) GetOrCreateNetworkInstanceProtocolISISInstanceLevel2() *NetworkDeviceNetworkInstanceProtocolISISInstanceLevel {
	if r.Level2 == nil {
		r.Level2 = &NetworkDeviceNetworkInstanceProtocolISISInstanceLevel{}
	}
	return r.Level2
}

func (r *NetworkDeviceNetworkInstanceProtocolISISInstance) AddOrUpdateNetworkInstanceProtocolISISInstanceInterface(new *NetworkDeviceNetworkInstanceProtocolISISInstanceInterface) {
	if r.Interfaces == nil {
		r.Interfaces = []*NetworkDeviceNetworkInstanceProtocolISISInstanceInterface{}
	}
	x := r.GetOrCreateNetworkInstanceProtocolISISInstanceInterface(new.SubInterfaceName)
	x.IPv4 = new.IPv4
	x.IPv6 = new.IPv6
	//x.Level = new.Level
	x.Passive = new.Passive
	x.NetworkType = new.NetworkType
}

func (r *NetworkDeviceNetworkInstanceProtocolISISInstance) GetOrCreateNetworkInstanceProtocolISISInstanceInterface(siName NetworkDeviceNetworkInstanceInterface) *NetworkDeviceNetworkInstanceProtocolISISInstanceInterface {
	for _, itfce := range r.Interfaces {
		if itfce.SubInterfaceName == siName {
			return itfce
		}
	}
	newInterface := &NetworkDeviceNetworkInstanceProtocolISISInstanceInterface{
		SubInterfaceName: siName,
	}
	r.Interfaces = append(r.Interfaces, newInterface)
	sort.SliceStable(r.Interfaces, func(i, j int) bool {
		return fmt.Sprintf("%s.%d", r.Interfaces[i].SubInterfaceName.Name, r.Interfaces[i].SubInterfaceName.ID) <
			fmt.Sprintf("%s.%d", r.Interfaces[j].SubInterfaceName.Name, r.Interfaces[j].SubInterfaceName.ID)
	})
	return newInterface
}

func (r *NetworkDeviceNetworkInstanceProtocolISISInstanceInterface) GetOrCreateNetworkInstanceProtocolISISInstanceInterfaceIPv4() *NetworkDeviceNetworkInstanceProtocolISISInstanceInterfaceIPv4 {
	if r.IPv4 == nil {
		r.IPv4 = &NetworkDeviceNetworkInstanceProtocolISISInstanceInterfaceIPv4{}
	}
	return r.IPv4
}

func (r *NetworkDeviceNetworkInstanceProtocolISISInstanceInterface) GetOrCreateNetworkInstanceProtocolISISInstanceInterfaceIPv6() *NetworkDeviceNetworkInstanceProtocolISISInstanceInterfaceIPv6 {
	if r.IPv6 == nil {
		r.IPv6 = &NetworkDeviceNetworkInstanceProtocolISISInstanceInterfaceIPv6{}
	}
	return r.IPv6
}

func (r *NetworkDeviceNetworkInstanceProtocolISISInstanceInterface) GetOrCreateNetworkInstanceProtocolISISInstanceInterfaceLevel1() *NetworkDeviceNetworkInstanceProtocolISISInstanceInterfaceLevel {
	if r.Level1 == nil {
		r.Level1 = &NetworkDeviceNetworkInstanceProtocolISISInstanceInterfaceLevel{}
	}
	return r.Level1
}

func (r *NetworkDeviceNetworkInstanceProtocolISISInstanceInterface) GetOrCreateNetworkInstanceProtocolISISInstanceInterfaceLevel2() *NetworkDeviceNetworkInstanceProtocolISISInstanceInterfaceLevel {
	if r.Level2 == nil {
		r.Level2 = &NetworkDeviceNetworkInstanceProtocolISISInstanceInterfaceLevel{}
	}
	return r.Level2
}

func (r *NetworkDeviceNetworkInstanceProtocolOSPF) AddOrUpdateNetworkInstanceProtocolOSPFInstances(new *NetworkDeviceNetworkInstanceProtocolOSPFInstance) {
	if r.Instances == nil {
		r.Instances = []*NetworkDeviceNetworkInstanceProtocolOSPFInstance{}
	}
	x := r.GetOrCreateNetworkInstanceProtocolOSPFInstance(new.Name)
	x.ASBR = new.ASBR
	x.MaxECMPPaths = new.MaxECMPPaths
	x.RouterID = new.RouterID
	x.Version = new.Version
}

func (r *NetworkDeviceNetworkInstanceProtocolOSPF) GetOrCreateNetworkInstanceProtocolOSPFInstance(name string) *NetworkDeviceNetworkInstanceProtocolOSPFInstance {
	for _, instance := range r.Instances {
		if instance.Name == name {
			return instance
		}
	}
	newInstance := &NetworkDeviceNetworkInstanceProtocolOSPFInstance{
		Name: name,
	}
	r.Instances = append(r.Instances, newInstance)
	sort.SliceStable(r.Instances, func(i, j int) bool {
		return r.Instances[i].Name < r.Instances[j].Name
	})
	return newInstance
}

func (r *NetworkDeviceNetworkInstanceProtocolOSPFInstance) AddOrUpdateNetworkInstanceProtocolOSPFInstanceArea(new *NetworkDeviceNetworkInstanceProtocolOSPFInstanceArea) {
	if r.Areas == nil {
		r.Areas = []*NetworkDeviceNetworkInstanceProtocolOSPFInstanceArea{}
	}
	x := r.GetOrCreateNetworkInstanceProtocolOSPFInstanceArea(new.Name)
	x.NSSA = new.NSSA
	x.Stub = new.Stub
}

func (r *NetworkDeviceNetworkInstanceProtocolOSPFInstance) GetOrCreateNetworkInstanceProtocolOSPFInstanceArea(name string) *NetworkDeviceNetworkInstanceProtocolOSPFInstanceArea {
	for _, area := range r.Areas {
		if area.Name == name {
			return area
		}
	}
	newArea := &NetworkDeviceNetworkInstanceProtocolOSPFInstanceArea{
		Name: name,
	}
	r.Areas = append(r.Areas, newArea)
	sort.SliceStable(r.Areas, func(i, j int) bool {
		return r.Areas[i].Name < r.Areas[j].Name
	})
	return newArea
}

func (r *NetworkDeviceNetworkInstanceProtocolOSPFInstanceArea) GetOrCreateNetworkInstanceProtocolOSPFInstanceAreaStub() *NetworkDeviceNetworkInstanceProtocolOSPFInstanceAreaStub {
	if r.Stub == nil {
		r.Stub = &NetworkDeviceNetworkInstanceProtocolOSPFInstanceAreaStub{}
	}
	return r.Stub
}

func (r *NetworkDeviceNetworkInstanceProtocolOSPFInstanceArea) GetOrCreateNetworkInstanceProtocolOSPFInstanceAreaNSSA() *NetworkDeviceNetworkInstanceProtocolOSPFInstanceAreaNSSA {
	if r.NSSA == nil {
		r.NSSA = &NetworkDeviceNetworkInstanceProtocolOSPFInstanceAreaNSSA{}
	}
	return r.NSSA
}

func (r *NetworkDeviceNetworkInstanceProtocolOSPFInstanceArea) AddOrUpdateNetworkInstanceProtocolOSPFInstanceAreaInterface(new *NetworkDeviceNetworkInstanceProtocolOSPFInstanceAreaInterface) {
	if r.Interfaces == nil {
		r.Interfaces = []*NetworkDeviceNetworkInstanceProtocolOSPFInstanceAreaInterface{}
	}
	x := r.GetOrCreateNetworkInstanceProtocolOSPFInstanceAreaInterface(new.SubInterfaceName)
	x.NetworkType = new.NetworkType
	x.Passive = new.Passive
	x.BFD = new.BFD
}

func (r *NetworkDeviceNetworkInstanceProtocolOSPFInstanceArea) GetOrCreateNetworkInstanceProtocolOSPFInstanceAreaInterface(siName NetworkDeviceNetworkInstanceInterface) *NetworkDeviceNetworkInstanceProtocolOSPFInstanceAreaInterface {
	for _, itfce := range r.Interfaces {
		if itfce.SubInterfaceName == siName {
			return itfce
		}
	}
	newInterface := &NetworkDeviceNetworkInstanceProtocolOSPFInstanceAreaInterface{
		SubInterfaceName: siName,
	}
	r.Interfaces = append(r.Interfaces, newInterface)
	sort.SliceStable(r.Interfaces, func(i, j int) bool {
		return fmt.Sprintf("%s.%d", r.Interfaces[i].SubInterfaceName.Name, r.Interfaces[i].SubInterfaceName.ID) <
			fmt.Sprintf("%s.%d", r.Interfaces[j].SubInterfaceName.Name, r.Interfaces[j].SubInterfaceName.ID)
	})
	return newInterface
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
	x.BFD = new.BFD

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

func (r *NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighbors) AddOrUpdateetworkInstanceProtocolBGPDynamicNeighborsInterface(new *NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighborsInterface) {
	if r.Interfaces == nil {
		r.Interfaces = []*NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighborsInterface{}
	}
	if new == nil {
		return
	}
	if new.SubInterfaceName.Name == "" {
		return
	}
	x := r.GetOrCreateNetworkInstanceProtocolBGPDynamicNeighborsInterface(new.SubInterfaceName)
	x.PeerAS = new.PeerAS
	x.PeerGroup = new.PeerGroup
}

func (r *NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighbors) GetOrCreateNetworkInstanceProtocolBGPDynamicNeighborsInterface(siName NetworkDeviceNetworkInstanceInterface) *NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighborsInterface {
	for _, itfce := range r.Interfaces {
		if itfce.SubInterfaceName == siName {
			return itfce
		}
	}
	newInterface := &NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighborsInterface{
		SubInterfaceName: siName,
	}
	r.Interfaces = append(r.Interfaces, newInterface)
	sort.SliceStable(r.Interfaces, func(i, j int) bool {
		return fmt.Sprintf("%s.%d", r.Interfaces[i].SubInterfaceName.Name, r.Interfaces[i].SubInterfaceName.ID) <
			fmt.Sprintf("%s.%d", r.Interfaces[j].SubInterfaceName.Name, r.Interfaces[j].SubInterfaceName.ID)
	})
	return newInterface
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
