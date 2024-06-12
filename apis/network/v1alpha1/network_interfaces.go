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
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strings"

	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// GetCondition returns the condition based on the condition kind
func (r *Network) GetCondition(t conditionv1alpha1.ConditionType) conditionv1alpha1.Condition {
	return r.Status.GetCondition(t)
}

// SetConditions sets the conditions on the resource. it allows for 0, 1 or more conditions
// to be set at once
func (r *Network) SetConditions(c ...conditionv1alpha1.Condition) {
	r.Status.SetConditions(c...)
}

func (r *Network) SetOverallStatus() {
	ready := true
	msg := ""
	condition := r.GetCondition(ConditionTypeNetworkParamReady)
	if ready && condition.Status == metav1.ConditionFalse {
		ready = false
		msg = fmt.Sprintf("network parameters not ready: %s", condition.Message)
	}
	condition = r.GetCondition(conditionv1alpha1.ConditionTypeDeviceConfigReady)
	if ready && condition.Status == metav1.ConditionFalse {
		ready = false
		msg = fmt.Sprintf("device configs for network not ready: %s", condition.Message)
	}
	if ready {
		r.Status.SetConditions(conditionv1alpha1.Ready())
	} else {
		r.Status.SetConditions(conditionv1alpha1.Failed(msg))
	}
}

func (r *Network) Validate() error {
	return nil
}

// BuildNetwork returns an Network from a client Object a Spec/Status
func BuildNetwork(meta metav1.ObjectMeta, spec *NetworkSpec, status *NetworkStatus) *Network {
	aspec := NetworkSpec{}
	if spec != nil {
		aspec = *spec
	}
	astatus := NetworkStatus{}
	if status != nil {
		astatus = *status
	}
	return &Network{
		TypeMeta: metav1.TypeMeta{
			APIVersion: SchemeGroupVersion.Identifier(),
			Kind:       NetworkKind,
		},
		ObjectMeta: meta,
		Spec:       aspec,
		Status:     astatus,
	}
}

func (r *Network) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: r.Namespace, Name: r.Name}
}

func (r *Network) GetNetworkName() string {
	parts := strings.Split(r.Name, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func (r *Network) IsDefaultNetwork() bool {
	networkName := r.GetNetworkName()
	return networkName == DefaultNetwork
}

func (r *NetworkInterface) IsDynamic() bool {
	return r.Selector != nil
}

func (r *Network) IsBridgePresent(name string) bool {
	for _, bd := range r.Spec.Bridges {
		if bd.Name == name {
			return true
		}
	}
	return false
}

func (r *Network) DidAChildConditionFail() bool {
	for _, childCondition := range ChildConditions {
		if r.GetCondition(childCondition).Reason == string(ConditionReasonFailed) {
			return true
		}
	}
	return true
}

func (r *Network) GetFailedMessage() string {
	var sb strings.Builder
	first := true
	for _, childCondition := range ChildConditions {
		condition := r.GetCondition(childCondition)
		if condition.Reason == string(ConditionReasonFailed) {
			if first {
				sb.WriteString(";")
				first = false
			}
			sb.WriteString(fmt.Sprintf("condition: %s failed, msg %s", string(childCondition), condition.Message))
		}
	}
	return sb.String()
}

func (r *Network) GetProcessingMessage() string {
	var sb strings.Builder
	first := true
	for _, childCondition := range ChildConditions {
		condition := r.GetCondition(childCondition)
		if condition.Status == metav1.ConditionFalse {
			if first {
				sb.WriteString(";")
				first = false
			}
			sb.WriteString(fmt.Sprintf("condition: %s still processing", string(childCondition)))
		}
	}
	return sb.String()
}

func (r *Network) AreChildConditionsReady() bool {
	for _, childCondition := range ChildConditions {
		if r.GetCondition(childCondition).Status == metav1.ConditionFalse {
			return false
		}
	}
	return true
}

func (r *Network) CalculateHash() ([sha1.Size]byte, error) {
	// Convert the struct to JSON
	jsonData, err := json.Marshal(r.Spec)
	if err != nil {
		return [sha1.Size]byte{}, err
	}

	// Calculate SHA-1 hash
	return sha1.Sum(jsonData), nil
}
