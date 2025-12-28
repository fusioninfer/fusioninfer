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

package scheduling

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/core/v1alpha1"
)

// Helper to convert PodTemplateSpec to RawExtension for tests
func toRawExtension(template *corev1.PodTemplateSpec) *runtime.RawExtension {
	if template == nil {
		return nil
	}
	raw, _ := json.Marshal(template)
	return &runtime.RawExtension{Raw: raw}
}

func TestIsPDDisaggregated(t *testing.T) {
	tests := []struct {
		name     string
		inferSvc *fusioninferiov1alpha1.InferenceService
		want     bool
	}{
		{
			name: "has both prefiller and decoder",
			inferSvc: &fusioninferiov1alpha1.InferenceService{
				Spec: fusioninferiov1alpha1.InferenceServiceSpec{
					Roles: []fusioninferiov1alpha1.Role{
						{Name: "prefill", ComponentType: fusioninferiov1alpha1.ComponentTypePrefiller},
						{Name: "decode", ComponentType: fusioninferiov1alpha1.ComponentTypeDecoder},
					},
				},
			},
			want: true,
		},
		{
			name: "only prefiller",
			inferSvc: &fusioninferiov1alpha1.InferenceService{
				Spec: fusioninferiov1alpha1.InferenceServiceSpec{
					Roles: []fusioninferiov1alpha1.Role{
						{Name: "prefill", ComponentType: fusioninferiov1alpha1.ComponentTypePrefiller},
					},
				},
			},
			want: false,
		},
		{
			name: "only worker",
			inferSvc: &fusioninferiov1alpha1.InferenceService{
				Spec: fusioninferiov1alpha1.InferenceServiceSpec{
					Roles: []fusioninferiov1alpha1.Role{
						{Name: "worker", ComponentType: fusioninferiov1alpha1.ComponentTypeWorker},
					},
				},
			},
			want: false,
		},
		{
			name: "worker with prefiller and decoder",
			inferSvc: &fusioninferiov1alpha1.InferenceService{
				Spec: fusioninferiov1alpha1.InferenceServiceSpec{
					Roles: []fusioninferiov1alpha1.Role{
						{Name: "prefill", ComponentType: fusioninferiov1alpha1.ComponentTypePrefiller},
						{Name: "decode", ComponentType: fusioninferiov1alpha1.ComponentTypeDecoder},
						{Name: "router", ComponentType: fusioninferiov1alpha1.ComponentTypeRouter},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPDDisaggregated(tt.inferSvc)
			if got != tt.want {
				t.Errorf("IsPDDisaggregated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNeedsGangScheduling(t *testing.T) {
	tests := []struct {
		name     string
		inferSvc *fusioninferiov1alpha1.InferenceService
		want     bool
	}{
		{
			name: "PD disaggregated needs gang scheduling",
			inferSvc: &fusioninferiov1alpha1.InferenceService{
				Spec: fusioninferiov1alpha1.InferenceServiceSpec{
					Roles: []fusioninferiov1alpha1.Role{
						{Name: "prefill", ComponentType: fusioninferiov1alpha1.ComponentTypePrefiller},
						{Name: "decode", ComponentType: fusioninferiov1alpha1.ComponentTypeDecoder},
					},
				},
			},
			want: true,
		},
		{
			name: "multi-node needs gang scheduling",
			inferSvc: &fusioninferiov1alpha1.InferenceService{
				Spec: fusioninferiov1alpha1.InferenceServiceSpec{
					Roles: []fusioninferiov1alpha1.Role{
						{
							Name:          "worker",
							ComponentType: fusioninferiov1alpha1.ComponentTypeWorker,
							Multinode:     &fusioninferiov1alpha1.Multinode{NodeCount: 2},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "single node worker does not need gang scheduling",
			inferSvc: &fusioninferiov1alpha1.InferenceService{
				Spec: fusioninferiov1alpha1.InferenceServiceSpec{
					Roles: []fusioninferiov1alpha1.Role{
						{Name: "worker", ComponentType: fusioninferiov1alpha1.ComponentTypeWorker},
					},
				},
			},
			want: false,
		},
		{
			name: "router role is ignored",
			inferSvc: &fusioninferiov1alpha1.InferenceService{
				Spec: fusioninferiov1alpha1.InferenceServiceSpec{
					Roles: []fusioninferiov1alpha1.Role{
						{Name: "worker", ComponentType: fusioninferiov1alpha1.ComponentTypeWorker},
						{Name: "router", ComponentType: fusioninferiov1alpha1.ComponentTypeRouter},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsGangScheduling(tt.inferSvc)
			if got != tt.want {
				t.Errorf("NeedsGangScheduling() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNeedsGangSchedulingForRole(t *testing.T) {
	pdInferSvc := &fusioninferiov1alpha1.InferenceService{
		Spec: fusioninferiov1alpha1.InferenceServiceSpec{
			Roles: []fusioninferiov1alpha1.Role{
				{Name: "prefill", ComponentType: fusioninferiov1alpha1.ComponentTypePrefiller},
				{Name: "decode", ComponentType: fusioninferiov1alpha1.ComponentTypeDecoder},
			},
		},
	}

	monolithicInferSvc := &fusioninferiov1alpha1.InferenceService{
		Spec: fusioninferiov1alpha1.InferenceServiceSpec{
			Roles: []fusioninferiov1alpha1.Role{
				{Name: "worker", ComponentType: fusioninferiov1alpha1.ComponentTypeWorker},
			},
		},
	}

	tests := []struct {
		name     string
		inferSvc *fusioninferiov1alpha1.InferenceService
		role     fusioninferiov1alpha1.Role
		want     bool
	}{
		{
			name:     "prefiller in PD setup",
			inferSvc: pdInferSvc,
			role:     fusioninferiov1alpha1.Role{Name: "prefill", ComponentType: fusioninferiov1alpha1.ComponentTypePrefiller},
			want:     true,
		},
		{
			name:     "decoder in PD setup",
			inferSvc: pdInferSvc,
			role:     fusioninferiov1alpha1.Role{Name: "decode", ComponentType: fusioninferiov1alpha1.ComponentTypeDecoder},
			want:     true,
		},
		{
			name:     "worker in monolithic setup - no multi-node",
			inferSvc: monolithicInferSvc,
			role:     fusioninferiov1alpha1.Role{Name: "worker", ComponentType: fusioninferiov1alpha1.ComponentTypeWorker},
			want:     false,
		},
		{
			name:     "worker with multi-node",
			inferSvc: monolithicInferSvc,
			role: fusioninferiov1alpha1.Role{
				Name:          "worker",
				ComponentType: fusioninferiov1alpha1.ComponentTypeWorker,
				Multinode:     &fusioninferiov1alpha1.Multinode{NodeCount: 4},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsGangSchedulingForRole(tt.inferSvc, tt.role)
			if got != tt.want {
				t.Errorf("NeedsGangSchedulingForRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildPodGroup(t *testing.T) {
	t.Run("PD disaggregated", func(t *testing.T) {
		inferSvc := &fusioninferiov1alpha1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pd-service",
				Namespace: "default",
			},
			Spec: fusioninferiov1alpha1.InferenceServiceSpec{
				Roles: []fusioninferiov1alpha1.Role{
					{
						Name:          "prefill",
						ComponentType: fusioninferiov1alpha1.ComponentTypePrefiller,
						Replicas:      ptr.To(int32(1)),
						Template: toRawExtension(&corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Resources: corev1.ResourceRequirements{
											Limits: corev1.ResourceList{
												"nvidia.com/gpu": resource.MustParse("2"),
											},
										},
									},
								},
							},
						}),
					},
					{
						Name:          "decode",
						ComponentType: fusioninferiov1alpha1.ComponentTypeDecoder,
						Replicas:      ptr.To(int32(2)),
						Template: toRawExtension(&corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Resources: corev1.ResourceRequirements{
											Limits: corev1.ResourceList{
												"nvidia.com/gpu": resource.MustParse("1"),
											},
										},
									},
								},
							},
						}),
					},
				},
			},
		}

		pg := BuildPodGroup(inferSvc)

		// Verify name
		if pg.Name != "pd-service" {
			t.Errorf("expected name pd-service, got %s", pg.Name)
		}

		// Verify namespace
		if pg.Namespace != "default" {
			t.Errorf("expected namespace default, got %s", pg.Namespace)
		}

		// Verify minMember: 1 (prefill) + 2 (decode) = 3
		if pg.Spec.MinMember != 3 {
			t.Errorf("expected minMember 3, got %d", pg.Spec.MinMember)
		}

		// Verify minTaskMember
		if pg.Spec.MinTaskMember["prefill-0"] != 1 {
			t.Errorf("expected minTaskMember[prefill-0]=1, got %d", pg.Spec.MinTaskMember["prefill-0"])
		}
		if pg.Spec.MinTaskMember["decode-0"] != 1 {
			t.Errorf("expected minTaskMember[decode-0]=1, got %d", pg.Spec.MinTaskMember["decode-0"])
		}
		if pg.Spec.MinTaskMember["decode-1"] != 1 {
			t.Errorf("expected minTaskMember[decode-1]=1, got %d", pg.Spec.MinTaskMember["decode-1"])
		}
	})

	t.Run("multi-node", func(t *testing.T) {
		inferSvc := &fusioninferiov1alpha1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multi-node-service",
				Namespace: "default",
			},
			Spec: fusioninferiov1alpha1.InferenceServiceSpec{
				Roles: []fusioninferiov1alpha1.Role{
					{
						Name:          "worker",
						ComponentType: fusioninferiov1alpha1.ComponentTypeWorker,
						Replicas:      ptr.To(int32(2)),
						Multinode:     &fusioninferiov1alpha1.Multinode{NodeCount: 4},
					},
				},
			},
		}

		pg := BuildPodGroup(inferSvc)

		// Verify minMember: 2 replicas * 4 nodes = 8
		if pg.Spec.MinMember != 8 {
			t.Errorf("expected minMember 8, got %d", pg.Spec.MinMember)
		}

		// Verify minTaskMember: each replica needs 4 nodes
		if pg.Spec.MinTaskMember["worker-0"] != 4 {
			t.Errorf("expected minTaskMember[worker-0]=4, got %d", pg.Spec.MinTaskMember["worker-0"])
		}
		if pg.Spec.MinTaskMember["worker-1"] != 4 {
			t.Errorf("expected minTaskMember[worker-1]=4, got %d", pg.Spec.MinTaskMember["worker-1"])
		}
	})

	t.Run("PD disaggregated with multi-node", func(t *testing.T) {
		inferSvc := &fusioninferiov1alpha1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pd-multi-node",
				Namespace: "default",
			},
			Spec: fusioninferiov1alpha1.InferenceServiceSpec{
				Roles: []fusioninferiov1alpha1.Role{
					{
						Name:          "prefill",
						ComponentType: fusioninferiov1alpha1.ComponentTypePrefiller,
						Replicas:      ptr.To(int32(1)),
						Multinode:     &fusioninferiov1alpha1.Multinode{NodeCount: 2},
					},
					{
						Name:          "decode",
						ComponentType: fusioninferiov1alpha1.ComponentTypeDecoder,
						Replicas:      ptr.To(int32(2)),
						Multinode:     &fusioninferiov1alpha1.Multinode{NodeCount: 4},
					},
				},
			},
		}

		pg := BuildPodGroup(inferSvc)

		// Verify minMember: (1*2) + (2*4) = 2 + 8 = 10
		if pg.Spec.MinMember != 10 {
			t.Errorf("expected minMember 10, got %d", pg.Spec.MinMember)
		}

		// Verify minTaskMember
		if pg.Spec.MinTaskMember["prefill-0"] != 2 {
			t.Errorf("expected minTaskMember[prefill-0]=2, got %d", pg.Spec.MinTaskMember["prefill-0"])
		}
		if pg.Spec.MinTaskMember["decode-0"] != 4 {
			t.Errorf("expected minTaskMember[decode-0]=4, got %d", pg.Spec.MinTaskMember["decode-0"])
		}
		if pg.Spec.MinTaskMember["decode-1"] != 4 {
			t.Errorf("expected minTaskMember[decode-1]=4, got %d", pg.Spec.MinTaskMember["decode-1"])
		}
	})

	t.Run("router role is skipped", func(t *testing.T) {
		inferSvc := &fusioninferiov1alpha1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "with-router",
				Namespace: "default",
			},
			Spec: fusioninferiov1alpha1.InferenceServiceSpec{
				Roles: []fusioninferiov1alpha1.Role{
					{
						Name:          "prefill",
						ComponentType: fusioninferiov1alpha1.ComponentTypePrefiller,
						Replicas:      ptr.To(int32(1)),
					},
					{
						Name:          "decode",
						ComponentType: fusioninferiov1alpha1.ComponentTypeDecoder,
						Replicas:      ptr.To(int32(1)),
					},
					{
						Name:          "router",
						ComponentType: fusioninferiov1alpha1.ComponentTypeRouter,
						Replicas:      ptr.To(int32(2)),
					},
				},
			},
		}

		pg := BuildPodGroup(inferSvc)

		// Router should not be counted
		if pg.Spec.MinMember != 2 {
			t.Errorf("expected minMember 2 (router excluded), got %d", pg.Spec.MinMember)
		}

		if _, exists := pg.Spec.MinTaskMember["router-0"]; exists {
			t.Error("router should not be in minTaskMember")
		}
	})
}

func TestGeneratePodGroupName(t *testing.T) {
	if got := GeneratePodGroupName("my-service"); got != "my-service" {
		t.Errorf("GeneratePodGroupName() = %s, want my-service", got)
	}
}

func TestGenerateTaskName(t *testing.T) {
	tests := []struct {
		roleName     string
		replicaIndex int32
		want         string
	}{
		{"prefill", 0, "prefill-0"},
		{"decode", 1, "decode-1"},
		{"worker", 5, "worker-5"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := GenerateTaskName(tt.roleName, tt.replicaIndex)
			if got != tt.want {
				t.Errorf("GenerateTaskName() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestGetNodeCount(t *testing.T) {
	tests := []struct {
		name string
		role fusioninferiov1alpha1.Role
		want int32
	}{
		{
			name: "nil multinode",
			role: fusioninferiov1alpha1.Role{},
			want: 1,
		},
		{
			name: "node count 4",
			role: fusioninferiov1alpha1.Role{
				Multinode: &fusioninferiov1alpha1.Multinode{NodeCount: 4},
			},
			want: 4,
		},
		{
			name: "node count 0 defaults to 1",
			role: fusioninferiov1alpha1.Role{
				Multinode: &fusioninferiov1alpha1.Multinode{NodeCount: 0},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetNodeCount(tt.role)
			if got != tt.want {
				t.Errorf("GetNodeCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetReplicaCount(t *testing.T) {
	tests := []struct {
		name string
		role fusioninferiov1alpha1.Role
		want int32
	}{
		{
			name: "nil replicas",
			role: fusioninferiov1alpha1.Role{},
			want: 1,
		},
		{
			name: "replicas 3",
			role: fusioninferiov1alpha1.Role{
				Replicas: ptr.To(int32(3)),
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetReplicaCount(tt.role)
			if got != tt.want {
				t.Errorf("GetReplicaCount() = %d, want %d", got, tt.want)
			}
		})
	}
}
