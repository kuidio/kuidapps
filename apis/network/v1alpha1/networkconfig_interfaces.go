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
func (r *NetworkConfig) GetCondition(t conditionv1alpha1.ConditionType) conditionv1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *NetworkConfig) SetConditions(c ...conditionv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

func (r *NetworkConfig) Validate() error {
	return nil
}

func (r *NetworkConfig) GetIPClaims() []*ipambev1alpha1.IPClaim {
	prefixes := make([]*ipambev1alpha1.IPClaim, 0, len(r.Spec.Prefixes))
	for _, prefix := range r.Spec.Prefixes {
		prefixes = append(prefixes, ipambev1alpha1.GetIPClaimFromPrefix(r, prefix))
	}
	return prefixes
}

func (r *NetworkConfig) GetASIndex() *asbev1alpha1.ASIndex {
	return asbev1alpha1.BuildASIndex(
		metav1.ObjectMeta{
			Namespace: r.Namespace,
			Name:      r.Name, // r.name = topology.<network> e.g default or vpc1, etc
		},
		nil,
		nil,
	)
}

func (r *NetworkConfig) GetGENIDClaim() *genidbev1alpha1.GENIDClaim {
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

func (r *NetworkConfig) GetASClaims() []*asbev1alpha1.ASClaim {
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

func (r *NetworkConfig) IsIPv4Enabled() bool {
	return r.Spec.Addressing == Addressing_DualStack || r.Spec.Addressing == Addressing_IPv4Only
}

func (r *NetworkConfig) IsIPv6Enabled() bool {
	return r.Spec.Addressing == Addressing_DualStack || r.Spec.Addressing == Addressing_IPv6Only
}

// GetNodeIPClaims get the ip claims based on the addressing that is enabled in the network config
// the clientObject is typically the network object and the node
func (r *NetworkConfig) GetNodeIPClaims(cr, o client.Object) []*ipambev1alpha1.IPClaim {
	ipclaims := make([]*ipambev1alpha1.IPClaim, 0)

	nodeID := infrabev1alpha1.String2NodeGroupNodeID(o.GetName())

	if r.IsIPv4Enabled() {
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

	if r.IsIPv6Enabled() {
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

func (r *NetworkConfig) GetNodeASClaim(cr, o client.Object) *asbev1alpha1.ASClaim {
	nodeID := infrabev1alpha1.String2NodeGroupNodeID(o.GetName())

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

func (r *NetworkConfig) GetLinkIPClaims(cr client.Object, link *infrabev1alpha1.Link) []*ipambev1alpha1.IPClaim {
	ipclaims := make([]*ipambev1alpha1.IPClaim, 0)
	linkName := getReducedLinkName(cr, link)
	if r.IsIPv4Enabled() {
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
							backend.KuidINVPurpose:          "link-internal",
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
	if r.IsIPv6Enabled() {
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
							backend.KuidINVPurpose:          "link-internal",
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

// BuildNetworkConfig returns an NetworkConfig from a client Object a Spec/Status
func BuildNetworkConfig(meta metav1.ObjectMeta, spec *NetworkConfigSpec, status *NetworkConfigStatus) *NetworkConfig {
	aspec := NetworkConfigSpec{}
	if spec != nil {
		aspec = *spec
	}
	astatus := NetworkConfigStatus{}
	if status != nil {
		astatus = *status
	}
	return &NetworkConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeGroupVersion.Identifier(),
			Kind:       NetworkConfigKind,
		},
		ObjectMeta: meta,
		Spec:       aspec,
		Status:     astatus,
	}
}

func (r *NetworkConfig) IsEBGPEnabled() bool {
	return r.Spec.Protocols != nil && r.Spec.Protocols.EBGP != nil
}

func (r *NetworkConfig) GetIBGPAS() uint32 {
	if r.Spec.Protocols != nil && r.Spec.Protocols.IBGP != nil && r.Spec.Protocols.IBGP.AS != nil {
		return *r.Spec.Protocols.IBGP.AS
	}
	return 0
}

func (r *NetworkConfig) IsIBGPEnabled() bool {
	return r.Spec.Protocols != nil && r.Spec.Protocols.IBGP != nil
}

func (r *NetworkConfig) GetLoopbackPrefixesPerAF() ([]string, []string) {
	ipv4Prefixes := []string{}
	ipv6Prefixes := []string{}
	for _, prefix := range r.Spec.Prefixes {
		pi, err := iputil.New(prefix.Prefix)
		if err != nil {
			continue
		}
		if pi.IsIpv4() {
			if prefix.Labels[backend.KuidINVPurpose] == "loopback" {
				ipv4Prefixes = append(ipv4Prefixes, prefix.Prefix)
			}
		} else {
			if prefix.Labels[backend.KuidINVPurpose] == "loopback" {
				ipv6Prefixes = append(ipv6Prefixes, prefix.Prefix)
			}
		}
	}
	return ipv4Prefixes, ipv6Prefixes
}

func (r *NetworkConfig) GetLoopbackPrefixes() []string {
	prefixes := []string{}
	for _, prefix := range r.Spec.Prefixes {
		if prefix.Labels[backend.KuidINVPurpose] == "loopback" {
			prefixes = append(prefixes, prefix.Prefix)
		}
	}
	return prefixes
}

func (r *NetworkConfig) IsVXLANEnabled() bool {
	return r.Spec.Encapsultation != nil && r.Spec.Encapsultation.VXLAN != nil
}

func (r *NetworkConfig) IsMPLSLDPEnabled() bool {
	return r.Spec.Encapsultation != nil && r.Spec.Encapsultation.MPLS != nil && r.Spec.Encapsultation.MPLS.LDP != nil
}

func (r *NetworkConfig) IsMPLSSREnabled() bool {
	return r.Spec.Encapsultation != nil && r.Spec.Encapsultation.MPLS != nil && r.Spec.Encapsultation.MPLS.SR != nil
}

func (r *NetworkConfig) IsMPLSRSVPEnabled() bool {
	return r.Spec.Encapsultation != nil && r.Spec.Encapsultation.MPLS != nil && r.Spec.Encapsultation.MPLS.RSVP != nil
}

func (r *NetworkConfig) ISSRv6Enabled() bool {
	return r.Spec.Encapsultation != nil && r.Spec.Encapsultation.SRV6 != nil
}

func (r *NetworkConfig) ISSRv6USIDEnabled() bool {
	return r.Spec.Encapsultation != nil && r.Spec.Encapsultation.SRV6 != nil && r.Spec.Encapsultation.SRV6.MicroSID != nil
}

func (r *NetworkConfig) ISBGPEVPNEnabled() bool {
	return r.Spec.Protocols != nil && r.Spec.Protocols.BGPEVPN != nil
}

func (r *NetworkConfig) GetOverlayProtocols() []string {
	overlayProtocols := []string{}
	if r.ISBGPEVPNEnabled() {
		overlayProtocols = append(overlayProtocols, "evpn")
	}
	return overlayProtocols
}
