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

	"github.com/henderiw/iputil"
	"github.com/kuidio/kuid/apis/backend"
	asbev1alpha1 "github.com/kuidio/kuid/apis/backend/as/v1alpha1"
	genidbev1alpha1 "github.com/kuidio/kuid/apis/backend/genid/v1alpha1"
	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	ipambev1alpha1 "github.com/kuidio/kuid/apis/backend/ipam/v1alpha1"
	commonv1alpha1 "github.com/kuidio/kuid/apis/common/v1alpha1"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetCondition returns the condition based on the condition kind
func (r *NetworkDesign) GetCondition(t conditionv1alpha1.ConditionType) conditionv1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *NetworkDesign) SetConditions(c ...conditionv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

func (r *NetworkDesign) Validate() error {
	return nil
}

func (r *NetworkDesign) GetIPClaims() []*ipambev1alpha1.IPClaim {
	claims := make([]*ipambev1alpha1.IPClaim, 0)
	if r.Spec.Interfaces != nil {
		if r.Spec.Interfaces.Loopback != nil {
			for _, prefix := range r.Spec.Interfaces.Loopback.Prefixes {
				if len(prefix.Labels) == 0 {
					prefix.Labels = map[string]string{}
				}
				prefix.Labels[backend.KuidINVPurpose] = string(InterfaceKindLoopback)
				prefix.Labels[backend.KuidIPAMIPPrefixTypeKey] = string(ipambev1alpha1.IPPrefixType_Pool)
				claims = append(claims, ipambev1alpha1.GetIPClaimFromPrefix(r, prefix))
			}
		}
		if r.Spec.Interfaces.ISL != nil {
			for _, prefix := range r.Spec.Interfaces.ISL.Prefixes {
				if len(prefix.Labels) == 0 {
					prefix.Labels = map[string]string{}
				}
				prefix.Labels[backend.KuidINVPurpose] = string(InterfaceKindISL)
				prefix.Labels[backend.KuidIPAMIPPrefixTypeKey] = string(ipambev1alpha1.IPPrefixType_Network)
				claims = append(claims, ipambev1alpha1.GetIPClaimFromPrefix(r, prefix))
			}
		}
	}
	return claims
}

func (r *NetworkDesign) GetASIndex() *asbev1alpha1.ASIndex {
	return asbev1alpha1.BuildASIndex(
		metav1.ObjectMeta{
			Namespace: r.Namespace,
			Name:      r.Name, // r.name = topology.<network> e.g default or vpc1, etc
		},
		nil,
		nil,
	)
}

func (r *NetworkDesign) GetGENIDClaim() *genidbev1alpha1.GENIDClaim {
	return genidbev1alpha1.BuildGENIDClaim(
		metav1.ObjectMeta{
			Namespace: r.Namespace,
			Name:      fmt.Sprintf("network.%s", r.Name), // r.name = topology.<network> e.g default or vpc1, etc
		},
		&genidbev1alpha1.GENIDClaimSpec{
			Index: fmt.Sprintf("network.%s", r.Spec.Topology),
			Owner: commonv1alpha1.GetOwnerReference(r),
		},
		nil,
	)
}

func (r *NetworkDesign) GetASClaims() []*asbev1alpha1.ASClaim {
	ases := make([]*asbev1alpha1.ASClaim, 0)
	if r.Spec.Protocols != nil {
		if r.Spec.Protocols.IBGP != nil {
			if r.Spec.Protocols.IBGP.AS != nil {
				ases = append(ases, asbev1alpha1.BuildASClaim(
					metav1.ObjectMeta{
						Namespace: r.Namespace,
						Name:      fmt.Sprintf("%s.ibgp", r.Name), // r.name = topology.<network> e.g default or vpc1, etc
					},
					&asbev1alpha1.ASClaimSpec{
						Index: r.Name,
						ID:    r.Spec.Protocols.IBGP.AS,
						Owner: commonv1alpha1.GetOwnerReference(r),
					},
					nil,
				))
			}
		}
		if r.Spec.Protocols.EBGP != nil {
			if r.Spec.Protocols.EBGP.ASPool != nil {
				ases = append(ases, asbev1alpha1.BuildASClaim(
					metav1.ObjectMeta{
						Namespace: r.Namespace,
						Name:      fmt.Sprintf("%s.aspool", r.Name), // r.name = topology.<network> e.g default or vpc1, etc
					},
					&asbev1alpha1.ASClaimSpec{
						Index: r.Name,
						Range: r.Spec.Protocols.EBGP.ASPool,
						Owner: commonv1alpha1.GetOwnerReference(r),
					},
					nil,
				))
			}
		}
	}
	return ases
}

func (r *NetworkDesign) IsLoopbackIPv4Enabled() bool {
	return r.Spec.Interfaces != nil && r.Spec.Interfaces.Loopback != nil &&
		(r.Spec.Interfaces.Loopback.Addressing == Addressing_DualStack ||
			r.Spec.Interfaces.Loopback.Addressing == Addressing_IPv4Only)
}

func (r *NetworkDesign) IsLoopbackIPv6Enabled() bool {
	return r.Spec.Interfaces != nil && r.Spec.Interfaces.Loopback != nil &&
		(r.Spec.Interfaces.Loopback.Addressing == Addressing_DualStack ||
			r.Spec.Interfaces.Loopback.Addressing == Addressing_IPv6Only)
}

func (r *NetworkDesign) IsISLIPv4Enabled() bool {
	return r.Spec.Interfaces != nil && r.Spec.Interfaces.ISL != nil &&
		(r.Spec.Interfaces.ISL.Addressing == Addressing_DualStack ||
			r.Spec.Interfaces.ISL.Addressing == Addressing_IPv4Only ||
			r.Spec.Interfaces.ISL.Addressing == Addressing_IPv4Unnumbered)
}

func (r *NetworkDesign) IsISLIPv4Numbered() bool {
	return r.Spec.Interfaces != nil && r.Spec.Interfaces.ISL != nil &&
		(r.Spec.Interfaces.ISL.Addressing == Addressing_DualStack ||
			r.Spec.Interfaces.ISL.Addressing == Addressing_IPv4Only)
}

func (r *NetworkDesign) IsISLIPv4UnNumbered() bool {
	return r.Spec.Interfaces != nil && r.Spec.Interfaces.ISL != nil &&
		(r.Spec.Interfaces.ISL.Addressing == Addressing_IPv4Unnumbered)
}

func (r *NetworkDesign) IsISLIPv6Enabled() bool {
	return r.Spec.Interfaces != nil && r.Spec.Interfaces.ISL != nil &&
		(r.Spec.Interfaces.ISL.Addressing == Addressing_DualStack ||
			r.Spec.Interfaces.ISL.Addressing == Addressing_IPv6Only ||
			r.Spec.Interfaces.ISL.Addressing == Addressing_IPv6Unnumbered)
}

func (r *NetworkDesign) IsISLIPv6Numbered() bool {
	return r.Spec.Interfaces != nil && r.Spec.Interfaces.ISL != nil &&
		(r.Spec.Interfaces.ISL.Addressing == Addressing_DualStack ||
			r.Spec.Interfaces.ISL.Addressing == Addressing_IPv6Only)
}

func (r *NetworkDesign) IsISLIPv6UnNumbered() bool {
	return r.Spec.Interfaces != nil && r.Spec.Interfaces.ISL != nil &&
		(r.Spec.Interfaces.ISL.Addressing == Addressing_IPv6Unnumbered)
}

// GetNodeIPClaims get the ip claims based on the addressing that is enabled in the network design
// the clientObject is typically the network object and the node
func (r *NetworkDesign) GetNodeIPClaims(cr, o client.Object) []*ipambev1alpha1.IPClaim {
	ipclaims := make([]*ipambev1alpha1.IPClaim, 0)

	nodeID := infrabev1alpha1.String2NodeGroupNodeID(o.GetName())

	if r.IsLoopbackIPv4Enabled() {
		// IPv4 enabled
		ipclaims = append(ipclaims, ipambev1alpha1.BuildIPClaim(
			metav1.ObjectMeta{
				Namespace: cr.GetNamespace(),
				Name:      fmt.Sprintf("%s.%s.ipv4", cr.GetName(), nodeID.Node), // network-name.nodenaame.ipv4
			},
			&ipambev1alpha1.IPClaimSpec{
				Index:      cr.GetName(),
				PrefixType: ptr.To[ipambev1alpha1.IPPrefixType](ipambev1alpha1.IPPrefixType_Pool),
				Owner:      commonv1alpha1.GetOwnerReference(cr),
				ClaimLabels: commonv1alpha1.ClaimLabels{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							backend.KuidINVPurpose:          "loopback",
							backend.KuidIPAMddressFamilyKey: "ipv4",
						},
					},
				},
			},
			nil,
		))
	} else {
		// we allocate an ipv4 address for the routerid
		ipclaims = append(ipclaims, ipambev1alpha1.BuildIPClaim(
			metav1.ObjectMeta{
				Namespace: cr.GetNamespace(),
				Name:      fmt.Sprintf("%s.%s.routerid", cr.GetName(), nodeID.Node), // network-name.nodenaame.routerid
			},
			&ipambev1alpha1.IPClaimSpec{
				Index:      cr.GetName(),
				PrefixType: ptr.To[ipambev1alpha1.IPPrefixType](ipambev1alpha1.IPPrefixType_Pool),
				Owner:      commonv1alpha1.GetOwnerReference(cr),
				ClaimLabels: commonv1alpha1.ClaimLabels{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							backend.KuidINVPurpose:          "loopback",
							backend.KuidIPAMddressFamilyKey: "ipv4",
						},
					},
				},
			},
			nil,
		))
	}

	if r.IsLoopbackIPv6Enabled() {
		ipclaims = append(ipclaims, ipambev1alpha1.BuildIPClaim(
			metav1.ObjectMeta{
				Namespace: o.GetNamespace(),
				Name:      fmt.Sprintf("%s.%s.ipv6", cr.GetName(), nodeID.Node),
			},
			&ipambev1alpha1.IPClaimSpec{
				Index:      cr.GetName(),
				PrefixType: ptr.To[ipambev1alpha1.IPPrefixType](ipambev1alpha1.IPPrefixType_Pool),
				Owner:      commonv1alpha1.GetOwnerReference(o),
				ClaimLabels: commonv1alpha1.ClaimLabels{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							backend.KuidINVPurpose:          "loopback",
							backend.KuidIPAMddressFamilyKey: "ipv6",
						},
					},
				},
			},
			nil,
		))
	}
	return ipclaims
}

