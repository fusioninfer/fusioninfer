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

package workload

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/v1alpha1"
)

// Helper to convert PodTemplateSpec to RawExtension for tests
func toRawExtension(template *corev1.PodTemplateSpec) *runtime.RawExtension {
	if template == nil {
		return nil
	}
	raw, _ := json.Marshal(template)
	return &runtime.RawExtension{Raw: raw}
}

func TestBuildLWS(t *testing.T) {
	t.Run("basic single node LWS", func(t *testing.T) {
		inferSvc := &fusioninferiov1alpha1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-service",
				Namespace: "default",
			},
		}

		role := fusioninferiov1alpha1.Role{
			Name:          "worker",
			ComponentType: fusioninferiov1alpha1.ComponentTypeWorker,
			Replicas:      ptr.To(int32(2)),
			Template: toRawExtension(&corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "vllm",
							Image: "vllm/vllm:latest",
						},
					},
				},
			}),
		}

		config := LWSConfig{
			NeedsGangScheduling: false,
		}

		lws := BuildLWS(inferSvc, role, config)

		// Verify name
		if lws.Name != "test-service-worker" {
			t.Errorf("expected name test-service-worker, got %s", lws.Name)
		}

		// Verify namespace
		if lws.Namespace != "default" {
			t.Errorf("expected namespace default, got %s", lws.Namespace)
		}

		// Verify replicas
		if *lws.Spec.Replicas != 2 {
			t.Errorf("expected replicas 2, got %d", *lws.Spec.Replicas)
		}

		// Verify size (nodes per replica)
		if *lws.Spec.LeaderWorkerTemplate.Size != 1 {
			t.Errorf("expected size 1, got %d", *lws.Spec.LeaderWorkerTemplate.Size)
		}

		// Verify labels
		if lws.Labels[LabelService] != "test-service" {
			t.Errorf("expected label %s=test-service, got %s", LabelService, lws.Labels[LabelService])
		}
		if lws.Labels[LabelComponentType] != string(fusioninferiov1alpha1.ComponentTypeWorker) {
			t.Errorf("expected label %s=worker, got %s", LabelComponentType, lws.Labels[LabelComponentType])
		}
	})

	t.Run("multi-node LWS", func(t *testing.T) {
		inferSvc := &fusioninferiov1alpha1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multi-node-service",
				Namespace: "default",
			},
		}

		role := fusioninferiov1alpha1.Role{
			Name:          "worker",
			ComponentType: fusioninferiov1alpha1.ComponentTypeWorker,
			Replicas:      ptr.To(int32(1)),
			Multinode:     &fusioninferiov1alpha1.Multinode{NodeCount: 4},
			Template: toRawExtension(&corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "vllm",
							Image: "vllm/vllm:latest",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"nvidia.com/gpu": resource.MustParse("8"),
								},
							},
						},
					},
				},
			}),
		}

		config := LWSConfig{
			NeedsGangScheduling: true,
			PodGroupName:        "multi-node-service",
			TaskName:            "worker-0",
		}

		lws := BuildLWS(inferSvc, role, config)

		// Verify size (4 nodes)
		if *lws.Spec.LeaderWorkerTemplate.Size != 4 {
			t.Errorf("expected size 4, got %d", *lws.Spec.LeaderWorkerTemplate.Size)
		}

		// Verify gang scheduling annotations
		podAnnotations := lws.Spec.LeaderWorkerTemplate.WorkerTemplate.ObjectMeta.Annotations
		if podAnnotations[AnnotationPodGroupName] != "multi-node-service" {
			t.Errorf("expected PodGroup annotation, got %s", podAnnotations[AnnotationPodGroupName])
		}
		if podAnnotations[AnnotationTaskSpec] != "worker-0" {
			t.Errorf("expected task-spec annotation worker-0, got %s", podAnnotations[AnnotationTaskSpec])
		}

		// Verify Volcano scheduler
		if lws.Spec.LeaderWorkerTemplate.WorkerTemplate.Spec.SchedulerName != VolcanoSchedulerName {
			t.Errorf("expected scheduler %s, got %s", VolcanoSchedulerName, lws.Spec.LeaderWorkerTemplate.WorkerTemplate.Spec.SchedulerName)
		}

		// Verify ray command wrapping
		container := lws.Spec.LeaderWorkerTemplate.WorkerTemplate.Spec.Containers[0]
		if len(container.Command) != 1 || container.Command[0] != "ray" {
			t.Errorf("expected command [ray], got %v", container.Command)
		}
		if container.Args[0] != "symmetric-run" {
			t.Errorf("expected first arg symmetric-run, got %s", container.Args[0])
		}
	})

	t.Run("per-replica LWS", func(t *testing.T) {
		inferSvc := &fusioninferiov1alpha1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "per-replica-service",
				Namespace: "default",
			},
		}

		role := fusioninferiov1alpha1.Role{
			Name:          "worker",
			ComponentType: fusioninferiov1alpha1.ComponentTypeWorker,
			Replicas:      ptr.To(int32(3)),
			Template: toRawExtension(&corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "vllm",
							Image: "vllm/vllm:latest",
						},
					},
				},
			}),
		}

		replicaIndex := int32(1)
		config := LWSConfig{
			ReplicaIndex:        &replicaIndex,
			NeedsGangScheduling: true,
			PodGroupName:        "per-replica-service",
			TaskName:            "worker-1",
		}

		lws := BuildLWS(inferSvc, role, config)

		// Verify name includes replica index
		if lws.Name != "per-replica-service-worker-1" {
			t.Errorf("expected name per-replica-service-worker-1, got %s", lws.Name)
		}

		// Verify replicas is always 1 in per-replica mode
		if *lws.Spec.Replicas != 1 {
			t.Errorf("expected replicas 1 in per-replica mode, got %d", *lws.Spec.Replicas)
		}

		// Verify replica index label
		if lws.Labels[LabelReplicaIndex] != "1" {
			t.Errorf("expected replica index label 1, got %s", lws.Labels[LabelReplicaIndex])
		}
	})
}

