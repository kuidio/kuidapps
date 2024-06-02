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
	"sync"

	netwv1alpha1 "github.com/kuidio/kuidapps/apis/network/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

func NewDevices(nsn types.NamespacedName) *Devices {
	return &Devices{
		nsn:     nsn,
		devices: map[string]*netwv1alpha1.Device{},
	}
}

type Devices struct {
	nsn     types.NamespacedName
	m       sync.RWMutex
	devices map[string]*netwv1alpha1.Device
}

func (r *Devices) GetNetworkDeviceConfigs() []*netwv1alpha1.NetworkDevice {
	r.m.RLock()
	defer r.m.RUnlock()

	dc := make([]*netwv1alpha1.NetworkDevice, 0, len(r.devices))
	for _, d := range r.devices {
		dc = append(dc, d.GetNetworkDevice())
	}
	return dc
}

func (r *Devices) AddProvider(nodeName, provider string) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.AddProvider(provider)
}

func (r *Devices) AddNetworkInstanceProtocolsBGPAS(nodeName, niName string, as uint32) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.GetOrCreateNetworkInstance(niName).GetOrCreateNetworkInstanceProtocols().GetOrCreateNetworkInstanceProtocolsBGP().AS = as
}

func (r *Devices) AddNetworkInstanceProtocolsBGPRouterID(nodeName, niName string, routerID string) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.GetOrCreateNetworkInstance(niName).GetOrCreateNetworkInstanceProtocols().GetOrCreateNetworkInstanceProtocolsBGP().RouterID = routerID
}

func (r *Devices) AddInterface(nodeName string, x *netwv1alpha1.NetworkDeviceInterface) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.AddOrUpdateInterface(x)
}

func (r *Devices) AddTunnelInterface(nodeName string, x *netwv1alpha1.NetworkDeviceTunnelInterface) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.AddOrUpdateTunnelInterface(x)
}

func (r *Devices) AddSubInterface(nodeName, ifName string, x *netwv1alpha1.NetworkDeviceInterfaceSubInterface) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	itfce := d.GetOrCreateInterface(ifName)
	if x.VLAN != nil {
		itfce.VLANTagging = true // HACK need to be properly fixed
	}
	itfce.AddOrUpdateInterfaceSubInterface(x)
	si := itfce.GetOrCreateInterfaceSubInterface(x.ID)
	si.IPv4 = x.IPv4
	si.IPv6 = x.IPv6
	si.PeerName = x.PeerName
	si.VLAN = x.VLAN
	si.Type = x.Type

	/*
		if len(ipv4) != 0 {
			sort.Strings(ipv4)
			si.GetOrCreateIPv4().Addresses = ipv4
		}
		if len(ipv6) != 0 {
			sort.Strings(ipv6)
			si.GetOrCreateIPv6().Addresses = ipv6
		}
	*/
}

func (r *Devices) AddTunnelSubInterface(nodeName, ifName string, x *netwv1alpha1.NetworkDeviceTunnelInterfaceSubInterface) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	itfce := d.GetOrCreateTunnelInterface(ifName)
	itfce.AddOrUpdateTunnelInterfaceSubInterface(x)
	si := itfce.GetOrCreateTunnelInterfaceSubInterface(x.ID)
	si.Type = x.Type
}

func (r *Devices) AddNetworkInstance(nodeName string, newNI *netwv1alpha1.NetworkDeviceNetworkInstance) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]

	ni := d.GetOrCreateNetworkInstance(newNI.Name)
	ni.Type = newNI.Type
}

func (r *Devices) AddNetworkInstanceSubInterface(nodeName, niName string, niItfce *netwv1alpha1.NetworkDeviceNetworkInstanceInterface) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	if len(d.GetOrCreateNetworkInstance(niName).Interfaces) == 0 {
		d.GetOrCreateNetworkInstance(niName).Interfaces = []*netwv1alpha1.NetworkDeviceNetworkInstanceInterface{}
	}
	for i, itfce := range d.GetOrCreateNetworkInstance(niName).Interfaces {
		if itfce.Name == niItfce.Name && itfce.ID == niItfce.ID {
			d.GetOrCreateNetworkInstance(niName).Interfaces[i] = niItfce
			return
		}
	}
	d.GetOrCreateNetworkInstance(niName).Interfaces = append(d.GetOrCreateNetworkInstance(niName).Interfaces, niItfce)
}

func (r *Devices) AddNetworkInstanceSubInterfaceVXLAN(nodeName, niName string, vxlanItfce *netwv1alpha1.NetworkDeviceNetworkInstanceInterface) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	if len(d.GetOrCreateNetworkInstance(niName).Interfaces) == 0 {
		d.GetOrCreateNetworkInstance(niName).Interfaces = []*netwv1alpha1.NetworkDeviceNetworkInstanceInterface{}
	}
	d.GetOrCreateNetworkInstance(niName).VXLANInterface = vxlanItfce
}

func (r *Devices) AddNetworkInstanceProtocolsISISInstance(nodeName, niName string, instance *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolISISInstance) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	isisInstance := d.GetOrCreateNetworkInstance(niName).
		GetOrCreateNetworkInstanceProtocols().
		GetOrCreateNetworkInstanceProtocolsISIS().
		GetOrCreateNetworkInstanceProtocolISISInstance(instance.Name)
	isisInstance.AddressFamilies = instance.AddressFamilies
	isisInstance.LevelCapability = instance.LevelCapability
	isisInstance.MaxECMPPaths = instance.MaxECMPPaths
	isisInstance.Net = instance.Net
}

