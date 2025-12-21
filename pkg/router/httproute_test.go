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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/v1alpha1"
	"github.com/fusioninfer/fusioninfer/pkg/workload"
)

func TestBuildHTTPRoute(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	role := fusioninferiov1alpha1.Role{
		Name:          "router",
		ComponentType: fusioninferiov1alpha1.ComponentTypeRouter,
	}

	httpRoute := BuildHTTPRoute(inferSvc, role)

	// Verify route name
	expectedName := "test-service-httproute"
	if httpRoute.Name != expectedName {
		t.Errorf("expected route name %s, got %s", expectedName, httpRoute.Name)
	}

	// Verify namespace
	if httpRoute.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %s", httpRoute.Namespace)
	}

	// Verify labels
	if httpRoute.Labels[workload.LabelService] != "test-service" {
		t.Errorf("expected label %s=%s, got %s", workload.LabelService, "test-service", httpRoute.Labels[workload.LabelService])
	}

	// Verify default rule is created
	if len(httpRoute.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(httpRoute.Spec.Rules))
	}

	// Verify backend ref points to InferencePool
	if len(httpRoute.Spec.Rules[0].BackendRefs) != 1 {
		t.Fatalf("expected 1 backend ref, got %d", len(httpRoute.Spec.Rules[0].BackendRefs))
	}

	backendRef := httpRoute.Spec.Rules[0].BackendRefs[0]
	if *backendRef.BackendRef.Group != gatewayv1.Group(InferencePoolGroup) {
		t.Errorf("expected group %s, got %s", InferencePoolGroup, *backendRef.BackendRef.Group)
	}
	if *backendRef.BackendRef.Kind != gatewayv1.Kind(InferencePoolKind) {
		t.Errorf("expected kind %s, got %s", InferencePoolKind, *backendRef.BackendRef.Kind)
	}

	expectedPoolName := "test-service-pool"
	if string(backendRef.BackendRef.Name) != expectedPoolName {
		t.Errorf("expected pool name %s, got %s", expectedPoolName, backendRef.BackendRef.Name)
	}
}

func TestBuildHTTPRouteWithUserProvidedSpec(t *testing.T) {
	hostname := gatewayv1.Hostname("api.example.com")
	parentRefName := gatewayv1.ObjectName("my-gateway")

	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	role := fusioninferiov1alpha1.Role{
		Name:          "router",
		ComponentType: fusioninferiov1alpha1.ComponentTypeRouter,
		HTTPRoute: &gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Name: parentRefName,
					},
				},
			},
			Hostnames: []gatewayv1.Hostname{hostname},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  ptrTo(gatewayv1.PathMatchPathPrefix),
								Value: ptrTo("/v1"),
							},
						},
					},
				},
			},
		},
	}

	httpRoute := BuildHTTPRoute(inferSvc, role)

	// Verify hostname is preserved
	if len(httpRoute.Spec.Hostnames) != 1 || httpRoute.Spec.Hostnames[0] != hostname {
		t.Errorf("expected hostname %s to be preserved", hostname)
	}

	// Verify parent ref is preserved
	if len(httpRoute.Spec.ParentRefs) != 1 || httpRoute.Spec.ParentRefs[0].Name != parentRefName {
		t.Error("expected parent ref to be preserved")
	}

	// Verify backend ref is updated to InferencePool
	if len(httpRoute.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(httpRoute.Spec.Rules))
	}

	backendRef := httpRoute.Spec.Rules[0].BackendRefs[0]
	expectedPoolName := "test-service-pool"
	if string(backendRef.BackendRef.Name) != expectedPoolName {
		t.Errorf("expected pool name %s, got %s", expectedPoolName, backendRef.BackendRef.Name)
	}
}

func TestBuildHTTPRouteWithMultipleRules(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	role := fusioninferiov1alpha1.Role{
		Name:          "router",
		ComponentType: fusioninferiov1alpha1.ComponentTypeRouter,
		HTTPRoute: &gatewayv1.HTTPRouteSpec{
			Rules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  ptrTo(gatewayv1.PathMatchPathPrefix),
								Value: ptrTo("/v1/chat"),
							},
						},
					},
				},
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  ptrTo(gatewayv1.PathMatchPathPrefix),
								Value: ptrTo("/v1/completions"),
							},
						},
					},
				},
			},
		},
	}

	httpRoute := BuildHTTPRoute(inferSvc, role)

	// Verify both rules have InferencePool backend
	if len(httpRoute.Spec.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(httpRoute.Spec.Rules))
	}

	expectedPoolName := "test-service-pool"
	for i, rule := range httpRoute.Spec.Rules {
		if len(rule.BackendRefs) != 1 {
			t.Errorf("rule %d: expected 1 backend ref, got %d", i, len(rule.BackendRefs))
			continue
		}
		if string(rule.BackendRefs[0].BackendRef.Name) != expectedPoolName {
			t.Errorf("rule %d: expected pool name %s, got %s", i, expectedPoolName, rule.BackendRefs[0].BackendRef.Name)
		}
	}
}

// helper function
func ptrTo[T any](v T) *T {
	return &v
}

