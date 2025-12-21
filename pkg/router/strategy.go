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
	"fmt"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/v1alpha1"
	"github.com/fusioninfer/fusioninfer/pkg/scheduling"
)

// GenerateEPPConfig generates EndpointPickerConfig YAML based on the routing strategy
func GenerateEPPConfig(inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role) string {
	// If user provided custom config, use it directly
	if role.EndpointPickerConfig != "" {
		return role.EndpointPickerConfig
	}

	// Generate config based on strategy
	switch role.Strategy {
	case fusioninferiov1alpha1.StrategyPrefixCache:
		return generatePrefixCacheConfig()
	case fusioninferiov1alpha1.StrategyKVCacheUtil:
		return generateKVCacheUtilConfig()
	case fusioninferiov1alpha1.StrategyQueueSize:
		return generateQueueSizeConfig()
	case fusioninferiov1alpha1.StrategyLoRAffinity:
		return generateLoRAffinityConfig()
	case fusioninferiov1alpha1.StrategyPDDisaggregation:
		return generatePDDisaggregationConfig(inferSvc)
	default:
		// Default to prefix-cache strategy
		return generatePrefixCacheConfig()
	}
}

func generatePrefixCacheConfig() string {
	return `apiVersion: inference.networking.x-k8s.io/v1alpha1
kind: EndpointPickerConfig
plugins:
- type: prefix-cache-scorer
  parameters:
    blockSize: 5
    maxPrefixBlocksToMatch: 256
    lruCapacityPerServer: 31250
- type: max-score-picker
schedulingProfiles:
- name: default
  plugins:
  - pluginRef: max-score-picker
  - pluginRef: prefix-cache-scorer
    weight: 100
`
}

func generateKVCacheUtilConfig() string {
	return `apiVersion: inference.networking.x-k8s.io/v1alpha1
kind: EndpointPickerConfig
plugins:
- type: kv-cache-utilization-scorer
- type: max-score-picker
schedulingProfiles:
- name: default
  plugins:
  - pluginRef: max-score-picker
  - pluginRef: kv-cache-utilization-scorer
    weight: 100
`
}

func generateQueueSizeConfig() string {
	return `apiVersion: inference.networking.x-k8s.io/v1alpha1
kind: EndpointPickerConfig
plugins:
- type: queue-scorer
- type: max-score-picker
schedulingProfiles:
- name: default
  plugins:
  - pluginRef: max-score-picker
  - pluginRef: queue-scorer
    weight: 100
`
}

func generateLoRAffinityConfig() string {
	return `apiVersion: inference.networking.x-k8s.io/v1alpha1
kind: EndpointPickerConfig
plugins:
- type: lora-affinity-scorer
- type: max-score-picker
schedulingProfiles:
- name: default
  plugins:
  - pluginRef: max-score-picker
  - pluginRef: lora-affinity-scorer
    weight: 100
`
}

func generatePDDisaggregationConfig(inferSvc *fusioninferiov1alpha1.InferenceService) string {
	// Find prefiller and decoder component types
	prefillerLabel := string(fusioninferiov1alpha1.ComponentTypePrefiller)
	decoderLabel := string(fusioninferiov1alpha1.ComponentTypeDecoder)

	// Check if this is actually a PD disaggregated setup
	if !scheduling.IsPDDisaggregated(inferSvc) {
		// Fall back to prefix-cache if not PD disaggregated
		return generatePrefixCacheConfig()
	}

	return fmt.Sprintf(`apiVersion: inference.networking.x-k8s.io/v1alpha1
kind: EndpointPickerConfig
plugins:
- type: pd-profile-handler
  parameters:
    threshold: 0
    hashBlockSize: 5
    primaryPort: 8000
- type: prefill-header-handler
- type: by-label
  name: prefill-pods
  parameters:
    label: "fusioninfer.io/component-type"
    validValues: ["%s"]
- type: by-label
  name: decode-pods
  parameters:
    label: "fusioninfer.io/component-type"
    validValues: ["%s"]
- type: prefix-cache-scorer
  parameters:
    hashBlockSize: 5
    maxPrefixBlocksToMatch: 256
    lruCapacityPerServer: 31250
- type: max-score-picker
schedulingProfiles:
- name: prefill
  plugins:
  - pluginRef: prefill-pods
  - pluginRef: max-score-picker
  - pluginRef: prefix-cache-scorer
    weight: 50
- name: decode
  plugins:
  - pluginRef: decode-pods
  - pluginRef: max-score-picker
  - pluginRef: prefix-cache-scorer
    weight: 50
`, prefillerLabel, decoderLabel)
}
