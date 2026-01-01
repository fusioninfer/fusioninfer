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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/core/v1alpha1"
	"github.com/fusioninfer/fusioninfer/pkg/util"
)

const (
	// Ray distributed inference constants
	RayHeadPort         = 6379
	LWSLeaderAddressEnv = "LWS_LEADER_ADDRESS"
)

const (
	// Labels
	LabelService       = "fusioninfer.io/service"
	LabelComponentType = "fusioninfer.io/component-type"
	LabelRoleName      = "fusioninfer.io/role-name"
	LabelReplicaIndex  = "fusioninfer.io/replica-index"
	// LabelSpecHash stores the hash of the resource spec for change detection
	// When the spec changes, the hash changes, triggering reconciliation
	LabelSpecHash = "fusioninfer.io/spec-hash"

	// Annotations for Volcano gang scheduling
	AnnotationPodGroupName = "scheduling.k8s.io/group-name"
	AnnotationTaskSpec     = "volcano.sh/task-spec"

	// Volcano scheduler name
	VolcanoSchedulerName = "volcano"
)

// LWSConfig holds configuration for building LWS
type LWSConfig struct {
	// PodGroupName is the name of the PodGroup for gang scheduling
	PodGroupName string
	// TaskName is the task identifier within the PodGroup (matches minTaskMember keys)
	TaskName string
	// NeedsGangScheduling indicates if gang scheduling should be enabled
	NeedsGangScheduling bool
	// ReplicaIndex is set for per-replica LWS mode (each replica gets its own LWS)
	// If nil, creates a normal LWS with all replicas
	ReplicaIndex *int32
}

// BuildLWS constructs a LeaderWorkerSet for the given role
// If config.ReplicaIndex is set, creates a per-replica LWS with replicas=1
func BuildLWS(inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role, config LWSConfig) *lwsv1.LeaderWorkerSet {
	// Generate LWS name (includes replica index for per-replica mode)
	lwsName := GenerateLWSNameWithIndex(inferSvc.Name, role.Name, config.ReplicaIndex)

	// Determine LWS size (number of pods per replica)
	size := int32(1)
	if role.Multinode != nil && role.Multinode.NodeCount >= 1 {
		size = role.Multinode.NodeCount
	}

	// Determine replicas
	// Per-replica mode: always 1 replica per LWS
	// Normal mode: use role's replica count
	replicas := int32(1)
	if config.ReplicaIndex == nil && role.Replicas != nil {
		replicas = *role.Replicas
	}

	// Build base labels (without spec hash, which will be added after spec is built)
	labels := map[string]string{
		LabelService:       inferSvc.Name,
		LabelComponentType: string(role.ComponentType),
		LabelRoleName:      role.Name,
	}

	// Add replica index label for per-replica mode
	if config.ReplicaIndex != nil {
		labels[LabelReplicaIndex] = fmt.Sprintf("%d", *config.ReplicaIndex)
	}

	// Build annotations for gang scheduling
	annotations := make(map[string]string)
	if config.NeedsGangScheduling && config.PodGroupName != "" {
		annotations[AnnotationPodGroupName] = config.PodGroupName
		if config.TaskName != "" {
			annotations[AnnotationTaskSpec] = config.TaskName
		}
	}

	// Build pod specs
	isMultiNode := IsMultiNode(role)
	workerPodSpec := buildPodSpec(role, config.NeedsGangScheduling)

	lws := &lwsv1.LeaderWorkerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lwsName,
			Namespace: inferSvc.Namespace,
			Labels:    labels,
		},
		Spec: lwsv1.LeaderWorkerSetSpec{
			Replicas:      ptr.To(replicas),
			StartupPolicy: lwsv1.LeaderCreatedStartupPolicy,
			RolloutStrategy: lwsv1.RolloutStrategy{
				Type: lwsv1.RollingUpdateStrategyType,
			},
			LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
				Size: ptr.To(size),
				WorkerTemplate: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      labels,
						Annotations: annotations,
					},
					Spec: workerPodSpec,
				},
			},
		},
	}

	// For multi-node deployments, use separate LeaderTemplate and WorkerTemplate
	if isMultiNode {
		leaderPodSpec := buildPodSpec(role, config.NeedsGangScheduling)
		wrapLeaderContainer(&leaderPodSpec.Containers[0])
		wrapWorkerContainer(&lws.Spec.LeaderWorkerTemplate.WorkerTemplate.Spec.Containers[0])

		lws.Spec.LeaderWorkerTemplate.LeaderTemplate = &corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      labels,
				Annotations: annotations,
			},
			Spec: leaderPodSpec,
		}
	}

	// Compute spec hash after the spec is fully built and set it as a label
	specHash := util.ComputeSpecHash(lws.Spec)
	lws.Labels[LabelSpecHash] = specHash

	return lws
}

