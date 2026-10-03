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

package cel

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/core/v1alpha1"
)

const (
	vllmImage   = "vllm/vllm-openai:v0.27.1"
	sglangImage = "lmsysorg/sglang:v0.5.4"

	// Messages of the RuntimeProfile validation rules.
	msgNoRole          = "set aggregated, or prefiller and decoder"
	msgAggregatedAndPD = "aggregated cannot be combined with prefiller or decoder"
	msgPDTogether      = "prefiller and decoder must be set together"
	msgImmutableSpec   = "spec is immutable"
	msgPickerAggregate = "endpointPicker can be set only with aggregated"
	msgKVTransferPD    = "kvTransfer is required with prefiller and decoder"
	msgKVTransferOnly  = "kvTransfer can be set only with prefiller and decoder"
)

// runtimeProfileObject returns a RuntimeProfile in the default namespace or a cluster-scoped
// ClusterRuntimeProfile.
func runtimeProfileObject(kind, name string, spec fusioninferiov1alpha1.RuntimeProfileSpec) client.Object {
	switch kind {
	case "RuntimeProfile":
		return &fusioninferiov1alpha1.RuntimeProfile{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       spec,
		}
	case "ClusterRuntimeProfile":
		return &fusioninferiov1alpha1.ClusterRuntimeProfile{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       spec,
		}
	default:
		panic(fmt.Sprintf("unknown kind %q", kind))
	}
}

// profileSpecOf returns the spec of a RuntimeProfile or ClusterRuntimeProfile, which share
// RuntimeProfileSpec.
func profileSpecOf(object client.Object) *fusioninferiov1alpha1.RuntimeProfileSpec {
	switch object := object.(type) {
	case *fusioninferiov1alpha1.RuntimeProfile:
		return &object.Spec
	case *fusioninferiov1alpha1.ClusterRuntimeProfile:
		return &object.Spec
	default:
		panic(fmt.Sprintf("%T is not a RuntimeProfile or ClusterRuntimeProfile", object))
	}
}

// engineRole returns a role whose Pod template runs image in an engine container with an http port.
func engineRole(image string) *fusioninferiov1alpha1.RuntimeComponentSpec {
	return &fusioninferiov1alpha1.RuntimeComponentSpec{
		PodTemplate: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "engine",
					Image: image,
					Args:  []string{"$(FUSIONINFER_MODEL_PATH)"},
					Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8000}},
				}},
			},
		},
	}
}

// multinodeRole returns an engine role whose logical replica spans nodeCount nodes.
func multinodeRole(image string, nodeCount int32) *fusioninferiov1alpha1.RuntimeComponentSpec {
	role := engineRole(image)
	role.Multinode = &fusioninferiov1alpha1.MultinodeSpec{NodeCount: nodeCount}
	return role
}

// aggregatedSpec returns a vLLM runtime with a single-node aggregated role.
func aggregatedSpec() fusioninferiov1alpha1.RuntimeProfileSpec {
	return fusioninferiov1alpha1.RuntimeProfileSpec{
		Backend:    fusioninferiov1alpha1.RuntimeBackendVLLM,
		Aggregated: engineRole(vllmImage),
	}
}

// disaggregatedSpec returns a vLLM runtime with single-node prefiller and decoder roles that transfer
// the KV cache with NIXL.
func disaggregatedSpec() fusioninferiov1alpha1.RuntimeProfileSpec {
	return fusioninferiov1alpha1.RuntimeProfileSpec{
		Backend:    fusioninferiov1alpha1.RuntimeBackendVLLM,
		KVTransfer: &fusioninferiov1alpha1.KVTransferSpec{Connector: fusioninferiov1alpha1.KVConnectorNIXL},
		Prefiller:  engineRole(vllmImage),
		Decoder:    engineRole(vllmImage),
	}
}

// withLoRA returns spec with the given LoRA loading mode.
func withLoRA(
	spec fusioninferiov1alpha1.RuntimeProfileSpec, mode fusioninferiov1alpha1.LoRALoadingMode,
) fusioninferiov1alpha1.RuntimeProfileSpec {
	spec.LoRA = &fusioninferiov1alpha1.RuntimeLoRASpec{LoadingMode: mode}
	return spec
}

// withEndpointPicker returns spec with the given default Endpoint Picker strategy.
func withEndpointPicker(
	spec fusioninferiov1alpha1.RuntimeProfileSpec, strategy fusioninferiov1alpha1.RoutingStrategy,
) fusioninferiov1alpha1.RuntimeProfileSpec {
	spec.EndpointPicker = &fusioninferiov1alpha1.EndpointPickerSpec{Strategy: strategy}
	return spec
}

// withKVConnector returns spec with a KV transfer through the given connector.
func withKVConnector(
	spec fusioninferiov1alpha1.RuntimeProfileSpec, connector fusioninferiov1alpha1.KVConnector,
) fusioninferiov1alpha1.RuntimeProfileSpec {
	spec.KVTransfer = &fusioninferiov1alpha1.KVTransferSpec{Connector: connector}
	return spec
}

