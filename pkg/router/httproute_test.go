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
	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	ns := gatewayv1.Namespace("gateway-ns")
	role := fusioninferiov1alpha1.Role{
		Name:          "router",
		ComponentType: fusioninferiov1alpha1.ComponentTypeRouter,
		HTTPRoute: &gatewayv1.HTTPRouteSpec{
			ParentRefs: []gatewayv1.ParentReference{
				{
					Name:      "my-gateway",
					Namespace: &ns,
				},
			},
			Hostnames: []gatewayv1.Hostname{"api.example.com"},
		},
	}

	httpRoute := BuildHTTPRoute(inferSvc, role)

	// Verify hostname is preserved
	if len(httpRoute.Spec.Hostnames) != 1 || httpRoute.Spec.Hostnames[0] != "api.example.com" {
		t.Errorf("expected hostname api.example.com to be preserved")
	}

	// Verify parent ref is preserved
	if len(httpRoute.Spec.ParentRefs) != 1 {
		t.Fatalf("expected 1 parent ref, got %d", len(httpRoute.Spec.ParentRefs))
	}
	if string(httpRoute.Spec.ParentRefs[0].Name) != "my-gateway" {
		t.Error("expected parent ref name my-gateway")
	}
	if httpRoute.Spec.ParentRefs[0].Namespace == nil || string(*httpRoute.Spec.ParentRefs[0].Namespace) != "gateway-ns" {
		t.Error("expected parent ref namespace gateway-ns")
	}

	// Verify backend ref is set to InferencePool
	if len(httpRoute.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(httpRoute.Spec.Rules))
	}

	backendRef := httpRoute.Spec.Rules[0].BackendRefs[0]
	expectedPoolName := "test-service-pool"
	if string(backendRef.BackendRef.Name) != expectedPoolName {
		t.Errorf("expected pool name %s, got %s", expectedPoolName, backendRef.BackendRef.Name)
	}
}

func TestBuildHTTPRouteWithSectionName(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	sectionName := gatewayv1.SectionName("https")
	role := fusioninferiov1alpha1.Role{
		Name:          "router",
		ComponentType: fusioninferiov1alpha1.ComponentTypeRouter,
		HTTPRoute: &gatewayv1.HTTPRouteSpec{
			ParentRefs: []gatewayv1.ParentReference{
				{
					Name:        "my-gateway",
					SectionName: &sectionName,
				},
			},
		},
	}

	httpRoute := BuildHTTPRoute(inferSvc, role)

	// Verify section name is preserved
	if len(httpRoute.Spec.ParentRefs) != 1 {
		t.Fatalf("expected 1 parent ref, got %d", len(httpRoute.Spec.ParentRefs))
	}
	if httpRoute.Spec.ParentRefs[0].SectionName == nil || string(*httpRoute.Spec.ParentRefs[0].SectionName) != "https" {
		t.Error("expected section name https")
	}
}
