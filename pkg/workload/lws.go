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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/core/v1alpha1"
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
	LabelRevision      = "fusioninfer.io/revision"

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

	// Build labels with revision for update detection
	labels := map[string]string{
		LabelService:       inferSvc.Name,
		LabelComponentType: string(role.ComponentType),
		LabelRoleName:      role.Name,
		LabelRevision:      fmt.Sprintf("%d", inferSvc.Generation),
	}

	// Add replica index label for per-replica mode
	if config.ReplicaIndex != nil {
		labels[LabelReplicaIndex] = strconv.Itoa(int(*config.ReplicaIndex))
	}

	// Build annotations for gang scheduling
	annotations := make(map[string]string)
	if config.NeedsGangScheduling && config.PodGroupName != "" {
		annotations[AnnotationPodGroupName] = config.PodGroupName
		if config.TaskName != "" {
			annotations[AnnotationTaskSpec] = config.TaskName
		}
	}

	// Build pod spec with optional ray command wrapping for multi-node
	podSpec := buildPodSpec(role, config.NeedsGangScheduling, IsMultiNode(role))

	lws := &lwsv1.LeaderWorkerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lwsName,
			Namespace: inferSvc.Namespace,
			Labels:    labels,
		},
		Spec: lwsv1.LeaderWorkerSetSpec{
			Replicas:      ptr.To(replicas),
			StartupPolicy: lwsv1.LeaderReadyStartupPolicy,
			LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
				Size: ptr.To(size),
				WorkerTemplate: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      labels,
						Annotations: annotations,
					},
					Spec: podSpec,
				},
			},
		},
	}

	return lws
}

// buildPodSpec constructs the pod spec from the role template
func buildPodSpec(role fusioninferiov1alpha1.Role, needsGangScheduling bool, isMultiNode bool) corev1.PodSpec {
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

	// Wrap command with ray symmetric-run for multi-node deployments
	if isMultiNode && len(spec.Containers) > 0 {
		wrapContainerWithRay(&spec.Containers[0], role)
	}

	return spec
}

// wrapContainerWithRay wraps the container command with ray symmetric-run for distributed inference
// This enables multi-node tensor parallelism using Ray's distributed execution
func wrapContainerWithRay(container *corev1.Container, role fusioninferiov1alpha1.Role) {
	nodeCount := getNodeCount(role)
	numGPUs := getGPUCount(container)

	// Build the original command/args
	originalCmd := container.Command
	originalArgs := container.Args

	// Construct ray symmetric-run command
	// Format: ray symmetric-run --address $(LWS_LEADER_ADDRESS):6379 --min-nodes N --num-gpus G -- <original_cmd> <original_args>
	rayArgs := []string{
		"symmetric-run",
		"--address",
		fmt.Sprintf("$(%s):%d", LWSLeaderAddressEnv, RayHeadPort),
		"--min-nodes",
		strconv.Itoa(int(nodeCount)),
	}

	// Add num-gpus if available
	if numGPUs > 0 {
		rayArgs = append(rayArgs, "--num-gpus", strconv.Itoa(numGPUs))
	}

	// Add separator and original command/args
	rayArgs = append(rayArgs, "--")

	// Append original command if exists
	if len(originalCmd) > 0 {
		rayArgs = append(rayArgs, originalCmd...)
	}

	// Append original args
	if len(originalArgs) > 0 {
		rayArgs = append(rayArgs, originalArgs...)
	}

	// Set new command and args
	container.Command = []string{"ray"}
	container.Args = rayArgs

	// Ensure Ray head port is exposed
	ensureRayHeadPort(container)

	// Add readiness probe to check Ray head is ready
	// This is used with LeaderReadyStartupPolicy to ensure workers wait for leader
	if container.ReadinessProbe == nil {
		container.ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(RayHeadPort),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       5,
			TimeoutSeconds:      3,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}
	}
}

// getGPUCount extracts the number of GPUs from container resource limits
func getGPUCount(container *corev1.Container) int {
	if container.Resources.Limits == nil {
		return 0
	}

	gpuResource := container.Resources.Limits[corev1.ResourceName("nvidia.com/gpu")]
	if gpuResource.IsZero() {
		return 0
	}

	return int(gpuResource.Value())
}

// ensureRayHeadPort ensures the Ray head port is exposed in the container
func ensureRayHeadPort(container *corev1.Container) {
	for _, port := range container.Ports {
		if port.ContainerPort == RayHeadPort {
			return // Already exposed
		}
	}

	// Add Ray head port
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
