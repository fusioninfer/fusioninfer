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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/v1alpha1"
	"github.com/fusioninfer/fusioninfer/pkg/workload"
)

const (
	// EPP container ports
	EPPGRPCPort       = 9002
	EPPHealthPort     = 9003
	EPPMetricsPort    = 9090
	EPPConfigPath     = "/config"
	EPPConfigFileName = "config.yaml"

	// Environment variable name for EPP image
	EnvEPPImage = "EPP_IMAGE"

	// DefaultEPPImage is the default EPP image if EPP_IMAGE env is not set
	DefaultEPPImage = "registry.k8s.io/gateway-api-inference-extension/epp:v1.2.1"
)

// GetEPPImage returns the EPP image from environment variable or default
func GetEPPImage() string {
	if image := os.Getenv(EnvEPPImage); image != "" {
		return image
	}
	return DefaultEPPImage
}

// BuildEPPConfigMap constructs the ConfigMap containing EndpointPickerConfig
func BuildEPPConfigMap(inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role) *corev1.ConfigMap {
	configMapName := GenerateEPPConfigMapName(inferSvc.Name)
	configYAML := GenerateEPPConfig(inferSvc, role)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: inferSvc.Namespace,
			Labels: map[string]string{
				workload.LabelService: inferSvc.Name,
			},
		},
		Data: map[string]string{
			EPPConfigFileName: configYAML,
		},
	}
}

// BuildEPPDeployment constructs the EPP Deployment
func BuildEPPDeployment(inferSvc *fusioninferiov1alpha1.InferenceService) *appsv1.Deployment {
	deploymentName := GenerateEPPDeploymentName(inferSvc.Name)
	configMapName := GenerateEPPConfigMapName(inferSvc.Name)
	poolName := GeneratePoolName(inferSvc.Name)

	labels := map[string]string{
		"app":                 deploymentName,
		workload.LabelService: inferSvc.Name,
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: inferSvc.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: deploymentName,
					Containers: []corev1.Container{
						{
							Name:  "epp",
							Image: GetEPPImage(),
							Args: []string{
								"--pool-name=" + poolName,
								"--pool-namespace=" + inferSvc.Namespace,
								"--config-file=" + EPPConfigPath + "/" + EPPConfigFileName,
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "grpc",
									ContainerPort: EPPGRPCPort,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "grpc-health",
									ContainerPort: EPPHealthPort,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "metrics",
									ContainerPort: EPPMetricsPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									GRPC: &corev1.GRPCAction{
										Port:    EPPHealthPort,
										Service: ptr.To("inference-extension"),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									GRPC: &corev1.GRPCAction{
										Port:    EPPHealthPort,
										Service: ptr.To("inference-extension"),
									},
								},
								PeriodSeconds: 2,
							},
							Env: []corev1.EnvVar{
								{
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "plugins-config-volume",
									MountPath: EPPConfigPath,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "plugins-config-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// BuildEPPService constructs the EPP Service
func BuildEPPService(inferSvc *fusioninferiov1alpha1.InferenceService) *corev1.Service {
	serviceName := GenerateEPPServiceName(inferSvc.Name)
	deploymentName := GenerateEPPDeploymentName(inferSvc.Name)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: inferSvc.Namespace,
			Labels: map[string]string{
				workload.LabelService: inferSvc.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": deploymentName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc-ext-proc",
					Port:       EPPGRPCPort,
					TargetPort: intstr.FromInt(EPPGRPCPort),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "grpc-health",
					Port:       EPPHealthPort,
					TargetPort: intstr.FromInt(EPPHealthPort),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "http-metrics",
					Port:       EPPMetricsPort,
					TargetPort: intstr.FromInt(EPPMetricsPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// BuildEPPServiceAccount constructs the ServiceAccount for EPP
func BuildEPPServiceAccount(inferSvc *fusioninferiov1alpha1.InferenceService) *corev1.ServiceAccount {
	saName := GenerateEPPDeploymentName(inferSvc.Name)

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: inferSvc.Namespace,
			Labels: map[string]string{
				workload.LabelService: inferSvc.Name,
			},
		},
	}
}
