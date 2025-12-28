/*
Copyright 2025.

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

package controller

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/v1alpha1"
)

const (
	// Condition Types
	ConditionTypeInitialized = "Initialized"
	ConditionTypeActive      = "Active"
	ConditionTypeFailed      = "Failed"

	// Condition Reasons
	ReasonInferenceServiceCreating   = "InferenceServiceCreating"
	ReasonInferenceServiceProcessing = "InferenceServiceProcessing"
	ReasonInferenceServiceAvailable  = "InferenceServiceAvailable"
	ReasonInferenceServiceFailed     = "InferenceServiceFailed"
)

// setInitCondition sets InferenceService condition to initialized
func setInitCondition(inferSvc *fusioninferiov1alpha1.InferenceService) {
	meta.SetStatusCondition(&inferSvc.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeInitialized,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonInferenceServiceCreating,
		Message:            "InferenceService initialized",
		LastTransitionTime: metav1.Now(),
	})
	inferSvc.Status.ObservedGeneration = inferSvc.Generation
}

// setProcessingCondition sets InferenceService condition to processing
func setProcessingCondition(inferSvc *fusioninferiov1alpha1.InferenceService) {
	meta.SetStatusCondition(&inferSvc.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeActive,
		Status:             metav1.ConditionFalse,
		Reason:             ReasonInferenceServiceProcessing,
		Message:            "InferenceService is being reconciled",
		LastTransitionTime: metav1.Now(),
	})
	inferSvc.Status.ObservedGeneration = inferSvc.Generation
}

// setFailedCondition sets InferenceService condition to failed
func setFailedCondition(inferSvc *fusioninferiov1alpha1.InferenceService, err error) {
	meta.SetStatusCondition(&inferSvc.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeFailed,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonInferenceServiceFailed,
		Message:            err.Error(),
		LastTransitionTime: metav1.Now(),
	})
	inferSvc.Status.ObservedGeneration = inferSvc.Generation
}

// setActiveCondition sets InferenceService condition to active (all components ready)
func setActiveCondition(inferSvc *fusioninferiov1alpha1.InferenceService) {
	meta.SetStatusCondition(&inferSvc.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeActive,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonInferenceServiceAvailable,
		Message:            "InferenceService is ready",
		LastTransitionTime: metav1.Now(),
	})
	inferSvc.Status.ObservedGeneration = inferSvc.Generation
}
