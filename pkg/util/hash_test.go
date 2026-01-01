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

package util

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"
)

func TestComputeSpecHash_LWSSpec(t *testing.T) {
	tests := []struct {
		name     string
		spec1    lwsv1.LeaderWorkerSetSpec
		spec2    lwsv1.LeaderWorkerSetSpec
		wantSame bool
	}{
		{
			name: "same LWS spec produces same hash",
			spec1: lwsv1.LeaderWorkerSetSpec{
				Replicas: ptr.To(int32(2)),
				LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
					Size: ptr.To(int32(4)),
					WorkerTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "vllm",
									Image: "vllm/vllm-openai:v0.13.0",
									Args:  []string{"--model", "Qwen/Qwen3-8B"},
								},
							},
						},
					},
				},
			},
			spec2: lwsv1.LeaderWorkerSetSpec{
				Replicas: ptr.To(int32(2)),
				LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
					Size: ptr.To(int32(4)),
					WorkerTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "vllm",
									Image: "vllm/vllm-openai:v0.13.0",
									Args:  []string{"--model", "Qwen/Qwen3-8B"},
								},
							},
						},
					},
				},
			},
			wantSame: true,
		},
		{
			name: "different replicas produces different hash",
			spec1: lwsv1.LeaderWorkerSetSpec{
				Replicas: ptr.To(int32(2)),
			},
			spec2: lwsv1.LeaderWorkerSetSpec{
				Replicas: ptr.To(int32(3)),
			},
			wantSame: false,
		},
		{
			name: "different size produces different hash",
			spec1: lwsv1.LeaderWorkerSetSpec{
				LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
					Size: ptr.To(int32(2)),
				},
			},
			spec2: lwsv1.LeaderWorkerSetSpec{
				LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
					Size: ptr.To(int32(4)),
				},
			},
			wantSame: false,
		},
		{
			name: "different container image produces different hash",
			spec1: lwsv1.LeaderWorkerSetSpec{
				LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
					WorkerTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Image: "vllm/vllm-openai:v0.12.0"},
							},
						},
					},
				},
			},
			spec2: lwsv1.LeaderWorkerSetSpec{
				LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
					WorkerTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Image: "vllm/vllm-openai:v0.13.0"},
							},
						},
					},
				},
			},
			wantSame: false,
		},
		{
			name: "different container args produces different hash",
			spec1: lwsv1.LeaderWorkerSetSpec{
				LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
					WorkerTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Args: []string{"--model", "Qwen/Qwen3-8B"}},
							},
						},
					},
				},
			},
			spec2: lwsv1.LeaderWorkerSetSpec{
				LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
					WorkerTemplate: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Args: []string{"--model", "Qwen/Qwen3-8B", "--tensor-parallel-size", "2"}},
							},
						},
					},
				},
			},
			wantSame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := ComputeSpecHash(tt.spec1)
			hash2 := ComputeSpecHash(tt.spec2)

			if tt.wantSame && hash1 != hash2 {
				t.Errorf("expected same hash, got %s and %s", hash1, hash2)
			}
			if !tt.wantSame && hash1 == hash2 {
				t.Errorf("expected different hash, got same: %s", hash1)
			}
		})
	}
}

func TestComputeSpecHash_DeploymentSpec(t *testing.T) {
	tests := []struct {
		name     string
		spec1    appsv1.DeploymentSpec
		spec2    appsv1.DeploymentSpec
		wantSame bool
	}{
		{
			name: "same deployment spec produces same hash",
			spec1: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "epp",
								Image: "registry.k8s.io/gateway-api-inference-extension/epp:v1.2.1",
								Ports: []corev1.ContainerPort{
									{ContainerPort: 9002},
								},
							},
						},
					},
				},
			},
			spec2: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "epp",
								Image: "registry.k8s.io/gateway-api-inference-extension/epp:v1.2.1",
								Ports: []corev1.ContainerPort{
									{ContainerPort: 9002},
								},
							},
						},
					},
				},
			},
			wantSame: true,
		},
		{
			name: "different image produces different hash",
			spec1: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Image: "epp:v1.2.0"},
						},
					},
				},
			},
			spec2: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Image: "epp:v1.2.1"},
						},
					},
				},
			},
			wantSame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := ComputeSpecHash(tt.spec1)
			hash2 := ComputeSpecHash(tt.spec2)

			if tt.wantSame && hash1 != hash2 {
				t.Errorf("expected same hash, got %s and %s", hash1, hash2)
			}
			if !tt.wantSame && hash1 == hash2 {
				t.Errorf("expected different hash, got same: %s", hash1)
			}
		})
	}
}

