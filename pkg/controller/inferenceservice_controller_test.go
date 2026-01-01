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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/core/v1alpha1"
	"github.com/fusioninfer/fusioninfer/pkg/workload"
)

// createTemplateRaw creates a runtime.RawExtension from PodTemplateSpec
func createTemplateRaw(template corev1.PodTemplateSpec) *runtime.RawExtension {
	data, err := json.Marshal(template)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal template: %v", err))
	}
	return &runtime.RawExtension{Raw: data}
}

// createTestInferenceService creates a basic InferenceService for testing in the default namespace.
func createTestInferenceService(name string) *fusioninferiov1alpha1.InferenceService {
	template := corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "vllm",
					Image: "vllm/vllm-openai:v0.13.0",
					Args:  []string{"--model", "Qwen/Qwen3-8B"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"nvidia.com/gpu": resource.MustParse("1"),
						},
					},
				},
			},
		},
	}

	return &fusioninferiov1alpha1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: fusioninferiov1alpha1.InferenceServiceSpec{
			Roles: []fusioninferiov1alpha1.Role{
				{
					Name:          "inference",
					ComponentType: fusioninferiov1alpha1.ComponentTypeWorker,
					Replicas:      ptr.To(int32(1)),
					Template:      createTemplateRaw(template),
				},
			},
		},
	}
}