func TestGenerateLWSNameWithIndex(t *testing.T) {
	tests := []struct {
		name         string
		inferSvcName string
		roleName     string
		replicaIndex *int32
		want         string
	}{
		{
			name:         "without replica index",
			inferSvcName: "my-service",
			roleName:     "worker",
			replicaIndex: nil,
			want:         "my-service-worker",
		},
		{
			name:         "with replica index 0",
			inferSvcName: "my-service",
			roleName:     "prefill",
			replicaIndex: ptr.To(int32(0)),
			want:         "my-service-prefill-0",
		},
		{
			name:         "with replica index 2",
			inferSvcName: "pd-service",
			roleName:     "decode",
			replicaIndex: ptr.To(int32(2)),
			want:         "pd-service-decode-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateLWSNameWithIndex(tt.inferSvcName, tt.roleName, tt.replicaIndex)
			if got != tt.want {
				t.Errorf("GenerateLWSNameWithIndex() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestIsMultiNode(t *testing.T) {
	tests := []struct {
		name string
		role fusioninferiov1alpha1.Role
		want bool
	}{
		{
			name: "nil multinode",
			role: fusioninferiov1alpha1.Role{},
			want: false,
		},
		{
			name: "node count 1 is not multi-node",
			role: fusioninferiov1alpha1.Role{
				Multinode: &fusioninferiov1alpha1.Multinode{NodeCount: 1},
			},
			want: false,
		},
		{
			name: "node count 2 is multi-node",
			role: fusioninferiov1alpha1.Role{
				Multinode: &fusioninferiov1alpha1.Multinode{NodeCount: 2},
			},
			want: true,
		},
		{
			name: "node count 8 is multi-node",
			role: fusioninferiov1alpha1.Role{
				Multinode: &fusioninferiov1alpha1.Multinode{NodeCount: 8},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMultiNode(tt.role)
			if got != tt.want {
				t.Errorf("IsMultiNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWrapContainerWithRay(t *testing.T) {
	role := fusioninferiov1alpha1.Role{
		Multinode: &fusioninferiov1alpha1.Multinode{NodeCount: 4},
	}

	container := &corev1.Container{
		Name:    "vllm",
		Image:   "vllm/vllm:latest",
		Command: []string{"python", "-m", "vllm.entrypoints.openai.api_server"},
		Args:    []string{"--model", "Qwen/Qwen3-8B"},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"nvidia.com/gpu": resource.MustParse("8"),
			},
		},
	}

	wrapContainerWithRay(container, role)

	// Verify command is now ray
	if len(container.Command) != 1 || container.Command[0] != "ray" {
		t.Errorf("expected command [ray], got %v", container.Command)
	}

	// Verify args structure
	expectedArgsPrefix := []string{
		"symmetric-run",
		"--address",
	}

	for i, expected := range expectedArgsPrefix {
		if container.Args[i] != expected {
			t.Errorf("expected args[%d]=%s, got %s", i, expected, container.Args[i])
		}
	}

	// Verify --min-nodes is set
	found := false
	for i, arg := range container.Args {
		if arg == "--min-nodes" && i+1 < len(container.Args) {
			if container.Args[i+1] != "4" {
				t.Errorf("expected --min-nodes 4, got %s", container.Args[i+1])
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("expected --min-nodes flag")
	}

	// Verify --num-gpus is set
	found = false
	for i, arg := range container.Args {
		if arg == "--num-gpus" && i+1 < len(container.Args) {
			if container.Args[i+1] != "8" {
				t.Errorf("expected --num-gpus 8, got %s", container.Args[i+1])
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("expected --num-gpus flag")
	}

	// Verify original command and args are preserved after --
	foundSeparator := false
	for i, arg := range container.Args {
		if arg == "--" {
			foundSeparator = true
			// Check original command/args follow
			remaining := container.Args[i+1:]
			if len(remaining) < 4 {
				t.Errorf("expected original command/args after --, got %v", remaining)
			}
			break
		}
	}
	if !foundSeparator {
		t.Error("expected -- separator in args")
	}

	// Verify Ray head port is added
	found = false
	for _, port := range container.Ports {
		if port.ContainerPort == RayHeadPort {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Ray head port to be added")
	}
}

func TestGetGPUCount(t *testing.T) {
	tests := []struct {
		name      string
		container *corev1.Container
		want      int
	}{
		{
			name: "no resources",
			container: &corev1.Container{
				Name: "test",
			},
			want: 0,
		},
		{
			name: "no GPU limit",
			container: &corev1.Container{
				Name: "test",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("4"),
					},
				},
			},
			want: 0,
		},
		{
			name: "8 GPUs",
			container: &corev1.Container{
				Name: "test",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"nvidia.com/gpu": resource.MustParse("8"),
					},
				},
			},
			want: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getGPUCount(tt.container)
			if got != tt.want {
				t.Errorf("getGPUCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEnsureRayHeadPort(t *testing.T) {
	t.Run("adds port when missing", func(t *testing.T) {
		container := &corev1.Container{
			Name: "test",
			Ports: []corev1.ContainerPort{
				{Name: "http", ContainerPort: 8000},
			},
		}

		ensureRayHeadPort(container)

		if len(container.Ports) != 2 {
			t.Errorf("expected 2 ports, got %d", len(container.Ports))
		}

		found := false
		for _, port := range container.Ports {
			if port.ContainerPort == RayHeadPort {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected Ray head port to be added")
		}
	})

	t.Run("does not duplicate existing port", func(t *testing.T) {
		container := &corev1.Container{
			Name: "test",
			Ports: []corev1.ContainerPort{
				{Name: "ray-head", ContainerPort: RayHeadPort},
			},
		}

		ensureRayHeadPort(container)

		if len(container.Ports) != 1 {
			t.Errorf("expected 1 port (no duplicate), got %d", len(container.Ports))
		}
	})
}