func (r *NetworkDesign) GetNodeASClaim(cr, o client.Object) *asbev1alpha1.ASClaim {
	nodeID := infrabev1alpha1.String2NodeGroupNodeID(o.GetName())

	fmt.Println("nc GetNodeASClaim", r.Spec.Protocols)

	if r.Spec.Protocols != nil && r.Spec.Protocols.EBGP != nil && r.Spec.Protocols.EBGP.ASPool != nil {
		return asbev1alpha1.BuildASClaim(
			metav1.ObjectMeta{
				Namespace: r.Namespace,
				Name:      fmt.Sprintf("%s.%s", cr.GetName(), nodeID.Node), // network.node
			},
			&asbev1alpha1.ASClaimSpec{
				Index: cr.GetName(),
				Owner: commonv1alpha1.GetOwnerReference(r),
				ClaimLabels: commonv1alpha1.ClaimLabels{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							backend.KuidClaimNameKey: fmt.Sprintf("%s.aspool", cr.GetName()),
						},
					},
				},
			},
			nil,
		)

	}
	return nil
}

func getReducedLinkName(cr client.Object, link *infrabev1alpha1.Link) string {
	var b strings.Builder
	for idx, ep := range link.Spec.Endpoints {
		if idx == 0 {
			b.WriteString(cr.GetName())
		}
		b.WriteString(fmt.Sprintf(".%s.%s", ep.Node, ep.Endpoint))
	}
	return b.String()
}

