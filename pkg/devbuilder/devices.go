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

func (r *Devices) AddAS(nodeName, niName string, as uint32) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.GetOrCreateNetworkInstance(niName).GetOrCreateprotocols().GetOrCreateBGP().AS = as
}

func (r *Devices) AddRouterID(nodeName, niName string, routerID string) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.GetOrCreateNetworkInstance(niName).GetOrCreateprotocols().GetOrCreateBGP().RouterID = routerID
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

func (r *Devices) AddSubInterface(nodeName, ifName string, x *netwv1alpha1.NetworkDeviceInterfaceSubInterface, ipv4, ipv6 []string) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	itfce := d.GetOrCreateInterface(ifName)
	itfce.AddOrUpdateSubInterface(x)
	si := itfce.GetOrCreateSubInterface(x.ID)
	if len(ipv4) != 0 {
		si.GetOrCreateIPv4().Addresses = ipv4
	}
	if len(ipv6) != 0 {
		si.GetOrCreateIPv6().Addresses = ipv6
	}
}

func (r *Devices) AddNetworkInstance(nodeName string, newNI *netwv1alpha1.NetworkDeviceNetworkInstance) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]

	ni := d.GetOrCreateNetworkInstance(newNI.Name)
	ni.NetworkInstanceType = newNI.NetworkInstanceType
}

func (r *Devices) AddNetworkInstanceSubInterface(nodeName, niName, ifName string) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	if len(d.GetOrCreateNetworkInstance(niName).Interfaces) == 0 {
		d.GetOrCreateNetworkInstance(niName).Interfaces = []string{}
	}
	d.GetOrCreateNetworkInstance(niName).Interfaces = append(d.GetOrCreateNetworkInstance(niName).Interfaces, ifName)
}

func (r *Devices) AddBGPNeighbor(nodeName, niName string, x *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPNeighbor) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.GetOrCreateNetworkInstance(niName).GetOrCreateprotocols().GetOrCreateBGP().AddOrUpdateNeighbor(x)
}

func (r *Devices) AddBGPPeerGroup(nodeName, niName string, x *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolBGPPeerGroup) {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	d.GetOrCreateNetworkInstance(niName).GetOrCreateprotocols().GetOrCreateBGP().AddOrUpdatePeerGroup(x)
}

func (r *Devices) GetSystemIP(nodeName, ifName string, id uint32, ipv4 bool) string {
	r.m.Lock()
	defer r.m.Unlock()
	if _, ok := r.devices[nodeName]; !ok {
		r.devices[nodeName] = netwv1alpha1.NewDevice(r.nsn, nodeName)
	}
	d := r.devices[nodeName]
	si := d.GetOrCreateInterface(ifName).GetOrCreateSubInterface(id)
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
