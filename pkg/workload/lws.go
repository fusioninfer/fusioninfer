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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/v1alpha1"
)

const (
	// Labels
	LabelService       = "fusioninfer.io/service"
	LabelComponentType = "fusioninfer.io/component-type"
	LabelRoleName      = "fusioninfer.io/role-name"

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
}

// BuildLWS constructs a LeaderWorkerSet for the given role
func BuildLWS(inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role, config LWSConfig) *lwsv1.LeaderWorkerSet {
	lwsName := GenerateLWSName(inferSvc.Name, role.Name)

	// Determine LWS size (number of pods per replica)
	size := int32(1)
	if role.Multinode != nil && role.Multinode.NodeCount >= 1 {
		size = role.Multinode.NodeCount
	}

	// Determine replicas
	replicas := int32(1)
	if role.Replicas != nil {
		replicas = *role.Replicas
	}

	// Build labels
	labels := map[string]string{
		LabelService:       inferSvc.Name,
		LabelComponentType: string(role.ComponentType),
		LabelRoleName:      role.Name,
	}

	// Build annotations for gang scheduling
	annotations := make(map[string]string)
	if config.NeedsGangScheduling && config.PodGroupName != "" {
		annotations[AnnotationPodGroupName] = config.PodGroupName
		if config.TaskName != "" {
			annotations[AnnotationTaskSpec] = config.TaskName
		}
	}

	// Build pod spec
	podSpec := buildPodSpec(role, config.NeedsGangScheduling)

	lws := &lwsv1.LeaderWorkerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lwsName,
			Namespace: inferSvc.Namespace,
			Labels:    labels,
		},
		Spec: lwsv1.LeaderWorkerSetSpec{
			Replicas: ptr.To(replicas),
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
func buildPodSpec(role fusioninferiov1alpha1.Role, needsGangScheduling bool) corev1.PodSpec {
	var spec corev1.PodSpec

	if role.Template != nil && role.Template.Spec.Containers != nil {
		spec = *role.Template.Spec.DeepCopy()
	}

	// Set Volcano scheduler if gang scheduling is needed
	if needsGangScheduling {
		spec.SchedulerName = VolcanoSchedulerName
	}

	return spec
}

// GenerateLWSName generates the LWS name from InferenceService name and role name
func GenerateLWSName(inferSvcName, roleName string) string {
	return fmt.Sprintf("%s-%s", inferSvcName, roleName)
}

// IsMultiNode returns true if the role is configured for multi-node deployment
func IsMultiNode(role fusioninferiov1alpha1.Role) bool {
	return role.Multinode != nil && role.Multinode.NodeCount >= 2
}

// GetReplicaCount returns the replica count for a role, defaulting to 1
func GetReplicaCount(role fusioninferiov1alpha1.Role) int32 {
	if role.Replicas != nil {
		return *role.Replicas
	}
	return 1
}

// GetNodeCount returns the node count for a role, defaulting to 1
func GetNodeCount(role fusioninferiov1alpha1.Role) int32 {
	if role.Multinode != nil && role.Multinode.NodeCount >= 1 {
		return role.Multinode.NodeCount
	}
	return 1
}
