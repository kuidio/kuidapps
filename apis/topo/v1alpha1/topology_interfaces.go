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

	genidbev1alpha1 "github.com/kuidio/kuid/apis/backend/genid/v1alpha1"
	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetCondition returns the condition based on the condition kind
func (r *Topology) GetCondition(t conditionv1alpha1.ConditionType) conditionv1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *Topology) SetConditions(c ...conditionv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

func (r *Topology) Validate() error {
	return nil
}

func (r *Topology) GetRegion() string {
	if r.Spec.Region != nil {
		return *r.Spec.Region
	}
	return "defaultRegion1"
}

func (r *Topology) GetSite() string {
	if r.Spec.Site != nil {
		return *r.Spec.Site
	}
	return "defaultSite1"
}

func (r *Topology) GetSiteID() *infrabev1alpha1.SiteID {
	return &infrabev1alpha1.SiteID{
		Region: r.GetRegion(),
		Site:   r.GetSite(),
	}
}

// BuildTopology returns an Topology from a client Object a Spec/Status
func BuildTopology(meta metav1.ObjectMeta, spec *TopologySpec, status *TopologyStatus) *Topology {
	aspec := TopologySpec{}
	if spec != nil {
		aspec = *spec
	}
	astatus := TopologyStatus{}
	if status != nil {
		astatus = *status
	}
	return &Topology{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeGroupVersion.Identifier(),
			Kind:       TopologyKind,
		},
		ObjectMeta: meta,
		Spec:       aspec,
		Status:     astatus,
	}
}

func (r *Topology) GetGENIDIndex() *genidbev1alpha1.GENIDIndex {
	return genidbev1alpha1.BuildGENIDIndex(
		metav1.ObjectMeta{
			Namespace: r.Namespace,
			Name:      fmt.Sprintf("network.%s", r.Name), // network.topology
		},
		&genidbev1alpha1.GENIDIndexSpec{
			Type: "16bit",
		},
		nil,
	)
}