func TestComputeSpecHash_ServiceSpec(t *testing.T) {
	tests := []struct {
		name     string
		spec1    corev1.ServiceSpec
		spec2    corev1.ServiceSpec
		wantSame bool
	}{
		{
			name: "same service spec produces same hash",
			spec1: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:       "grpc",
						Port:       9002,
						TargetPort: intstr.FromInt(9002),
					},
				},
				Selector: map[string]string{"app": "epp"},
			},
			spec2: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:       "grpc",
						Port:       9002,
						TargetPort: intstr.FromInt(9002),
					},
				},
				Selector: map[string]string{"app": "epp"},
			},
			wantSame: true,
		},
		{
			name: "different port produces different hash",
			spec1: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{Port: 8000},
				},
			},
			spec2: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{Port: 9002},
				},
			},
			wantSame: false,
		},
		{
			name: "different selector produces different hash",
			spec1: corev1.ServiceSpec{
				Selector: map[string]string{"app": "epp-v1"},
			},
			spec2: corev1.ServiceSpec{
				Selector: map[string]string{"app": "epp-v2"},
			},
			wantSame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := ComputeSpecHash(tt.spec1)
			hash2 := ComputeSpecHash(tt.spec2)

			if tt.wantSame && hash1 != hash2 {
				t.Errorf("expected same hash, got %s and %s", hash1, hash2)
			}
			if !tt.wantSame && hash1 == hash2 {
				t.Errorf("expected different hash, got same: %s", hash1)
			}
		})
	}
}

func TestComputeSpecHash_ConfigMapData(t *testing.T) {
	tests := []struct {
		name     string
		data1    map[string]string
		data2    map[string]string
		wantSame bool
	}{
		{
			name:     "same config data produces same hash",
			data1:    map[string]string{"config.yaml": "strategy: prefix-cache"},
			data2:    map[string]string{"config.yaml": "strategy: prefix-cache"},
			wantSame: true,
		},
		{
			name:     "different config content produces different hash",
			data1:    map[string]string{"config.yaml": "strategy: prefix-cache"},
			data2:    map[string]string{"config.yaml": "strategy: kv-cache-util"},
			wantSame: false,
		},
		{
			name:     "different config key produces different hash",
			data1:    map[string]string{"config.yaml": "test"},
			data2:    map[string]string{"config.yml": "test"},
			wantSame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := ComputeSpecHash(tt.data1)
			hash2 := ComputeSpecHash(tt.data2)

			if tt.wantSame && hash1 != hash2 {
				t.Errorf("expected same hash, got %s and %s", hash1, hash2)
			}
			if !tt.wantSame && hash1 == hash2 {
				t.Errorf("expected different hash, got same: %s", hash1)
			}
		})
	}
}

func TestComputeSpecHash_Deterministic(t *testing.T) {
	spec := lwsv1.LeaderWorkerSetSpec{
		Replicas: ptr.To(int32(2)),
		LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
			Size: ptr.To(int32(4)),
			WorkerTemplate: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "vllm",
							Image: "vllm/vllm-openai:v0.13.0",
							Args:  []string{"--model", "Qwen/Qwen3-8B", "--tensor-parallel-size", "8"},
						},
					},
				},
			},
		},
	}

	// Compute hash multiple times
	hash1 := ComputeSpecHash(spec)
	hash2 := ComputeSpecHash(spec)
	hash3 := ComputeSpecHash(spec)

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("hash is not deterministic: %s, %s, %s", hash1, hash2, hash3)
	}
}

func TestComputeSpecHash_NotEmpty(t *testing.T) {
	testCases := []struct {
		name string
		obj  interface{}
	}{
		{"LWS spec", lwsv1.LeaderWorkerSetSpec{}},
		{"Deployment spec", appsv1.DeploymentSpec{}},
		{"Service spec", corev1.ServiceSpec{}},
		{"ConfigMap data", map[string]string{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash := ComputeSpecHash(tc.obj)
			if hash == "" {
				t.Errorf("hash should not be empty for %s", tc.name)
			}
		})
	}
}

func TestComputeSpecHash_SafeEncoding(t *testing.T) {
	spec := lwsv1.LeaderWorkerSetSpec{
		Replicas: ptr.To(int32(1)),
		LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
			Size: ptr.To(int32(2)),
		},
	}

	hash := ComputeSpecHash(spec)

	// SafeEncodeString should produce alphanumeric + '-' + '_' characters
	for _, c := range hash {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			t.Errorf("hash contains non-safe character: %c in %s", c, hash)
		}
	}
}
