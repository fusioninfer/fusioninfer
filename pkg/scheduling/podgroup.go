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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schedulingv1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/core/v1alpha1"
	"github.com/fusioninfer/fusioninfer/pkg/util"
	"github.com/fusioninfer/fusioninfer/pkg/workload"
)

// IsPDDisaggregated returns true if the InferenceService has both prefiller and decoder roles
func IsPDDisaggregated(inferSvc *fusioninferiov1alpha1.InferenceService) bool {
	hasPrefill := false
	hasDecode := false

	for _, role := range inferSvc.Spec.Roles {
		switch role.ComponentType {
		case fusioninferiov1alpha1.ComponentTypePrefiller:
			hasPrefill = true
		case fusioninferiov1alpha1.ComponentTypeDecoder:
			hasDecode = true
		}
	}

	return hasPrefill && hasDecode
}

// NeedsGangScheduling returns true if the InferenceService requires gang scheduling
// Gang scheduling is needed for:
// 1. Multi-node deployments (nodeCount >= 2)
// 2. PD disaggregated deployments (prefiller + decoder must be scheduled together)
func NeedsGangScheduling(inferSvc *fusioninferiov1alpha1.InferenceService) bool {
	// PD disaggregated always needs gang scheduling
	if IsPDDisaggregated(inferSvc) {
		return true
	}

	// Check if any role needs multi-node gang scheduling
	for _, role := range inferSvc.Spec.Roles {
		if role.ComponentType == fusioninferiov1alpha1.ComponentTypeRouter {
			continue
		}
		if role.Multinode != nil && role.Multinode.NodeCount >= 2 {
			return true
		}
	}

	return false
}

// NeedsGangSchedulingForRole returns true if the specific role requires gang scheduling
func NeedsGangSchedulingForRole(inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role) bool {
	// PD disaggregated: prefiller and decoder need gang scheduling
	if IsPDDisaggregated(inferSvc) {
		return role.ComponentType == fusioninferiov1alpha1.ComponentTypePrefiller ||
			role.ComponentType == fusioninferiov1alpha1.ComponentTypeDecoder
	}

	// Multi-node needs gang scheduling
	return role.Multinode != nil && role.Multinode.NodeCount >= 2
}

// BuildPodGroup constructs a Volcano PodGroup for the InferenceService
// Creates a single shared PodGroup with minTaskMember keyed by {roleName}-{replicaIndex}
// This ensures both cross-role scheduling (PD) and intra-replica atomic scheduling (multi-node)
//
// Example for PD disaggregated with prefill(replicas=1, nodes=2) + decode(replicas=2, nodes=4):
//
//	minMember: 10 (2 + 4 + 4)
//	minTaskMember:
//	  prefill-0: 2   # All 2 nodes in prefill replica 0 must be scheduled together
//	  decode-0: 4    # All 4 nodes in decode replica 0 must be scheduled together
//	  decode-1: 4    # All 4 nodes in decode replica 1 must be scheduled together
//
// The scheduler will ensure at least one complete replica of each role is scheduled,
// and each replica's pods are scheduled atomically (all-or-nothing within a replica).
func BuildPodGroup(inferSvc *fusioninferiov1alpha1.InferenceService) *schedulingv1beta1.PodGroup {
	pgName := GeneratePodGroupName(inferSvc.Name)

	// Calculate minMember and minTaskMember based on roles
	minMember := int32(0)
	minTaskMember := make(map[string]int32)
	minResources := corev1.ResourceList{}

	for _, role := range inferSvc.Spec.Roles {
		// Skip router roles
		if role.ComponentType == fusioninferiov1alpha1.ComponentTypeRouter {
			continue
		}

		// Skip roles that don't need gang scheduling
		if !NeedsGangSchedulingForRole(inferSvc, role) {
			continue
		}

		// Get replica count and node count
		replicas := GetReplicaCount(role)
		nodeCount := GetNodeCount(role)

		// Create minTaskMember entry for each replica
		// Key format: {roleName}-{replicaIndex}
		// Value: nodeCount (all pods in the replica must be scheduled together)
		for i := int32(0); i < replicas; i++ {
			taskName := GenerateTaskName(role.Name, i)
			minTaskMember[taskName] = nodeCount
			minMember += nodeCount
		}

		// Calculate total resources needed
		addRoleResources(minResources, role, replicas*nodeCount)
	}

	pg := &schedulingv1beta1.PodGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgName,
			Namespace: inferSvc.Namespace,
			Labels: map[string]string{
				workload.LabelService: inferSvc.Name,
			},
		},
		Spec: schedulingv1beta1.PodGroupSpec{
			MinMember:     minMember,
			MinTaskMember: minTaskMember,
			MinResources:  &minResources,
		},
	}

	// Compute spec hash after the spec is fully built
	pg.Labels[workload.LabelSpecHash] = util.ComputeSpecHash(pg.Spec)

	return pg
}

// addRoleResources adds the resource requirements for a role to the resource list
func addRoleResources(resources corev1.ResourceList, role fusioninferiov1alpha1.Role, totalPods int32) {
	if role.Template == nil || role.Template.Raw == nil {
		return
	}

	// Parse PodTemplateSpec from RawExtension
	var template corev1.PodTemplateSpec
	if err := json.Unmarshal(role.Template.Raw, &template); err != nil {
		return
	}

	if len(template.Spec.Containers) == 0 {
		return
	}

	for _, container := range template.Spec.Containers {
		if container.Resources.Limits != nil {
			for resourceName, quantity := range container.Resources.Limits {
				// Multiply by totalPods
				totalQuantity := quantity.DeepCopy()
				totalQuantity.Set(quantity.Value() * int64(totalPods))

				if existing, ok := resources[resourceName]; ok {
					existing.Add(totalQuantity)
					resources[resourceName] = existing
				} else {
					resources[resourceName] = totalQuantity
				}
			}
		}
	}
}

// GeneratePodGroupName generates the PodGroup name for an InferenceService
func GeneratePodGroupName(inferSvcName string) string {
	return inferSvcName
}

// GenerateTaskName generates the task name for minTaskMember
// Format: {roleName}-{replicaIndex}
// This matches the volcano.sh/task-spec annotation value in pod templates
func GenerateTaskName(roleName string, replicaIndex int32) string {
	return fmt.Sprintf("%s-%d", roleName, replicaIndex)
}

// GetNodeCount returns the node count for a role, defaulting to 1
func GetNodeCount(role fusioninferiov1alpha1.Role) int32 {
	if role.Multinode != nil && role.Multinode.NodeCount >= 1 {
		return role.Multinode.NodeCount
	}
	return 1
}

// GetReplicaCount returns the replica count for a role, defaulting to 1
func GetReplicaCount(role fusioninferiov1alpha1.Role) int32 {
	if role.Replicas != nil {
		return *role.Replicas
	}
	return 1
}
