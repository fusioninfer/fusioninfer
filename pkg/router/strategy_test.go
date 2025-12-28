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
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/core/v1alpha1"
)

func TestGenerateEPPConfig_CustomConfig(t *testing.T) {
	customConfig := `apiVersion: inference.networking.x-k8s.io/v1alpha1
kind: EndpointPickerConfig
plugins:
- type: custom-plugin
`
	inferSvc := &fusioninferiov1alpha1.InferenceService{}
	role := fusioninferiov1alpha1.Role{
		EndpointPickerConfig: customConfig,
	}

	got := GenerateEPPConfig(inferSvc, role)
	if got != customConfig {
		t.Errorf("expected custom config to be returned as-is")
	}
}

func TestGenerateEPPConfig_PrefixCache(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{}
	role := fusioninferiov1alpha1.Role{
		Strategy: fusioninferiov1alpha1.StrategyPrefixCache,
	}

	got := GenerateEPPConfig(inferSvc, role)

	// Verify it contains prefix-cache-scorer plugin
	if !strings.Contains(got, "prefix-cache-scorer") {
		t.Error("expected prefix-cache-scorer plugin in config")
	}
	if !strings.Contains(got, "max-score-picker") {
		t.Error("expected max-score-picker plugin in config")
	}
}

func TestGenerateEPPConfig_KVCacheUtil(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{}
	role := fusioninferiov1alpha1.Role{
		Strategy: fusioninferiov1alpha1.StrategyKVCacheUtil,
	}

	got := GenerateEPPConfig(inferSvc, role)

	if !strings.Contains(got, "kv-cache-utilization-scorer") {
		t.Error("expected kv-cache-utilization-scorer plugin in config")
	}
}

func TestGenerateEPPConfig_QueueSize(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{}
	role := fusioninferiov1alpha1.Role{
		Strategy: fusioninferiov1alpha1.StrategyQueueSize,
	}

	got := GenerateEPPConfig(inferSvc, role)

	if !strings.Contains(got, "queue-scorer") {
		t.Error("expected queue-scorer plugin in config")
	}
}

func TestGenerateEPPConfig_LoRAffinity(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{}
	role := fusioninferiov1alpha1.Role{
		Strategy: fusioninferiov1alpha1.StrategyLoRAffinity,
	}

	got := GenerateEPPConfig(inferSvc, role)

	if !strings.Contains(got, "lora-affinity-scorer") {
		t.Error("expected lora-affinity-scorer plugin in config")
	}
}

func TestGenerateEPPConfig_PDDisaggregation(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pd-service",
		},
		Spec: fusioninferiov1alpha1.InferenceServiceSpec{
			Roles: []fusioninferiov1alpha1.Role{
				{
					Name:          "prefill",
					ComponentType: fusioninferiov1alpha1.ComponentTypePrefiller,
				},
				{
					Name:          "decode",
					ComponentType: fusioninferiov1alpha1.ComponentTypeDecoder,
				},
			},
		},
	}
	role := fusioninferiov1alpha1.Role{
		Strategy: fusioninferiov1alpha1.StrategyPDDisaggregation,
	}

	got := GenerateEPPConfig(inferSvc, role)

	// Verify PD disaggregation specific plugins
	if !strings.Contains(got, "pd-profile-handler") {
		t.Error("expected pd-profile-handler plugin in config")
	}
	if !strings.Contains(got, "prefill-header-handler") {
		t.Error("expected prefill-header-handler plugin in config")
	}
	if !strings.Contains(got, "by-label") {
		t.Error("expected by-label plugin in config")
	}

	// Verify scheduling profiles for prefill and decode
	if !strings.Contains(got, "name: prefill") {
		t.Error("expected prefill scheduling profile")
	}
	if !strings.Contains(got, "name: decode") {
		t.Error("expected decode scheduling profile")
	}

	// Verify component type labels
	if !strings.Contains(got, string(fusioninferiov1alpha1.ComponentTypePrefiller)) {
		t.Error("expected prefiller component type in config")
	}
	if !strings.Contains(got, string(fusioninferiov1alpha1.ComponentTypeDecoder)) {
		t.Error("expected decoder component type in config")
	}
}

func TestGenerateEPPConfig_PDDisaggregation_FallbackWhenNotPD(t *testing.T) {
	// Non-PD service (only worker, no prefiller/decoder)
	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "monolithic-service",
		},
		Spec: fusioninferiov1alpha1.InferenceServiceSpec{
			Roles: []fusioninferiov1alpha1.Role{
				{
					Name:          "worker",
					ComponentType: fusioninferiov1alpha1.ComponentTypeWorker,
				},
			},
		},
	}
	role := fusioninferiov1alpha1.Role{
		Strategy: fusioninferiov1alpha1.StrategyPDDisaggregation,
	}

	got := GenerateEPPConfig(inferSvc, role)

	// Should fall back to prefix-cache since it's not actually PD disaggregated
	if strings.Contains(got, "pd-profile-handler") {
		t.Error("should not contain pd-profile-handler for non-PD service")
	}
	if !strings.Contains(got, "prefix-cache-scorer") {
		t.Error("should fall back to prefix-cache-scorer")
	}
}

func TestGenerateEPPConfig_DefaultStrategy(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{}
	role := fusioninferiov1alpha1.Role{
		// No strategy set
	}

	got := GenerateEPPConfig(inferSvc, role)

	// Default should be prefix-cache
	if !strings.Contains(got, "prefix-cache-scorer") {
		t.Error("expected default strategy to be prefix-cache")
	}
}

