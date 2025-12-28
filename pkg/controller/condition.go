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
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

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

// setInferenceServiceInitCondition sets InferenceService condition to initialized (first time only)
func (r *InferenceServiceReconciler) setInferenceServiceInitCondition(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService) error {
	meta.SetStatusCondition(&inferSvc.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeInitialized,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonInferenceServiceCreating,
		Message:            "InferenceService initialized",
		LastTransitionTime: metav1.Now(),
	})
	inferSvc.Status.ObservedGeneration = inferSvc.Generation
	if err := r.Status().Update(ctx, inferSvc); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to update InferenceService init condition")
		return err
	}
	return nil
}

// setInferenceServiceProcessingCondition sets InferenceService condition to processing
func (r *InferenceServiceReconciler) setInferenceServiceProcessingCondition(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService) error {
	meta.SetStatusCondition(&inferSvc.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeActive,
		Status:             metav1.ConditionFalse,
		Reason:             ReasonInferenceServiceProcessing,
		Message:            "InferenceService is being reconciled",
		LastTransitionTime: metav1.Now(),
	})
	inferSvc.Status.ObservedGeneration = inferSvc.Generation
	if err := r.Status().Update(ctx, inferSvc); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to update InferenceService processing condition")
		return err
	}
	return nil
}

// setInferenceServiceFailedCondition sets InferenceService condition to failed
func (r *InferenceServiceReconciler) setInferenceServiceFailedCondition(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService, err error) {
	meta.SetStatusCondition(&inferSvc.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeFailed,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonInferenceServiceFailed,
		Message:            err.Error(),
		LastTransitionTime: metav1.Now(),
	})
	if updateErr := r.Status().Update(ctx, inferSvc); updateErr != nil {
		logf.FromContext(ctx).Error(updateErr, "Failed to update InferenceService failed condition")
	}
}

// setInferenceServiceActiveCondition sets InferenceService condition to active (all components ready)
func (r *InferenceServiceReconciler) setInferenceServiceActiveCondition(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService) error {
	meta.SetStatusCondition(&inferSvc.Status.Conditions, metav1.Condition{
		Type:               ConditionTypeActive,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonInferenceServiceAvailable,
		Message:            "InferenceService is ready",
		LastTransitionTime: metav1.Now(),
	})
	inferSvc.Status.ObservedGeneration = inferSvc.Generation
	if err := r.Status().Update(ctx, inferSvc); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to update InferenceService active condition")
		return err
	}
	return nil
}
