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
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetCondition returns the condition based on the condition kind
func (r *NetworkDevice) GetCondition(t conditionv1alpha1.ConditionType) conditionv1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *NetworkDevice) SetConditions(c ...conditionv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

func (r *NetworkDevice) Validate() error {
	return nil
}

// BuildNetwork returns an Network from a client Object a Spec/Status
func BuildNetworkDevice(meta metav1.ObjectMeta, spec *NetworkDeviceSpec, status *NetworkDeviceStatus) *NetworkDevice {
	aspec := NetworkDeviceSpec{}
	if spec != nil {
		aspec = *spec
	}
	astatus := NetworkDeviceStatus{}
	if status != nil {
		astatus = *status
	}
	return &NetworkDevice{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeGroupVersion.Identifier(),
			Kind:       NetworkDeviceKind,
		},
		ObjectMeta: meta,
		Spec:       aspec,
		Status:     astatus,
	}
}