func (r *NetworkDesign) GetLinkIPClaims(cr client.Object, link *infrabev1alpha1.Link) []*ipambev1alpha1.IPClaim {
	ipclaims := make([]*ipambev1alpha1.IPClaim, 0)
	linkName := getReducedLinkName(cr, link)
	if r.IsISLIPv4Numbered() {
		ipClaimLinkName := fmt.Sprintf("%s.ipv4", linkName) // linkName.ipv4
		ipclaims = append(ipclaims, ipambev1alpha1.BuildIPClaim(
			metav1.ObjectMeta{
				Namespace: cr.GetNamespace(),
				Name:      ipClaimLinkName,
			},
			&ipambev1alpha1.IPClaimSpec{
				Index:         cr.GetName(),
				PrefixType:    ptr.To[ipambev1alpha1.IPPrefixType](ipambev1alpha1.IPPrefixType_Network),
				AddressFamily: ptr.To[iputil.AddressFamily](iputil.AddressFamilyIpv4),
				CreatePrefix:  ptr.To(true),
				PrefixLength:  ptr.To[uint32](31),
				Owner:         commonv1alpha1.GetOwnerReference(cr),
				ClaimLabels: commonv1alpha1.ClaimLabels{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							backend.KuidINVPurpose:          string(InterfaceKindISL),
							backend.KuidIPAMddressFamilyKey: "ipv4",
						},
					},
				},
			},
			nil,
		))

		for _, ep := range link.Spec.Endpoints {
			ipclaims = append(ipclaims, ipambev1alpha1.BuildIPClaim(
				metav1.ObjectMeta{
					Namespace: cr.GetNamespace(),
					Name:      fmt.Sprintf("%s.%s.%s.ipv4", cr.GetName(), ep.Node, ep.Endpoint), // epName.ipv4
				},
				&ipambev1alpha1.IPClaimSpec{
					Index:      cr.GetName(),
					PrefixType: ptr.To[ipambev1alpha1.IPPrefixType](ipambev1alpha1.IPPrefixType_Network),
					Owner:      commonv1alpha1.GetOwnerReference(cr),
					ClaimLabels: commonv1alpha1.ClaimLabels{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								backend.KuidClaimNameKey:        ipClaimLinkName,
								backend.KuidIPAMddressFamilyKey: "ipv4",
							},
						},
					},
				},
				nil,
			))
		}
	}
	if r.IsISLIPv6Numbered() {
		ipClaimLinkName := fmt.Sprintf("%s.ipv6", linkName) // linkName.ipv4
		ipclaims = append(ipclaims, ipambev1alpha1.BuildIPClaim(
			metav1.ObjectMeta{
				Namespace: cr.GetNamespace(),
				Name:      ipClaimLinkName,
			},
			&ipambev1alpha1.IPClaimSpec{
				Index:         cr.GetName(),
				PrefixType:    ptr.To[ipambev1alpha1.IPPrefixType](ipambev1alpha1.IPPrefixType_Network),
				AddressFamily: ptr.To[iputil.AddressFamily](iputil.AddressFamilyIpv6),
				CreatePrefix:  ptr.To(true),
				PrefixLength:  ptr.To[uint32](127),
				Owner:         commonv1alpha1.GetOwnerReference(cr),
				ClaimLabels: commonv1alpha1.ClaimLabels{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							backend.KuidINVPurpose:          string(InterfaceKindISL),
							backend.KuidIPAMddressFamilyKey: "ipv6",
						},
					},
				},
			},
			nil,
		))

		for _, ep := range link.Spec.Endpoints {
			ipclaims = append(ipclaims, ipambev1alpha1.BuildIPClaim(
				metav1.ObjectMeta{
					Namespace: cr.GetNamespace(),
					Name:      fmt.Sprintf("%s.%s.%s.ipv6", cr.GetName(), ep.Node, ep.Endpoint), // epName.ipv6
				},
				&ipambev1alpha1.IPClaimSpec{
					Index:      cr.GetName(),
					PrefixType: ptr.To[ipambev1alpha1.IPPrefixType](ipambev1alpha1.IPPrefixType_Network),
					Owner:      commonv1alpha1.GetOwnerReference(cr),
					ClaimLabels: commonv1alpha1.ClaimLabels{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								backend.KuidClaimNameKey:        ipClaimLinkName,
								backend.KuidIPAMddressFamilyKey: "ipv6",
							},
						},
					},
				},
				nil,
			))
		}
	}

	return ipclaims
}

