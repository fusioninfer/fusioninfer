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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	inferenceapi "sigs.k8s.io/gateway-api-inference-extension/api/v1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/core/v1alpha1"
	"github.com/fusioninfer/fusioninfer/pkg/util"
	"github.com/fusioninfer/fusioninfer/pkg/workload"
)

const (
	// Default target port for inference servers
	DefaultTargetPort = 8000
	// Default EPP gRPC port
	DefaultEPPPort = 9002
	// LWS worker index label - leader pod always has index "0"
	LWSWorkerIndexLabel = "leaderworkerset.sigs.k8s.io/worker-index"
)

// BuildInferencePool constructs an InferencePool that selects worker pods
func BuildInferencePool(
	inferSvc *fusioninferiov1alpha1.InferenceService,
	workerRoles []fusioninferiov1alpha1.Role,
) *inferenceapi.InferencePool {
	poolName := GeneratePoolName(inferSvc.Name)
	eppServiceName := GenerateEPPServiceName(inferSvc.Name)

	// Build selector to match worker pods
	selector := buildPoolSelector(inferSvc, workerRoles)

	pool := &inferenceapi.InferencePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      poolName,
			Namespace: inferSvc.Namespace,
			Labels: map[string]string{
				workload.LabelService: inferSvc.Name,
			},
		},
		Spec: inferenceapi.InferencePoolSpec{
			Selector: selector,
			TargetPorts: []inferenceapi.Port{
				{
					Number: inferenceapi.PortNumber(DefaultTargetPort),
				},
			},
			EndpointPickerRef: inferenceapi.EndpointPickerRef{
				Name: inferenceapi.ObjectName(eppServiceName),
				Port: &inferenceapi.Port{
					Number: inferenceapi.PortNumber(DefaultEPPPort),
				},
			},
		},
	}

	// Compute spec hash after the spec is fully built
	pool.Labels[workload.LabelSpecHash] = util.ComputeSpecHash(pool.Spec)

	return pool
}

// buildPoolSelector builds the label selector for the InferencePool
func buildPoolSelector(
	inferSvc *fusioninferiov1alpha1.InferenceService,
	workerRoles []fusioninferiov1alpha1.Role,
) inferenceapi.LabelSelector {
	// Select pods belonging to this InferenceService
	matchLabels := map[inferenceapi.LabelKey]inferenceapi.LabelValue{
		inferenceapi.LabelKey(workload.LabelService): inferenceapi.LabelValue(inferSvc.Name),
	}

	// If there's only one worker role, also match by component type
	if len(workerRoles) == 1 {
		compType := workerRoles[0].ComponentType
		matchLabels[inferenceapi.LabelKey(workload.LabelComponentType)] = inferenceapi.LabelValue(compType)
	}

	// Only select leader pods (worker-index=0) for routing
	// For single-node: the only pod has worker-index=0
	// For multi-node: only the leader pod (with vLLM server) has worker-index=0
	matchLabels[inferenceapi.LabelKey(LWSWorkerIndexLabel)] = "0"

	return inferenceapi.LabelSelector{
		MatchLabels: matchLabels,
	}
}

// GeneratePoolName generates the InferencePool name
func GeneratePoolName(inferSvcName string) string {
	return fmt.Sprintf("%s-pool", inferSvcName)
}

// GenerateEPPServiceName generates the EPP Service name
func GenerateEPPServiceName(inferSvcName string) string {
	return fmt.Sprintf("%s-epp", inferSvcName)
}

// GenerateEPPDeploymentName generates the EPP Deployment name
func GenerateEPPDeploymentName(inferSvcName string) string {
	return fmt.Sprintf("%s-epp", inferSvcName)
}

// GenerateEPPConfigMapName generates the EPP ConfigMap name
func GenerateEPPConfigMapName(inferSvcName string) string {
	return fmt.Sprintf("%s-epp-config", inferSvcName)
}

// GenerateHTTPRouteName generates the HTTPRoute name
func GenerateHTTPRouteName(inferSvcName string) string {
	return fmt.Sprintf("%s-httproute", inferSvcName)
}
