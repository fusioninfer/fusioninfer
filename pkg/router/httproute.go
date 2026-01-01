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
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/core/v1alpha1"
	"github.com/fusioninfer/fusioninfer/pkg/util"
	"github.com/fusioninfer/fusioninfer/pkg/workload"
)

const (
	// InferencePool API group
	InferencePoolGroup = "inference.networking.k8s.io"
	InferencePoolKind  = "InferencePool"
)

// BuildHTTPRoute constructs an HTTPRoute that routes traffic to the InferencePool
func BuildHTTPRoute(
	inferSvc *fusioninferiov1alpha1.InferenceService,
	role fusioninferiov1alpha1.Role,
) *gatewayv1.HTTPRoute {
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

	// Parse user-provided HTTPRouteSpec from RawExtension
	if role.HTTPRoute != nil && role.HTTPRoute.Raw != nil {
		var spec gatewayv1.HTTPRouteSpec
		if err := json.Unmarshal(role.HTTPRoute.Raw, &spec); err == nil {
			httpRoute.Spec = spec
		}
	}

	// Always add/override the InferencePool backend rule
	httpRoute.Spec.Rules = []gatewayv1.HTTPRouteRule{
		{
			BackendRefs: []gatewayv1.HTTPBackendRef{
				buildInferencePoolBackendRef(poolName),
			},
		},
	}

	// Compute spec hash after the spec is fully built
	httpRoute.Labels[workload.LabelSpecHash] = util.ComputeSpecHash(httpRoute.Spec)

	return httpRoute
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