// BuildNetworkDesign returns an NetworkDesign from a client Object a Spec/Status
func BuildNetworkDesign(meta metav1.ObjectMeta, spec *NetworkDesignSpec, status *NetworkDesignStatus) *NetworkDesign {
	aspec := NetworkDesignSpec{}
	if spec != nil {
		aspec = *spec
	}
	astatus := NetworkDesignStatus{}
	if status != nil {
		astatus = *status
	}
	return &NetworkDesign{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeGroupVersion.Identifier(),
			Kind:       NetworkDesignKind,
		},
		ObjectMeta: meta,
		Spec:       aspec,
		Status:     astatus,
	}
}

func (r *NetworkDesign) IsISISEnabled() bool {
	return r.Spec.Protocols != nil && r.Spec.Protocols.ISIS != nil
}

func (r *NetworkDesign) IsOSPFEnabled() bool {
	return r.Spec.Protocols != nil && r.Spec.Protocols.OSPF != nil
}

func (r *NetworkDesign) IsEBGPEnabled() bool {
	return r.Spec.Protocols != nil && r.Spec.Protocols.EBGP != nil
}

func (r *NetworkDesign) GetIBGPAS() uint32 {
	if r.Spec.Protocols != nil && r.Spec.Protocols.IBGP != nil && r.Spec.Protocols.IBGP.AS != nil {
		return *r.Spec.Protocols.IBGP.AS
	}
	return 0
}

