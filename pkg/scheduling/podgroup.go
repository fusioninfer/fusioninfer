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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schedulingv1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/v1alpha1"
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

// NeedsGangScheduling returns true if the role requires gang scheduling
// Gang scheduling is needed for:
// 1. Multi-node deployments (nodeCount >= 2)
// 2. PD disaggregated deployments (prefiller + decoder must be scheduled together)
func NeedsGangScheduling(inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role) bool {
	// Multi-node always needs gang scheduling
	if role.Multinode != nil && role.Multinode.NodeCount >= 2 {
		return true
	}

	// PD disaggregated needs cross-role gang scheduling
	if IsPDDisaggregated(inferSvc) {
		return role.ComponentType == fusioninferiov1alpha1.ComponentTypePrefiller ||
			role.ComponentType == fusioninferiov1alpha1.ComponentTypeDecoder
	}

	return false
}

// BuildPodGroup constructs a Volcano PodGroup for the InferenceService
// For PD disaggregated scenarios, it creates a single PodGroup spanning all roles
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

		taskName := string(role.ComponentType)

		// For PD disaggregated, we need at least 1 of each role
		if IsPDDisaggregated(inferSvc) {
			minTaskMember[taskName] = 1
			minMember++

			// Calculate resources for one replica of this role
			addRoleResources(minResources, role, 1)
		} else {
			// For monolithic multi-node, each replica needs all its nodes
			nodeCount := int32(1)
			if role.Multinode != nil && role.Multinode.NodeCount >= 1 {
				nodeCount = role.Multinode.NodeCount
			}
			minMember += nodeCount

			// Calculate resources for one replica
			addRoleResources(minResources, role, nodeCount)
		}
	}

	pg := &schedulingv1beta1.PodGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgName,
			Namespace: inferSvc.Namespace,
		},
		Spec: schedulingv1beta1.PodGroupSpec{
			MinMember:    minMember,
			MinResources: &minResources,
		},
	}

	// Only set minTaskMember for PD disaggregated scenarios
	if IsPDDisaggregated(inferSvc) && len(minTaskMember) > 0 {
		pg.Spec.MinTaskMember = minTaskMember
	}

	return pg
}

// BuildPerReplicaPodGroup constructs a PodGroup for a single LWS replica
// Used for monolithic multi-node deployments where each replica is scheduled independently
func BuildPerReplicaPodGroup(inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role, replicaIndex int32) *schedulingv1beta1.PodGroup {
	pgName := GeneratePerReplicaPodGroupName(inferSvc.Name, role.Name, replicaIndex)

	nodeCount := int32(1)
	if role.Multinode != nil && role.Multinode.NodeCount >= 1 {
		nodeCount = role.Multinode.NodeCount
	}

	minResources := corev1.ResourceList{}
	addRoleResources(minResources, role, nodeCount)

	return &schedulingv1beta1.PodGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgName,
			Namespace: inferSvc.Namespace,
		},
		Spec: schedulingv1beta1.PodGroupSpec{
			MinMember:    nodeCount,
			MinResources: &minResources,
		},
	}
}

// addRoleResources adds the resource requirements for a role to the resource list
func addRoleResources(resources corev1.ResourceList, role fusioninferiov1alpha1.Role, nodeCount int32) {
	if role.Template == nil || len(role.Template.Spec.Containers) == 0 {
		return
	}

	for _, container := range role.Template.Spec.Containers {
		if container.Resources.Limits != nil {
			for resourceName, quantity := range container.Resources.Limits {
				// Multiply by nodeCount
				totalQuantity := quantity.DeepCopy()
				totalQuantity.Set(quantity.Value() * int64(nodeCount))

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

// GeneratePodGroupName generates the PodGroup name for cross-role gang scheduling
func GeneratePodGroupName(inferSvcName string) string {
	return inferSvcName
}

// GeneratePerReplicaPodGroupName generates the PodGroup name for a specific LWS replica
func GeneratePerReplicaPodGroupName(inferSvcName, roleName string, replicaIndex int32) string {
	return resource.NewQuantity(int64(replicaIndex), resource.DecimalSI).String()
}

// GetWorkerRoles returns all non-router roles from the InferenceService
func GetWorkerRoles(inferSvc *fusioninferiov1alpha1.InferenceService) []fusioninferiov1alpha1.Role {
	var workerRoles []fusioninferiov1alpha1.Role
	for _, role := range inferSvc.Spec.Roles {
		if role.ComponentType != fusioninferiov1alpha1.ComponentTypeRouter {
			workerRoles = append(workerRoles, role)
		}
	}
	return workerRoles
}
