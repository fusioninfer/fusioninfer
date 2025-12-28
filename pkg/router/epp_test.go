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
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/core/v1alpha1"
	"github.com/fusioninfer/fusioninfer/pkg/workload"
)

func TestBuildEPPConfigMap(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	role := fusioninferiov1alpha1.Role{
		Name:          "router",
		ComponentType: fusioninferiov1alpha1.ComponentTypeRouter,
		Strategy:      fusioninferiov1alpha1.StrategyPrefixCache,
	}

	cm := BuildEPPConfigMap(inferSvc, role)

	// Verify ConfigMap name
	expectedName := "test-service-epp-config"
	if cm.Name != expectedName {
		t.Errorf("expected ConfigMap name %s, got %s", expectedName, cm.Name)
	}

	// Verify namespace
	if cm.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %s", cm.Namespace)
	}

	// Verify label
	if cm.Labels[workload.LabelService] != "test-service" {
		t.Errorf("expected label %s=%s, got %s", workload.LabelService, "test-service", cm.Labels[workload.LabelService])
	}

	// Verify config file exists
	if _, ok := cm.Data[EPPConfigFileName]; !ok {
		t.Errorf("expected config file %s to exist in ConfigMap data", EPPConfigFileName)
	}
}

func TestBuildEPPDeployment(t *testing.T) {
	// Set EPP image env var for test
	testImage := "ghcr.io/kubernetes-sigs/gateway-api-inference-extension/epp:v1.2.1"
	os.Setenv(EnvEPPImage, testImage)
	defer os.Unsetenv(EnvEPPImage)

	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	deploy := BuildEPPDeployment(inferSvc)

	// Verify Deployment name
	expectedName := "test-service-epp"
	if deploy.Name != expectedName {
		t.Errorf("expected Deployment name %s, got %s", expectedName, deploy.Name)
	}

	// Verify namespace
	if deploy.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %s", deploy.Namespace)
	}

	// Verify replicas
	if *deploy.Spec.Replicas != 1 {
		t.Errorf("expected 1 replica, got %d", *deploy.Spec.Replicas)
	}

	// Verify container image
	if len(deploy.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(deploy.Spec.Template.Spec.Containers))
	}

	container := deploy.Spec.Template.Spec.Containers[0]
	if container.Image != testImage {
		t.Errorf("expected image %s, got %s", testImage, container.Image)
	}

	// Verify container ports
	expectedPorts := map[int32]string{
		EPPGRPCPort:    "grpc",
		EPPHealthPort:  "grpc-health",
		EPPMetricsPort: "metrics",
	}

	for _, port := range container.Ports {
		expectedName, ok := expectedPorts[port.ContainerPort]
		if !ok {
			t.Errorf("unexpected port %d", port.ContainerPort)
			continue
		}
		if port.Name != expectedName {
			t.Errorf("expected port name %s for port %d, got %s", expectedName, port.ContainerPort, port.Name)
		}
	}

	// Verify ServiceAccountName
	if deploy.Spec.Template.Spec.ServiceAccountName != expectedName {
		t.Errorf("expected ServiceAccountName %s, got %s", expectedName, deploy.Spec.Template.Spec.ServiceAccountName)
	}

	// Verify volume mount
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("expected 1 volume mount, got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].MountPath != EPPConfigPath {
		t.Errorf("expected mount path %s, got %s", EPPConfigPath, container.VolumeMounts[0].MountPath)
	}

	// Verify liveness probe
	if container.LivenessProbe == nil {
		t.Error("expected liveness probe to be set")
	} else if container.LivenessProbe.GRPC == nil {
		t.Error("expected GRPC liveness probe")
	} else if container.LivenessProbe.GRPC.Port != EPPHealthPort {
		t.Errorf("expected liveness probe port %d, got %d", EPPHealthPort, container.LivenessProbe.GRPC.Port)
	}

	// Verify readiness probe
	if container.ReadinessProbe == nil {
		t.Error("expected readiness probe to be set")
	} else if container.ReadinessProbe.GRPC == nil {
		t.Error("expected GRPC readiness probe")
	}
}