var _ = Describe("InferenceService Controller", func() {
	const (
		timeout  = time.Second * 30
		interval = time.Millisecond * 250
	)

	// ==========================================================================
	// Basic Reconciliation Tests
	// ==========================================================================
	Context("Basic Reconciliation", func() {
		const resourceName = "test-basic-reconcile"
		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{Name: resourceName, Namespace: "default"}

		AfterEach(func() {
			resource := &fusioninferiov1alpha1.InferenceService{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				By("Cleanup InferenceService")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should create LWS when InferenceService is created", func() {
			By("Creating InferenceService")
			inferSvc := createTestInferenceService(resourceName)
			Expect(k8sClient.Create(ctx, inferSvc)).To(Succeed())

			By("Waiting for LWS to be created")
			lwsKey := types.NamespacedName{Name: resourceName + "-inference-0", Namespace: "default"}
			Eventually(func() error {
				return k8sClient.Get(ctx, lwsKey, &lwsv1.LeaderWorkerSet{})
			}, timeout, interval).Should(Succeed())
		})
	})

	// ==========================================================================
	// Spec Hash Update Tests - Verifies reconciliation triggers on spec changes
	// ==========================================================================
	Context("Spec Hash Updates", func() {
		ctx := context.Background()

		// Test 1: Verify hash label exists and changes when replicas change
		It("should create new LWS when replicas increase", func() {
			inferSvcName := "test-replicas-change"
			typeNamespacedName := types.NamespacedName{Name: inferSvcName, Namespace: "default"}

			By("Creating InferenceService with 1 replica")
			inferSvc := createTestInferenceService(inferSvcName)
			Expect(k8sClient.Create(ctx, inferSvc)).Should(Succeed())

			By("Waiting for first LWS to be created")
			lwsKey := types.NamespacedName{Name: inferSvcName + "-inference-0", Namespace: "default"}
			createdLWS := &lwsv1.LeaderWorkerSet{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, lwsKey, createdLWS) == nil
			}, timeout, interval).Should(BeTrue())

			By("Verifying LWS has spec-hash label")
			Expect(createdLWS.Labels[workload.LabelSpecHash]).NotTo(BeEmpty())

			By("Increasing replicas to 2")
			Eventually(func() error {
				if err := k8sClient.Get(ctx, typeNamespacedName, inferSvc); err != nil {
					return err
				}
				inferSvc.Spec.Roles[0].Replicas = ptr.To(int32(2))
				return k8sClient.Update(ctx, inferSvc)
			}, timeout, interval).Should(Succeed())

			By("Waiting for second LWS to be created")
			newLWSKey := types.NamespacedName{Name: inferSvcName + "-inference-1", Namespace: "default"}
			Eventually(func() bool {
				return k8sClient.Get(ctx, newLWSKey, &lwsv1.LeaderWorkerSet{}) == nil
			}, timeout, interval).Should(BeTrue())

			By("Cleanup")
			Expect(k8sClient.Delete(ctx, inferSvc)).Should(Succeed())
		})

		// Test 2: Verify LWS is updated when container image changes
		It("should update LWS when container image changes", func() {
			inferSvcName := "test-image-change"
			typeNamespacedName := types.NamespacedName{Name: inferSvcName, Namespace: "default"}

			By("Creating InferenceService")
			inferSvc := createTestInferenceService(inferSvcName)
			Expect(k8sClient.Create(ctx, inferSvc)).Should(Succeed())

			By("Waiting for LWS to be created")
			lwsKey := types.NamespacedName{Name: inferSvcName + "-inference-0", Namespace: "default"}
			createdLWS := &lwsv1.LeaderWorkerSet{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, lwsKey, createdLWS) == nil
			}, timeout, interval).Should(BeTrue())

			initialSpecHash := createdLWS.Labels[workload.LabelSpecHash]
			Expect(initialSpecHash).NotTo(BeEmpty())

			By("Updating container image")
			Eventually(func() error {
				if err := k8sClient.Get(ctx, typeNamespacedName, inferSvc); err != nil {
					return err
				}
				// Create new template with updated image
				newTemplate := corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "vllm",
								Image: "vllm/vllm-openai:v0.14.0", // Changed
								Args:  []string{"--model", "Qwen/Qwen3-8B"},
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										"nvidia.com/gpu": resource.MustParse("1"),
									},
								},
							},
						},
					},
				}
				inferSvc.Spec.Roles[0].Template = createTemplateRaw(newTemplate)
				return k8sClient.Update(ctx, inferSvc)
			}, timeout, interval).Should(Succeed())

			By("Waiting for LWS spec-hash to change")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, lwsKey, createdLWS); err != nil {
					return false
				}
				return createdLWS.Labels[workload.LabelSpecHash] != initialSpecHash
			}, timeout, interval).Should(BeTrue(), "Spec-hash should change when image changes")

			By("Cleanup")
			Expect(k8sClient.Delete(ctx, inferSvc)).Should(Succeed())
		})

		// Test 3: Verify LWS is NOT updated when only metadata changes
		It("should NOT update LWS when only metadata changes", func() {
			inferSvcName := "test-no-spec-change"
			typeNamespacedName := types.NamespacedName{Name: inferSvcName, Namespace: "default"}

			By("Creating InferenceService")
			inferSvc := createTestInferenceService(inferSvcName)
			Expect(k8sClient.Create(ctx, inferSvc)).Should(Succeed())

			By("Waiting for LWS to be created")
			lwsKey := types.NamespacedName{Name: inferSvcName + "-inference-0", Namespace: "default"}
			createdLWS := &lwsv1.LeaderWorkerSet{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, lwsKey, createdLWS) == nil
			}, timeout, interval).Should(BeTrue())

			initialSpecHash := createdLWS.Labels[workload.LabelSpecHash]
			initialResourceVersion := createdLWS.ResourceVersion

			By("Updating only InferenceService annotations (not spec)")
			Eventually(func() error {
				if err := k8sClient.Get(ctx, typeNamespacedName, inferSvc); err != nil {
					return err
				}
				if inferSvc.Annotations == nil {
					inferSvc.Annotations = make(map[string]string)
				}
				inferSvc.Annotations["test-annotation"] = "test-value"
				return k8sClient.Update(ctx, inferSvc)
			}, timeout, interval).Should(Succeed())

			By("Waiting for potential reconciliation")
			time.Sleep(2 * time.Second)

			By("Verifying LWS was NOT updated")
			Expect(k8sClient.Get(ctx, lwsKey, createdLWS)).To(Succeed())
			Expect(createdLWS.ResourceVersion).To(Equal(initialResourceVersion),
				"LWS ResourceVersion should not change when spec doesn't change")
			Expect(createdLWS.Labels[workload.LabelSpecHash]).To(Equal(initialSpecHash),
				"Spec-hash should remain the same")

			By("Cleanup")
			Expect(k8sClient.Delete(ctx, inferSvc)).Should(Succeed())
		})
	})

	// ==========================================================================
	// Spec Change Propagation Tests - Verifies InferenceService changes propagate to LWS
	// ==========================================================================
	Context("Spec Change Propagation", func() {
		ctx := context.Background()

		// Verify that InferenceService spec changes trigger LWS update
		It("should update LWS when InferenceService spec changes", func() {
			inferSvcName := "test-spec-change-triggers-update"
			typeNamespacedName := types.NamespacedName{Name: inferSvcName, Namespace: "default"}

			By("Creating InferenceService")
			inferSvc := createTestInferenceService(inferSvcName)
			Expect(k8sClient.Create(ctx, inferSvc)).Should(Succeed())

			By("Waiting for LWS to be created")
			lwsKey := types.NamespacedName{Name: inferSvcName + "-inference-0", Namespace: "default"}
			createdLWS := &lwsv1.LeaderWorkerSet{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, lwsKey, createdLWS) == nil
			}, timeout, interval).Should(BeTrue())

			initialSpecHash := createdLWS.Labels[workload.LabelSpecHash]
			Expect(initialSpecHash).NotTo(BeEmpty())

			By("Modifying InferenceService spec (changing args)")
			Eventually(func() error {
				if err := k8sClient.Get(ctx, typeNamespacedName, inferSvc); err != nil {
					return err
				}
				// Create new template with different args
				newTemplate := corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "vllm",
								Image: "vllm/vllm-openai:v0.13.0",
								Args:  []string{"--model", "Qwen/Qwen3-8B", "--max-model-len", "4096"}, // Added arg
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										"nvidia.com/gpu": resource.MustParse("1"),
									},
								},
							},
						},
					},
				}
				inferSvc.Spec.Roles[0].Template = createTemplateRaw(newTemplate)
				return k8sClient.Update(ctx, inferSvc)
			}, timeout, interval).Should(Succeed())

			By("Waiting for controller to detect hash mismatch and update LWS")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, lwsKey, createdLWS); err != nil {
					return false
				}
				// Controller should detect hash mismatch (desired != existing label) and update
				return createdLWS.Labels[workload.LabelSpecHash] != initialSpecHash
			}, timeout, interval).Should(BeTrue(),
				"Controller should update LWS when InferenceService spec changes")

			By("Cleanup")
			Expect(k8sClient.Delete(ctx, inferSvc)).Should(Succeed())
		})
	})
})
