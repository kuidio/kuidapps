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
	"fmt"

	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	netwv1alpha1 "github.com/kuidio/kuidapps/apis/network/v1alpha1"
	"k8s.io/utils/ptr"
)

type link struct {
	l        *infrabev1alpha1.Link
	nodeID   []infrabev1alpha1.NodeID
	nodeName []string
	epName   []string
	systemID []string
	provider []string
	as       []uint32
	ipv4     []string
	ipv6     []string
}

func newLink(l *infrabev1alpha1.Link) *link {
	return &link{
		l:        l,
		nodeID:   make([]infrabev1alpha1.NodeID, 2),
		nodeName: make([]string, 2),
		epName:   make([]string, 2),
		systemID: make([]string, 2),
		provider: make([]string, 2),
		as:       make([]uint32, 2),
		ipv4:     make([]string, 2),
		ipv6:     make([]string, 2),
	}
}

func (r *link) getNodeID(idx uint) infrabev1alpha1.NodeID {
	if idx > 1 {
		return infrabev1alpha1.NodeID{}
	}
	return r.nodeID[idx]
}

func (r *link) addNodeID(idx uint, nodeID infrabev1alpha1.NodeID) {
	if idx > 1 {
		return
	}
	r.nodeID[idx] = nodeID
}

func (r *link) getNodeName(idx uint) string {
	if idx > 1 {
		return ""
	}
	return r.nodeName[idx]
}

func (r *link) addNodeName(idx uint, node string) {
	if idx > 1 {
		return
	}
	r.nodeName[idx] = node
}

func (r *link) getEPName(idx uint) string {
	if idx > 1 {
		return ""
	}
	return r.epName[idx]
}

func (r *link) addEPName(idx uint, node string) {
	if idx > 1 {
		return
	}
	r.epName[idx] = node
}

func (r *link) getSystemID(idx uint) string {
	if idx > 1 {
		return ""
	}
	return r.systemID[idx]
}

func (r *link) addSystemID(idx uint, systemID string) {
	if idx > 1 {
		return
	}
	r.systemID[idx] = systemID
}

func (r *link) getProvider(idx uint) string {
	if idx > 1 {
		return ""
	}
	return r.provider[idx]
}

func (r *link) AddProvider(idx uint, provider string) {
	if idx > 1 {
		return
	}
	r.provider[idx] = provider
}

func (r *link) getAS(idx uint) uint32 {
	if idx > 1 {
		return 0
	}
	return r.as[idx]
}

func (r *link) addAS(idx uint, as uint32) {
	if idx > 1 {
		return
	}
	r.as[idx] = as
}

func (r *link) getIpv4(idx uint) string {
	if idx > 1 {
		return ""
	}
	return r.ipv4[idx]
}

func (r *link) addIpv4(idx uint, ipv4 string) {
	if idx > 1 {
		return
	}
	r.ipv4[idx] = ipv4
}

func (r *link) getIpv6(idx uint) string {
	if idx > 1 {
		return ""
	}
	return r.ipv4[idx]
}

func (r *link) addIpv6(idx uint, ipv6 string) {
	if idx > 1 {
		return
	}
	r.ipv6[idx] = ipv6
}

func (r *link) getUnderlaySubInterface(idx uint, networkDesign *netwv1alpha1.NetworkDesign, id uint32) *netwv1alpha1.NetworkDeviceInterfaceSubInterface {
	j := idx ^ 1
	si := &netwv1alpha1.NetworkDeviceInterfaceSubInterface{
		PeerName: fmt.Sprintf("%s.%s", r.getNodeName(j), r.getEPName(j)), // these are the peer names
		ID:       id,
		Type:     netwv1alpha1.SubInterfaceType_Routed,
	}
	if networkDesign.IsUnderlayIPv4Numbered() {
		si.IPv4 = &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv4{
			Addresses: []string{r.getIpv4(idx)},
		}
	}
	if networkDesign.IsUnderlayIPv4UnNumbered() {
		si.IPv4 = &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv4{}
	}
	if networkDesign.IsUnderlayIPv6Numbered() {
		si.IPv6 = &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv6{
			Addresses: []string{r.getIpv6(idx)},
		}
	}
	if networkDesign.IsUnderlayIPv6UnNumbered() {
		si.IPv6 = &netwv1alpha1.NetworkDeviceInterfaceSubInterfaceIPv6{}
	}
	return si
}