func TestBuildEPPService(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	svc := BuildEPPService(inferSvc)

	// Verify Service name
	expectedName := "test-service-epp"
	if svc.Name != expectedName {
		t.Errorf("expected Service name %s, got %s", expectedName, svc.Name)
	}

	// Verify namespace
	if svc.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %s", svc.Namespace)
	}

	// Verify service type
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected ServiceType ClusterIP, got %s", svc.Spec.Type)
	}

	// Verify selector
	if svc.Spec.Selector["app"] != expectedName {
		t.Errorf("expected selector app=%s, got %s", expectedName, svc.Spec.Selector["app"])
	}

	// Verify ports
	if len(svc.Spec.Ports) != 3 {
		t.Fatalf("expected 3 ports, got %d", len(svc.Spec.Ports))
	}

	portMap := make(map[int32]string)
	for _, port := range svc.Spec.Ports {
		portMap[port.Port] = port.Name
	}

	expectedServicePorts := map[int32]string{
		EPPGRPCPort:    "grpc-ext-proc",
		EPPHealthPort:  "grpc-health",
		EPPMetricsPort: "http-metrics",
	}

	for port, name := range expectedServicePorts {
		if portMap[port] != name {
			t.Errorf("expected port %d to have name %s, got %s", port, name, portMap[port])
		}
	}
}

func TestBuildEPPServiceAccount(t *testing.T) {
	inferSvc := &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
	}

	sa := BuildEPPServiceAccount(inferSvc)

	// Verify ServiceAccount name
	expectedName := "test-service-epp"
	if sa.Name != expectedName {
		t.Errorf("expected ServiceAccount name %s, got %s", expectedName, sa.Name)
	}

	// Verify namespace
	if sa.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %s", sa.Namespace)
	}

	// Verify label
	if sa.Labels[workload.LabelService] != "test-service" {
		t.Errorf("expected label %s=%s, got %s", workload.LabelService, "test-service", sa.Labels[workload.LabelService])
	}
}

func TestGenerateEPPNames(t *testing.T) {
	testCases := []struct {
		inferSvcName string
		wantPool     string
		wantService  string
		wantDeploy   string
		wantConfig   string
	}{
		{
			inferSvcName: "my-service",
			wantPool:     "my-service-pool",
			wantService:  "my-service-epp",
			wantDeploy:   "my-service-epp",
			wantConfig:   "my-service-epp-config",
		},
		{
			inferSvcName: "qwen-inference",
			wantPool:     "qwen-inference-pool",
			wantService:  "qwen-inference-epp",
			wantDeploy:   "qwen-inference-epp",
			wantConfig:   "qwen-inference-epp-config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.inferSvcName, func(t *testing.T) {
			if got := GeneratePoolName(tc.inferSvcName); got != tc.wantPool {
				t.Errorf("GeneratePoolName() = %s, want %s", got, tc.wantPool)
			}
			if got := GenerateEPPServiceName(tc.inferSvcName); got != tc.wantService {
				t.Errorf("GenerateEPPServiceName() = %s, want %s", got, tc.wantService)
			}
			if got := GenerateEPPDeploymentName(tc.inferSvcName); got != tc.wantDeploy {
				t.Errorf("GenerateEPPDeploymentName() = %s, want %s", got, tc.wantDeploy)
			}
			if got := GenerateEPPConfigMapName(tc.inferSvcName); got != tc.wantConfig {
				t.Errorf("GenerateEPPConfigMapName() = %s, want %s", got, tc.wantConfig)
			}
		})
	}
}

func TestGetEPPImage(t *testing.T) {
	// Test image from env var
	t.Run("image from env", func(t *testing.T) {
		customImage := "my-registry.io/epp:custom-tag"
		os.Setenv(EnvEPPImage, customImage)
		defer os.Unsetenv(EnvEPPImage)

		got := GetEPPImage()
		if got != customImage {
			t.Errorf("GetEPPImage() = %s, want %s", got, customImage)
		}
	})

	// Test default when env var is not set
	t.Run("default when env not set", func(t *testing.T) {
		os.Unsetenv(EnvEPPImage)

		got := GetEPPImage()
		if got != DefaultEPPImage {
			t.Errorf("GetEPPImage() = %s, want %s", got, DefaultEPPImage)
		}
	})
}