// buildPodSpec constructs the pod spec from the role template
func buildPodSpec(role fusioninferiov1alpha1.Role, needsGangScheduling bool) corev1.PodSpec {
	var spec corev1.PodSpec

	// Parse PodTemplateSpec from RawExtension
	if role.Template != nil && role.Template.Raw != nil {
		var template corev1.PodTemplateSpec
		if err := json.Unmarshal(role.Template.Raw, &template); err == nil {
			spec = *template.Spec.DeepCopy()
		}
	}

	// Set Volcano scheduler if gang scheduling is needed
	if needsGangScheduling {
		spec.SchedulerName = VolcanoSchedulerName
	}

	return spec
}

// wrapLeaderContainer wraps the leader container with Ray head + vLLM command
// Leader: ray start --head && <original command> --distributed-executor-backend ray
func wrapLeaderContainer(container *corev1.Container) {
	originalCmd := strings.Join(container.Command, " ")
	originalArgs := strings.Join(container.Args, " ")

	// If no command specified, use default vLLM serve command
	// This handles vllm-openai images where only args are provided
	if originalCmd == "" {
		originalCmd = "vllm serve"
	}

	// Check if --distributed-executor-backend is already specified
	hasBackendFlag := strings.Contains(originalArgs, "distributed-executor-backend")
	vllmMultinodeFlags := ""
	if !hasBackendFlag {
		vllmMultinodeFlags = "--distributed-executor-backend ray"
	}

	// Leader command: ray start --head && vllm serve ...
	leaderCmd := fmt.Sprintf("ray start --head --port=%d && %s %s %s",
		RayHeadPort, originalCmd, originalArgs, vllmMultinodeFlags)

	container.Command = []string{"/bin/sh", "-c"}
	container.Args = []string{leaderCmd}

	// Add Ray head port
	addRayHeadPort(container)

	// Add readiness probe for leader
	if container.ReadinessProbe == nil {
		container.ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(RayHeadPort),
				},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			TimeoutSeconds:      5,
			SuccessThreshold:    1,
			FailureThreshold:    6,
		}
	}
}

// wrapWorkerContainer wraps the worker container with Ray worker command
// Worker: ray start --address=<leader>:6379 --block
func wrapWorkerContainer(container *corev1.Container) {
	// Worker command: ray start --address --block
	workerCmd := fmt.Sprintf("ray start --address=$%s:%d --block",
		LWSLeaderAddressEnv, RayHeadPort)

	container.Command = []string{"/bin/sh", "-c"}
	container.Args = []string{workerCmd}
}

// addRayHeadPort adds the Ray head port to the container
func addRayHeadPort(container *corev1.Container) {
	container.Ports = append(container.Ports, corev1.ContainerPort{
		Name:          "ray-head",
		ContainerPort: RayHeadPort,
		Protocol:      corev1.ProtocolTCP,
	})
}

// generateLWSName generates the LWS name from InferenceService name and role name
// This is a private helper; use GenerateLWSNameWithIndex for external calls
func generateLWSName(inferSvcName, roleName string) string {
	return fmt.Sprintf("%s-%s", inferSvcName, roleName)
}

// GenerateLWSNameWithIndex generates the LWS name, including replica index for per-replica mode
func GenerateLWSNameWithIndex(inferSvcName, roleName string, replicaIndex *int32) string {
	if replicaIndex == nil {
		return generateLWSName(inferSvcName, roleName)
	}
	return fmt.Sprintf("%s-%s-%d", inferSvcName, roleName, *replicaIndex)
}

// IsMultiNode returns true if the role is configured for multi-node deployment
func IsMultiNode(role fusioninferiov1alpha1.Role) bool {
	return role.Multinode != nil && role.Multinode.NodeCount >= 2
}

// getNodeCount returns the node count for a role, defaulting to 1
// This is a local helper; for external use, call scheduling.GetNodeCount()
func getNodeCount(role fusioninferiov1alpha1.Role) int32 {
	if role.Multinode != nil && role.Multinode.NodeCount >= 1 {
		return role.Multinode.NodeCount
	}
	return 1
}