func (r *NetworkDesign) IsIBGPEnabled() bool {
	return r.Spec.Protocols != nil && r.Spec.Protocols.IBGP != nil
}

func (r *NetworkDesign) GetLoopbackPrefixesPerAF() ([]string, []string) {
	ipv4Prefixes := []string{}
	ipv6Prefixes := []string{}

	if r.Spec.Interfaces != nil && r.Spec.Interfaces.Loopback != nil {
		for _, prefix := range r.Spec.Interfaces.Loopback.Prefixes {
			pi, err := iputil.New(prefix.Prefix)
			if err != nil {
				continue
			}
			if pi.IsIpv4() {
				ipv4Prefixes = append(ipv4Prefixes, prefix.Prefix)

			} else {
				ipv6Prefixes = append(ipv6Prefixes, prefix.Prefix)
			}
		}
	}

	return ipv4Prefixes, ipv6Prefixes
}

func (r *NetworkDesign) GetLoopbackPrefixes() []string {
	prefixes := []string{}
	if r.Spec.Interfaces != nil && r.Spec.Interfaces.Loopback != nil {
		for _, prefix := range r.Spec.Interfaces.Loopback.Prefixes {
			prefixes = append(prefixes, prefix.Prefix)
		}
	}
	return prefixes
}

func (r *NetworkDesign) IsVXLANEnabled() bool {
	return r.Spec.Encapsultation != nil && r.Spec.Encapsultation.VXLAN != nil
}

func (r *NetworkDesign) IsMPLSLDPEnabled() bool {
	return r.Spec.Encapsultation != nil && r.Spec.Encapsultation.MPLS != nil && r.Spec.Encapsultation.MPLS.LDP != nil
}

func (r *NetworkDesign) IsMPLSSREnabled() bool {
	return r.Spec.Encapsultation != nil && r.Spec.Encapsultation.MPLS != nil && r.Spec.Encapsultation.MPLS.SR != nil
}

func (r *NetworkDesign) IsMPLSRSVPEnabled() bool {
	return r.Spec.Encapsultation != nil && r.Spec.Encapsultation.MPLS != nil && r.Spec.Encapsultation.MPLS.RSVP != nil
}

func (r *NetworkDesign) IsSRv6Enabled() bool {
	return r.Spec.Encapsultation != nil && r.Spec.Encapsultation.SRV6 != nil
}

func (r *NetworkDesign) IsSRv6USIDEnabled() bool {
	return r.Spec.Encapsultation != nil && r.Spec.Encapsultation.SRV6 != nil && r.Spec.Encapsultation.SRV6.MicroSID != nil
}

func (r *NetworkDesign) IsBGPEVPNEnabled() bool {
	return r.Spec.Protocols != nil && r.Spec.Protocols.BGPEVPN != nil
}

func (r *NetworkDesign) IsBGPIPVPNv4Enabled() bool {
	return r.Spec.Protocols != nil && r.Spec.Protocols.BGPVPNv4 != nil
}

func (r *NetworkDesign) IsBGPIPVPNv6Enabled() bool {
	return r.Spec.Protocols != nil && r.Spec.Protocols.BGPVPNv6 != nil
}

func (r *NetworkDesign) IsBGPRouteTargetEnabled() bool {
	return r.Spec.Protocols != nil && r.Spec.Protocols.BGPRouteTarget != nil
}