// withoutKVTransfer returns spec without a KV transfer.
func withoutKVTransfer(spec fusioninferiov1alpha1.RuntimeProfileSpec) fusioninferiov1alpha1.RuntimeProfileSpec {
	spec.KVTransfer = nil
	return spec
}

// TestRuntimeProfileAcceptsSupportedRuntimes checks the valid combinations of backend, roles,
// multinode and LoRA loading.
func TestRuntimeProfileAcceptsSupportedRuntimes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc string
		name string
		spec fusioninferiov1alpha1.RuntimeProfileSpec
	}{
		{"aggregated", "profile-aggregated", aggregatedSpec()},
		{"prefill/decode", "profile-pd", disaggregatedSpec()},
		{"multinode aggregated", "profile-multinode", fusioninferiov1alpha1.RuntimeProfileSpec{
			Backend:    fusioninferiov1alpha1.RuntimeBackendVLLM,
			Aggregated: multinodeRole(vllmImage, 4),
		}},
		{"multinode prefill/decode", "profile-multinode-pd", fusioninferiov1alpha1.RuntimeProfileSpec{
			Backend:    fusioninferiov1alpha1.RuntimeBackendSGLang,
			KVTransfer: &fusioninferiov1alpha1.KVTransferSpec{Connector: fusioninferiov1alpha1.KVConnectorNIXL},
			Prefiller:  multinodeRole(sglangImage, 2),
			Decoder:    engineRole(sglangImage),
		}},
		{"preload LoRA", "profile-lora-preload",
			withLoRA(aggregatedSpec(), fusioninferiov1alpha1.LoRALoadingModePreload)},
		{"dynamic LoRA", "profile-lora-dynamic",
			withLoRA(disaggregatedSpec(), fusioninferiov1alpha1.LoRALoadingModeDynamic)},
		{"endpoint picker", "profile-endpoint-picker",
			withEndpointPicker(aggregatedSpec(), fusioninferiov1alpha1.StrategyPrefixCache)},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			createObject(t, runtimeProfileObject("RuntimeProfile", tt.name, tt.spec))
		})
	}
}

// TestRuntimeProfileRejectsInvalidRoles checks that a runtime sets either aggregated or both
// prefiller and decoder.
func TestRuntimeProfileRejectsInvalidRoles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc    string
		name    string
		spec    fusioninferiov1alpha1.RuntimeProfileSpec
		message string
	}{
		{"no role", "bad-roles-none",
			fusioninferiov1alpha1.RuntimeProfileSpec{Backend: fusioninferiov1alpha1.RuntimeBackendVLLM}, msgNoRole},
		{"prefiller without decoder", "bad-roles-prefiller", fusioninferiov1alpha1.RuntimeProfileSpec{
			Backend:   fusioninferiov1alpha1.RuntimeBackendVLLM,
			Prefiller: engineRole(vllmImage),
		}, msgPDTogether},
		{"decoder without prefiller", "bad-roles-decoder", fusioninferiov1alpha1.RuntimeProfileSpec{
			Backend: fusioninferiov1alpha1.RuntimeBackendVLLM,
			Decoder: engineRole(vllmImage),
		}, msgPDTogether},
		{"aggregated with prefiller and decoder", "bad-roles-all", fusioninferiov1alpha1.RuntimeProfileSpec{
			Backend:    fusioninferiov1alpha1.RuntimeBackendVLLM,
			Aggregated: engineRole(vllmImage),
			Prefiller:  engineRole(vllmImage),
			Decoder:    engineRole(vllmImage),
		}, msgAggregatedAndPD},
		{"endpoint picker with prefiller and decoder", "bad-roles-endpoint-picker",
			withEndpointPicker(disaggregatedSpec(), fusioninferiov1alpha1.StrategyPrefixCache), msgPickerAggregate},
		{"prefiller and decoder without kvTransfer", "bad-roles-no-kv-transfer",
			withoutKVTransfer(disaggregatedSpec()), msgKVTransferPD},
		{"kvTransfer with aggregated", "bad-roles-kv-transfer",
			withKVConnector(aggregatedSpec(), fusioninferiov1alpha1.KVConnectorNIXL), msgKVTransferOnly},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			object := runtimeProfileObject("RuntimeProfile", tt.name, tt.spec)
			expectInvalid(t, k8sClient.Create(t.Context(), object), tt.message)
		})
	}
}