func (r *link) getBFDLinkParameters(idx uint, networkDesign *netwv1alpha1.NetworkDesign) *infrabev1alpha1.BFDLinkParameters {
	bfdParams := networkDesign.GetUnderlayBFDParameters()
	bfdParams.Enabled = ptr.To[bool](true) // we ignore the enabled flag uin underlay
	// we override the link
	if r.l.Spec.BFD != nil {
		if r.l.Spec.BFD.Enabled != nil {
			*bfdParams.Enabled = *r.l.Spec.BFD.Enabled
		}
		if r.l.Spec.BFD.MinEchoRx != nil {
			*bfdParams.MinEchoRx = *r.l.Spec.BFD.MinEchoRx
		}
		if r.l.Spec.BFD.MinRx != nil {
			*bfdParams.MinRx = *r.l.Spec.BFD.MinRx
		}
		if r.l.Spec.BFD.MinTx != nil {
			*bfdParams.MinTx = *r.l.Spec.BFD.MinTx
		}
		if r.l.Spec.BFD.Multiplier != nil {
			*bfdParams.Multiplier = *r.l.Spec.BFD.Multiplier
		}
	}
	return bfdParams
}

func (r *link) getOSPFArea(idx uint, networkDesign *netwv1alpha1.NetworkDesign) string {
	area := networkDesign.GetOSPFArea()
	if r.l.Spec.OSPF != nil &&
		r.l.Spec.OSPF.Area != nil {
		return *r.l.Spec.OSPF.Area
	}
	return area
}

func (r *link) getOSPFPassive(idx uint, networkDesign *netwv1alpha1.NetworkDesign) bool {
	if r.l.Spec.OSPF != nil &&
		r.l.Spec.OSPF.Passive != nil {
		return *r.l.Spec.OSPF.Passive
	}
	return false
}

func (r *link) getOSPFBFD(idx uint, networkDesign *netwv1alpha1.NetworkDesign) bool {
	if !networkDesign.IsOSPFBFDEnabled() {
		return false
	}
	if r.l.Spec.OSPF != nil &&
		r.l.Spec.OSPF.BFD != nil {
		return *r.l.Spec.OSPF.BFD
	}
	return true // if BFD is gloabbly enabled and not disabled per interface we return true
}

func (r *link) getISISInterface(idx uint, networkDesign *netwv1alpha1.NetworkDesign, id uint32) *netwv1alpha1.NetworkDeviceNetworkInstanceProtocolISISInstanceInterface {
	isisItfce := &netwv1alpha1.NetworkDeviceNetworkInstanceProtocolISISInstanceInterface{
		SubInterfaceName: netwv1alpha1.NetworkDeviceNetworkInstanceInterface{
			Name: r.getEPName(idx),
			ID:   id,
		},
	}

	if r.l.Spec.ISIS != nil &&
		r.l.Spec.ISIS.Passive != nil {
		isisItfce.Passive = *r.l.Spec.ISIS.Passive
	}
	if isisItfce.Passive {
		// if passive, set the network type to unknown
		isisItfce.NetworkType = infrabev1alpha1.NetworkTypeUnknown
	} else {
		isisItfce.NetworkType = infrabev1alpha1.NetworkTypeP2P
		if r.l.Spec.ISIS != nil &&
			r.l.Spec.ISIS.NetworkType != nil {
			isisItfce.NetworkType = *r.l.Spec.ISIS.NetworkType
		}
	}

	switch networkDesign.GetISISLevel() {
	case infrabev1alpha1.ISISLevelL1:
		
	case infrabev1alpha1.ISISLevelL2:

	case infrabev1alpha1.ISISLevelL1L2:

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
}
