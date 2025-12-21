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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/v1alpha1"
	"github.com/fusioninfer/fusioninfer/pkg/workload"
)

const (
	// InferencePool API group
	InferencePoolGroup = "inference.networking.x-k8s.io"
	InferencePoolKind  = "InferencePool"
)

// BuildHTTPRoute constructs an HTTPRoute that routes traffic to the InferencePool
func BuildHTTPRoute(inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role) *gatewayv1.HTTPRoute {
	routeName := GenerateHTTPRouteName(inferSvc.Name)
	poolName := GeneratePoolName(inferSvc.Name)

	httpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: inferSvc.Namespace,
			Labels: map[string]string{
				workload.LabelService: inferSvc.Name,
			},
		},
		Spec: gatewayv1.HTTPRouteSpec{},
	}

	// Copy user-provided HTTPRoute spec if available
	if role.HTTPRoute != nil {
		httpRoute.Spec = *role.HTTPRoute.DeepCopy()
	}

	// Ensure backend refs point to our InferencePool
	httpRoute.Spec.Rules = ensureInferencePoolBackendRef(httpRoute.Spec.Rules, poolName)

	return httpRoute
}

// ensureInferencePoolBackendRef ensures HTTPRoute rules have backend refs pointing to the InferencePool
func ensureInferencePoolBackendRef(rules []gatewayv1.HTTPRouteRule, poolName string) []gatewayv1.HTTPRouteRule {
	if len(rules) == 0 {
		// Create a default rule if none provided
		rules = []gatewayv1.HTTPRouteRule{
			{
				BackendRefs: []gatewayv1.HTTPBackendRef{
					buildInferencePoolBackendRef(poolName),
				},
			},
		}
		return rules
	}

	// Ensure each rule has the InferencePool backend ref
	for i := range rules {
		if len(rules[i].BackendRefs) == 0 {
			rules[i].BackendRefs = []gatewayv1.HTTPBackendRef{
				buildInferencePoolBackendRef(poolName),
			}
		} else {
			// Update existing backend refs to point to InferencePool
			for j := range rules[i].BackendRefs {
				rules[i].BackendRefs[j] = buildInferencePoolBackendRef(poolName)
			}
		}
	}

	return rules
}

// buildInferencePoolBackendRef creates a backend ref pointing to an InferencePool
func buildInferencePoolBackendRef(poolName string) gatewayv1.HTTPBackendRef {
	group := gatewayv1.Group(InferencePoolGroup)
	kind := gatewayv1.Kind(InferencePoolKind)

	return gatewayv1.HTTPBackendRef{
		BackendRef: gatewayv1.BackendRef{
			BackendObjectReference: gatewayv1.BackendObjectReference{
				Group: &group,
				Kind:  &kind,
				Name:  gatewayv1.ObjectName(poolName),
			},
		},
	}
}