func (r *Devices) AddNetworkInstanceProtocolsISISInstanceInterface(nodeName, niName, instanceName string, itfce *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolISISInstanceInterface) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.GetOrCreateNetworkInstance(niName).
		GetOrCreateNetworkInstanceProtocols().
		GetOrCreateNetworkInstanceProtocolsISIS().
		GetOrCreateNetworkInstanceProtocolISISInstance(instanceName).
		AddOrUpdateNetworkInstanceProtocolISISInstanceInterface(itfce)
}

func (r *Devices) AddNetworkInstanceProtocolsOSPFInstance(nodeName, niName string, instance *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolOSPFInstance) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	ospfInstance := d.GetOrCreateNetworkInstance(niName).
		GetOrCreateNetworkInstanceProtocols().
		GetOrCreateNetworkInstanceProtocolsOSPF().
		GetOrCreateNetworkInstanceProtocolOSPFInstance(instance.Name)
	ospfInstance.RouterID = instance.RouterID
	ospfInstance.Version = instance.Version
	ospfInstance.MaxECMPPaths = instance.MaxECMPPaths
	ospfInstance.ASBR = instance.ASBR
}

func (r *Devices) AddNetworkInstanceProtocolsOSPFInstanceArea(nodeName, niName, instanceName string, area *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolOSPFInstanceArea) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.GetOrCreateNetworkInstance(niName).
		GetOrCreateNetworkInstanceProtocols().
		GetOrCreateNetworkInstanceProtocolsOSPF().
		GetOrCreateNetworkInstanceProtocolOSPFInstance(instanceName).
		AddOrUpdateNetworkInstanceProtocolOSPFInstanceArea(area)
}

func (r *Devices) AddNetworkInstanceProtocolsOSPFInstanceAreaInterface(nodeName, niName, instanceName, area string, itfce *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolOSPFInstanceAreaInterface) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.GetOrCreateNetworkInstance(niName).
		GetOrCreateNetworkInstanceProtocols().
		GetOrCreateNetworkInstanceProtocolsOSPF().
		GetOrCreateNetworkInstanceProtocolOSPFInstance(instanceName).
		GetOrCreateNetworkInstanceProtocolOSPFInstanceArea(area).
		AddOrUpdateNetworkInstanceProtocolOSPFInstanceAreaInterface(itfce)
}

func (r *Devices) AddNetworkInstanceprotocolsBGPNeighbor(nodeName, niName string, x *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.GetOrCreateNetworkInstance(niName).GetOrCreateNetworkInstanceProtocols().GetOrCreateNetworkInstanceProtocolsBGP().AddOrUpdateNetworkInstanceProtocolBGNeighbor(x)
}

func (r *Devices) AddNetworkInstanceprotocolsBGPDynamicNeighbor(nodeName, niName string, new *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighborsInterface) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.GetOrCreateNetworkInstance(niName).GetOrCreateNetworkInstanceProtocols().GetOrCreateNetworkInstanceProtocolsBGP().GetOrCreateNetworkInstanceProtocolBGPDynamicNeighbors().AddOrUpdateetworkInstanceProtocolBGPDynamicNeighborsInterface(new)
}

func (r *Devices) AddAddNetworkInstanceprotocolsBGPPeerGroup(nodeName, niName string, x *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.GetOrCreateNetworkInstance(niName).GetOrCreateNetworkInstanceProtocols().GetOrCreateNetworkInstanceProtocolsBGP().AddOrUpdateNetworkInstanceProtocolBGPPeerGroup(x)
}

func (r *Devices) GetSystemIP(nodeName, ifName string, id uint32, ipv4 bool) string {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	si := d.GetOrCreateInterface(ifName).GetOrCreateInterfaceSubInterface(id)
	if ipv4 {
		if si.IPv4 != nil && len(si.IPv4.Addresses) > 0 {
			return si.IPv4.Addresses[0]
		}
	} else {
		if si.IPv6 != nil && len(si.IPv6.Addresses) > 0 {
			return si.IPv6.Addresses[0]
		}
	}
	return ""
}

func (r *Devices) AddRoutingPolicy(nodeName, policyName string, ipv4, ipv6 []string) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	rp := d.GetOrCreateRoutingPolicy(policyName)
	rp.IPv4Prefixes = ipv4
	rp.IPv6Prefixes = ipv6

}

func (r *Devices) AddNetworkInstanceProtocolsBGPVPN(nodeName, niName string, x *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPVPN) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	bgpvpn := d.GetOrCreateNetworkInstance(niName).GetOrCreateNetworkInstanceProtocols().GetOrCreateNetworkInstanceProtocolsBGPVPN()
	bgpvpn.ExportRouteTarget = x.ExportRouteTarget
	bgpvpn.ImportRouteTarget = x.ImportRouteTarget
}

func (r *Devices) AddNetworkInstanceProtocolsBGPEVPN(nodeName, niName string, x *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPEVPN) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	bgpevpn := d.GetOrCreateNetworkInstance(niName).GetOrCreateNetworkInstanceProtocols().GetOrCreateNetworkInstanceProtocolsBGPEVPN()
	bgpevpn.ECMP = x.ECMP
	bgpevpn.EVI = x.EVI
	bgpevpn.VXLANInterface = x.VXLANInterface
}

func (r *Devices) AddSystemProtocolsBGPVPN(nodeName string, x *netwv1alpha1.NetworkDeviceSystemProtocolsBGPVPN) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.GetOrCreateSystem().GetOrCreateSystemProtocols().GetOrCreateSystemProtocolsBGPVPN()
}
