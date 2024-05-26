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

const (
	// ConditionTypeReady represents the resource ready condition
	ConditionTypeNetworkParamReady  conditionv1alpha1.ConditionType = "NetworkPararmReady"
	ConditionTypeNetworkDeviceReady conditionv1alpha1.ConditionType = "NetworkDeviceReady"
	ConditionTypeNetworkDeployReady conditionv1alpha1.ConditionType = "NetworkDeployReady"
)

var ChildConditions = []conditionv1alpha1.ConditionType{
	ConditionTypeNetworkParamReady,
	ConditionTypeNetworkDeviceReady,
}

// A ConditionReason represents the reason a resource is in a condition.
type ConditionReason string

// Reasons a resource is ready or not
const (
	ConditionReasonReady   ConditionReason = "Ready"
	ConditionReasonFailed  ConditionReason = "Failed"
	ConditionReasonUnknown ConditionReason = "Unknown"
)

// NetworkParamReady returns a condition that indicates the resource is
// satofying this condition
func NetworkParamReady() conditionv1alpha1.Condition {
	return conditionv1alpha1.Condition{
		Condition: metav1.Condition{
			Type:               string(ConditionTypeNetworkParamReady),
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             string(ConditionReasonReady),
		}}
}

// NetworkParamFailed returns a condition that indicates the resource is
// not satisfying this condition
func NetworkParamFailed(msg string) conditionv1alpha1.Condition {
	return conditionv1alpha1.Condition{
		Condition: metav1.Condition{
			Type:               string(ConditionTypeNetworkParamReady),
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             string(ConditionReasonFailed),
			Message:            msg,
		}}
}

// NetworkDeviceReady returns a condition that indicates the resource is
// satofying this condition
func NetworkDeviceReady() conditionv1alpha1.Condition {
	return conditionv1alpha1.Condition{
		Condition: metav1.Condition{
			Type:               string(ConditionTypeNetworkDeviceReady),
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             string(ConditionReasonReady),
		}}
}

// NetworkDeviceProcessing returns a condition that indicates the resource is
// not satisfying this condition
func NetworkDeviceProcessing(msg string) conditionv1alpha1.Condition {
	return conditionv1alpha1.Condition{
		Condition: metav1.Condition{
			Type:               string(ConditionTypeNetworkDeviceReady),
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Message:            msg,
			Reason:             string(conditionv1alpha1.ConditionReasonProcessing),
		}}
}

// NetworkDeviceFailed returns a condition that indicates the resource is
// not satisfying this condition
func NetworkDeviceFailed(msg string) conditionv1alpha1.Condition {
	return conditionv1alpha1.Condition{
		Condition: metav1.Condition{
			Type:               string(ConditionTypeNetworkParamReady),
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             string(ConditionReasonFailed),
			Message:            msg,
		}}
}

/*
// NetworkDeviceReady returns a condition that indicates the resource is
// satofying this condition
func NetworkDeployReady() conditionv1alpha1.Condition {
	return conditionv1alpha1.Condition{
		Condition: metav1.Condition{
			Type:               string(ConditionTypeNetworkDeployReady),
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             string(ConditionReasonReady),
		}}
}

// NetworkDeployProcessing returns a condition that indicates the resource is
// not satisfying this condition
func NetworkDeployProcessing(msg string) conditionv1alpha1.Condition {
	return conditionv1alpha1.Condition{
		Condition: metav1.Condition{
			Type:               string(ConditionTypeNetworkDeployReady),
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Message:            msg,
			Reason:             string(conditionv1alpha1.ConditionReasonProcessing),
		}}
}

// NetworkDeployFailed returns a condition that indicates the resource is
// not satisfying this condition
func NetworkDeployFailed(msg string) conditionv1alpha1.Condition {
	return conditionv1alpha1.Condition{
		Condition: metav1.Condition{
			Type:               string(ConditionTypeNetworkDeployReady),
			Status:             metav1.ConditionFalse,
			LastTransitionTime: metav1.Now(),
			Reason:             string(ConditionReasonFailed),
			Message:            msg,
		}}
}
*/