// TestRuntimeProfileLimitsEndpointPickerStrategies checks that endpointPicker accepts only the
// strategies of an aggregated deployment, although RoutingStrategy also has lora-affinity and
// pd-disaggregation.
func TestRuntimeProfileLimitsEndpointPickerStrategies(t *testing.T) {
	t.Parallel()
	for _, strategy := range []fusioninferiov1alpha1.RoutingStrategy{
		fusioninferiov1alpha1.StrategyLoRAffinity,
		fusioninferiov1alpha1.StrategyPDDisaggregation,
	} {
		t.Run(string(strategy), func(t *testing.T) {
			t.Parallel()
			object := runtimeProfileObject("RuntimeProfile", "bad-picker-"+string(strategy),
				withEndpointPicker(aggregatedSpec(), strategy))
			expectInvalid(t, k8sClient.Create(t.Context(), object), fmt.Sprintf("Unsupported value: %q", strategy))
		})
	}
}

// TestRuntimeProfileLimitsKVConnectors checks that kvTransfer accepts only the connectors that the
// Controller supports.
func TestRuntimeProfileLimitsKVConnectors(t *testing.T) {
	t.Parallel()
	for _, connector := range []fusioninferiov1alpha1.KVConnector{"mooncake", "lmcache"} {
		t.Run(string(connector), func(t *testing.T) {
			t.Parallel()
			object := runtimeProfileObject("RuntimeProfile", "bad-kv-connector-"+string(connector),
				withKVConnector(disaggregatedSpec(), connector))
			expectInvalid(t, k8sClient.Create(t.Context(), object), fmt.Sprintf("Unsupported value: %q", connector))
		})
	}
}

// TestRuntimeProfileKeepsPodTemplateMetadata checks that the labels and annotations of a Pod
// template survive pruning, which needs the CRD to be generated with generateEmbeddedObjectMeta.
func TestRuntimeProfileKeepsPodTemplateMetadata(t *testing.T) {
	t.Parallel()
	metadata := metav1.ObjectMeta{
		Labels:      map[string]string{"example.com/runtime": "vllm"},
		Annotations: map[string]string{"example.com/owner": "team-a"},
	}
	spec := aggregatedSpec()
	spec.Aggregated.PodTemplate.ObjectMeta = metadata
	object := runtimeProfileObject("RuntimeProfile", "template-metadata", spec)
	createObject(t, object)

	stored := &fusioninferiov1alpha1.RuntimeProfile{}
	if err := k8sClient.Get(t.Context(), client.ObjectKeyFromObject(object), stored); err != nil {
		t.Fatalf("get RuntimeProfile: %v", err)
	}
	got := stored.Spec.Aggregated.PodTemplate.ObjectMeta
	if diff := cmp.Diff(metadata.Labels, got.Labels); diff != "" {
		t.Errorf("template labels mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(metadata.Annotations, got.Annotations); diff != "" {
		t.Errorf("template annotations mismatch (-want +got):\n%s", diff)
	}
}

// TestRuntimeProfileSpecIsImmutable checks that metadata can change after creation but spec cannot.
func TestRuntimeProfileSpecIsImmutable(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"RuntimeProfile", "ClusterRuntimeProfile"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			name := "immutable-" + strings.ToLower(kind)
			createObject(t, runtimeProfileObject(kind, name, aggregatedSpec()))

			latest := runtimeProfileObject(kind, name, fusioninferiov1alpha1.RuntimeProfileSpec{})
			if err := updateObject(ctx, latest, func(object client.Object) {
				object.SetLabels(map[string]string{"fusioninfer.io/test": "metadata-update"})
			}); err != nil {
				t.Errorf("update metadata: %v", err)
			}

			rejected := []struct {
				desc   string
				mutate func(client.Object)
			}{
				{"backend", func(object client.Object) {
					profileSpecOf(object).Backend = fusioninferiov1alpha1.RuntimeBackendSGLang
				}},
				{"lora", func(object client.Object) {
					profileSpecOf(object).LoRA = &fusioninferiov1alpha1.RuntimeLoRASpec{
						LoadingMode: fusioninferiov1alpha1.LoRALoadingModeDynamic,
					}
				}},
				{"endpointPicker", func(object client.Object) {
					profileSpecOf(object).EndpointPicker = &fusioninferiov1alpha1.EndpointPickerSpec{
						Strategy: fusioninferiov1alpha1.StrategyQueueSize,
					}
				}},
				{"multinode", func(object client.Object) {
					profileSpecOf(object).Aggregated.Multinode = &fusioninferiov1alpha1.MultinodeSpec{NodeCount: 2}
				}},
				{"podTemplate", func(object client.Object) {
					profileSpecOf(object).Aggregated = engineRole("vllm/vllm-openai:v0.28.0")
				}},
				{"roles", func(object client.Object) {
					*profileSpecOf(object) = disaggregatedSpec()
				}},
			}
			for _, update := range rejected {
				t.Run(update.desc, func(t *testing.T) {
					latest := runtimeProfileObject(kind, name, fusioninferiov1alpha1.RuntimeProfileSpec{})
					expectInvalid(t, updateObject(ctx, latest, update.mutate), msgImmutableSpec)
				})
			}
		})
	}
}
