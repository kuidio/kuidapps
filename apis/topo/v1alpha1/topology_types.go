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
	"reflect"

	infravbe1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TopologySpec defines the desired state of Topology
type TopologySpec struct {
	// Region identifies the default region this topology is located in
	// +optional
	Region *string `json:"region,omitempty" yaml:"region,omitempty" protobuf:"bytes,1,opt,name=region"`
	// Site identifies the default site this topology is located in
	// +optional
	Site *string `json:"site,omitempty" yaml:"site,omitempty" protobuf:"bytes,1,opt,name=site"`
	// Location identifies the default location this topology is located in
	// +optional
	Location *infravbe1alpha1.Location `json:"location,omitempty" yaml:"location,omitempty"`
	// ContainerLab holds the containerlab topology
	ContainerLab *string `json:"containerLab,omitempty" yaml:"containerLab,omitempty" protobuf:"bytes,1,opt,name=containerLab"`
}

// TopologyStatus defines the observed state of Topology
type TopologyStatus struct {
	// ConditionedStatus provides the status of the TopologyClab using conditions
	// - a ready condition indicates the overall status of the resource
	conditionv1alpha1.ConditionedStatus `json:",inline" yaml:",inline" protobuf:"bytes,1,opt,name=conditionedStatus"`
}

// +kubebuilder:object:root=true
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:resource:categories={kuid, topo}
// Topology is the Topology for the Topology API
// +k8s:openapi-gen=true
type Topology struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   TopologySpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status TopologyStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +kubebuilder:object:root=true
// TopologyClabList contains a list of TopologyClabs
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TopologyList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Topology `json:"items" yaml:"items" protobuf:"bytes,2,rep,name=items"`
}

var (
	TopologyKind = reflect.TypeOf(Topology{}).Name()
)