func (r *NetworkDesign) IsBGPLabelUnicastv4Enabled() bool {
	return r.Spec.Protocols != nil && r.Spec.Protocols.BGPLabeledUnicastv4 != nil
}

func (r *NetworkDesign) IsBGPLabelUnicastv6Enabled() bool {
	return r.Spec.Protocols != nil && r.Spec.Protocols.BGPLabeledUnicastv6 != nil
}

/*
func (r *NetworkDesign) GetOverlayProtocols() []string {
	overlayProtocols := []string{}
	if r.IsBGPEVPNEnabled() {
		overlayProtocols = append(overlayProtocols, "evpn")
	}
	return overlayProtocols
}
*/

func (r *NetworkDesign) GetUnderlayAddressFamiliesToBeDisabled() []string {
	afs := []string{}
	if r.IsISISEnabled() || r.IsOSPFEnabled() {
	} else {
		if !r.IsISLIPv4Enabled() || !r.IsLoopbackIPv4Enabled() {
			afs = append(afs, "ipv4-unicast")
		}
		if !r.IsISLIPv6Enabled() || !r.IsLoopbackIPv6Enabled() {
			afs = append(afs, "ipv6-unicast")
		}
	}
	if r.IsBGPEVPNEnabled() {
		afs = append(afs, "evpn")
	}
	if r.IsBGPIPVPNv4Enabled() {
		afs = append(afs, "l3vpn-ipv4-unicast")
	}
	if r.IsBGPIPVPNv6Enabled() {
		afs = append(afs, "l3vpn-ipv6-unicast")
	}
	if r.IsBGPRouteTargetEnabled() {
		afs = append(afs, "route-target")
	}
	if r.IsBGPLabelUnicastv4Enabled() {
		afs = append(afs, "ipv4-labeled-unicast")
	}
	if r.IsBGPLabelUnicastv6Enabled() {
		afs = append(afs, "ipv6-labeled-unicast")
	}
	return afs
}

func (r *NetworkDesign) GetOverlayAddressFamiliesToBeDisabled() []string {
	afs := []string{}
	if r.IsISISEnabled() || r.IsOSPFEnabled() {
	} else {
		if r.IsISLIPv4Enabled() || r.IsLoopbackIPv4Enabled() {
			afs = append(afs, "ipv4-unicast")
		}
		if r.IsISLIPv6Enabled() || r.IsLoopbackIPv6Enabled() {
			afs = append(afs, "ipv6-unicast")
		}
	}
	// we dont need to disable the specific overlay address families
	return afs
}

// GetAllAddressFamilies retrun all address families enabled in the network design
func (r *NetworkDesign) GetAllEnabledAddressFamilies() []string {
	afs := []string{}
	if !r.IsISISEnabled() && !r.IsOSPFEnabled() && (r.IsISLIPv4Enabled() || r.IsLoopbackIPv4Enabled()) {
		afs = append(afs, "ipv4-unicast")
	}
	if !r.IsISISEnabled() && !r.IsOSPFEnabled() && (r.IsISLIPv6Enabled() || r.IsLoopbackIPv6Enabled()) {
		afs = append(afs, "ipv6-unicast")
	}
	if r.IsBGPEVPNEnabled() {
		afs = append(afs, "evpn")
	}
	if r.IsBGPIPVPNv4Enabled() {
		afs = append(afs, "l3vpn-ipv4-unicast")
	}
	if r.IsBGPIPVPNv6Enabled() {
		afs = append(afs, "l3vpn-ipv6-unicast")
	}
	if r.IsBGPRouteTargetEnabled() {
		afs = append(afs, "route-target")
	}
	if r.IsBGPLabelUnicastv4Enabled() {
		afs = append(afs, "ipv4-labeled-unicast")
	}
	if r.IsBGPLabelUnicastv6Enabled() {
		afs = append(afs, "ipv6-labeled-unicast")
	}
	return afs
}

// GetIGPAddressFamilies retrun all address families enabled in the network design
func (r *NetworkDesign) GetIGPAddressFamilies() []string {
	afs := []string{}
	if r.IsISLIPv4Enabled() || r.IsLoopbackIPv4Enabled() {
		afs = append(afs, "ipv4-unicast")
	}
	if r.IsISLIPv6Enabled() || r.IsLoopbackIPv6Enabled() {
		afs = append(afs, "ipv6-unicast")
	}
	return afs
}
