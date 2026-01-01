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

package router

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	inferenceapi "sigs.k8s.io/gateway-api-inference-extension/api/v1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/core/v1alpha1"
	"github.com/fusioninfer/fusioninfer/pkg/workload"
)

func TestBuildInferencePool(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	workerRoles := []fusioninferiov1alpha1.Role{
		{
			Name:          "worker",
			ComponentType: fusioninferiov1alpha1.ComponentTypeWorker,
		},
	}

	pool := BuildInferencePool(inferSvc, workerRoles)

	// Verify pool name
	expectedName := "test-service-pool"
	if pool.Name != expectedName {
		t.Errorf("expected pool name %s, got %s", expectedName, pool.Name)
	}

	// Verify namespace
	if pool.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %s", pool.Namespace)
	}

	// Verify labels
	if pool.Labels[workload.LabelService] != "test-service" {
		t.Errorf("expected label %s=%s, got %s", workload.LabelService, "test-service", pool.Labels[workload.LabelService])
	}

	// Verify target port
	if len(pool.Spec.TargetPorts) != 1 {
		t.Fatalf("expected 1 target port, got %d", len(pool.Spec.TargetPorts))
	}
	if pool.Spec.TargetPorts[0].Number != inferenceapi.PortNumber(DefaultTargetPort) {
		t.Errorf("expected target port %d, got %d", DefaultTargetPort, pool.Spec.TargetPorts[0].Number)
	}

	// Verify EPP reference
	expectedEPPName := "test-service-epp"
	if string(pool.Spec.EndpointPickerRef.Name) != expectedEPPName {
		t.Errorf("expected EPP name %s, got %s", expectedEPPName, pool.Spec.EndpointPickerRef.Name)
	}
	if pool.Spec.EndpointPickerRef.Port.Number != inferenceapi.PortNumber(DefaultEPPPort) {
		t.Errorf("expected EPP port %d, got %d", DefaultEPPPort, pool.Spec.EndpointPickerRef.Port.Number)
	}

	// Verify selector includes service label
	if _, ok := pool.Spec.Selector.MatchLabels[inferenceapi.LabelKey(workload.LabelService)]; !ok {
		t.Error("expected selector to include service label")
	}

	// Verify selector includes worker-index=0 to select only leader pods
	workerIndexValue, ok := pool.Spec.Selector.MatchLabels[inferenceapi.LabelKey(LWSWorkerIndexLabel)]
	if !ok {
		t.Error("expected selector to include LWS worker-index label")
	}
	if string(workerIndexValue) != "0" {
		t.Errorf("expected worker-index=0, got %s", workerIndexValue)
	}
}

func TestBuildInferencePoolWithSingleWorkerRole(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	workerRoles := []fusioninferiov1alpha1.Role{
		{
			Name:          "inference",
			ComponentType: fusioninferiov1alpha1.ComponentTypeWorker,
		},
	}

	pool := BuildInferencePool(inferSvc, workerRoles)

	// Verify selector includes component type when single worker role
	componentTypeValue, ok := pool.Spec.Selector.MatchLabels[inferenceapi.LabelKey(workload.LabelComponentType)]
	if !ok {
		t.Error("expected selector to include component type label for single worker role")
	}
	if string(componentTypeValue) != string(fusioninferiov1alpha1.ComponentTypeWorker) {
		t.Errorf("expected component type %s, got %s", fusioninferiov1alpha1.ComponentTypeWorker, componentTypeValue)
	}
}

func TestBuildInferencePoolWithMultipleWorkerRoles(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pd-service",
			Namespace: "default",
		},
	}

	// PD disaggregated setup with prefiller and decoder
	workerRoles := []fusioninferiov1alpha1.Role{
		{
			Name:          "prefill",
			ComponentType: fusioninferiov1alpha1.ComponentTypePrefiller,
		},
		{
			Name:          "decode",
			ComponentType: fusioninferiov1alpha1.ComponentTypeDecoder,
		},
	}

	pool := BuildInferencePool(inferSvc, workerRoles)

	// Verify selector does NOT include component type for multiple worker roles
	if _, ok := pool.Spec.Selector.MatchLabels[inferenceapi.LabelKey(workload.LabelComponentType)]; ok {
		t.Error("selector should not include component type label for multiple worker roles")
	}
}

func TestGenerateHTTPRouteName(t *testing.T) {
	testCases := []struct {
		inferSvcName string
		want         string
	}{
		{"my-service", "my-service-httproute"},
		{"qwen-inference", "qwen-inference-httproute"},
	}

	for _, tc := range testCases {
		t.Run(tc.inferSvcName, func(t *testing.T) {
			got := GenerateHTTPRouteName(tc.inferSvcName)
			if got != tc.want {
				t.Errorf("GenerateHTTPRouteName(%s) = %s, want %s", tc.inferSvcName, got, tc.want)
			}
		})
	}
}
