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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	inferenceapi "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"
	schedulingv1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"

	fusioninferiov1alpha1 "github.com/fusioninfer/fusioninfer/api/v1alpha1"
	"github.com/fusioninfer/fusioninfer/pkg/router"
	"github.com/fusioninfer/fusioninfer/pkg/scheduling"
	"github.com/fusioninfer/fusioninfer/pkg/workload"
)

// InferenceServiceReconciler reconciles a InferenceService object
type InferenceServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=fusioninfer.io,resources=inferenceservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=fusioninfer.io,resources=inferenceservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=fusioninfer.io,resources=inferenceservices/finalizers,verbs=update
// +kubebuilder:rbac:groups=leaderworkerset.x-k8s.io,resources=leaderworkersets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.volcano.sh,resources=podgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps;services;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=inference.networking.x-k8s.io,resources=inferencepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Sets Processing at start, Failed on error, Active at end
func (r *InferenceServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch InferenceService
	inferSvc := &fusioninferiov1alpha1.InferenceService{}
	if err := r.Get(ctx, req.NamespacedName, inferSvc); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("InferenceService not found, ignoring")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Reconciling InferenceService", "name", inferSvc.Name)

	// 2. Set Init condition (first time only)
	if len(inferSvc.Status.Conditions) == 0 {
		if err := r.setInferenceServiceInitCondition(ctx, inferSvc); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 3. Set Processing condition
	if err := r.setInferenceServiceProcessingCondition(ctx, inferSvc); err != nil {
		return ctrl.Result{}, err
	}

	// 4. Reconcile PodGroup for gang scheduling (must be created before LWS)
	if err := r.reconcilePodGroup(ctx, inferSvc); err != nil {
		log.Error(err, "Failed to reconcile PodGroup")
		r.setInferenceServiceFailedCondition(ctx, inferSvc, err)
		return ctrl.Result{}, err
	}

	// 5. Collect worker roles for router
	var workerRoles []fusioninferiov1alpha1.Role

	// 6. Reconcile workloads (LWS) for each role
	for _, role := range inferSvc.Spec.Roles {
		switch role.ComponentType {
		case fusioninferiov1alpha1.ComponentTypeWorker,
			fusioninferiov1alpha1.ComponentTypePrefiller,
			fusioninferiov1alpha1.ComponentTypeDecoder:
			workerRoles = append(workerRoles, role)
			if err := r.reconcileLWS(ctx, inferSvc, role); err != nil {
				log.Error(err, "Failed to reconcile LWS", "role", role.Name)
				r.setInferenceServiceFailedCondition(ctx, inferSvc, err)
				return ctrl.Result{}, err
			}

		case fusioninferiov1alpha1.ComponentTypeRouter:
			// Router will be reconciled after worker roles are collected
		}
	}

	// 7. Reconcile router components
	for _, role := range inferSvc.Spec.Roles {
		if role.ComponentType == fusioninferiov1alpha1.ComponentTypeRouter {
			if err := r.reconcileRouter(ctx, inferSvc, role, workerRoles); err != nil {
				log.Error(err, "Failed to reconcile Router", "role", role.Name)
				r.setInferenceServiceFailedCondition(ctx, inferSvc, err)
				return ctrl.Result{}, err
			}
		}
	}

	// 8. Update component status and check readiness
	if err := r.updateComponentStatus(ctx, inferSvc); err != nil {
		log.Error(err, "Failed to update component status")
		return ctrl.Result{}, err
	}

	// 9. Set Active condition if all components are ready
	allReady := r.checkAllComponentsReady(inferSvc)
	if allReady {
		if err := r.setInferenceServiceActiveCondition(ctx, inferSvc); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// reconcilePodGroup creates or updates a single PodGroup for the InferenceService
// The PodGroup uses minTaskMember with keys in format {roleName}-{replicaIndex}
// This ensures both cross-role scheduling (PD) and intra-replica atomic scheduling (multi-node)
func (r *InferenceServiceReconciler) reconcilePodGroup(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService) error {
	log := logf.FromContext(ctx)

	// Only create PodGroup if gang scheduling is needed
	if !scheduling.NeedsGangScheduling(inferSvc) {
		return nil
	}

	pg := scheduling.BuildPodGroup(inferSvc)

	// Set owner reference
	if err := controllerutil.SetControllerReference(inferSvc, pg, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference on PodGroup: %w", err)
	}

	// Create or update
	existingPG := &schedulingv1beta1.PodGroup{}
	err := r.Get(ctx, types.NamespacedName{Name: pg.Name, Namespace: pg.Namespace}, existingPG)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Creating PodGroup", "name", pg.Name)
			return r.Create(ctx, pg)
		}
		return err
	}

	// Update only if revision changed
	if existingPG.Labels[workload.LabelRevision] != pg.Labels[workload.LabelRevision] {
		existingPG.Labels = pg.Labels
		existingPG.Spec = pg.Spec
		log.V(1).Info("Updating PodGroup", "name", pg.Name)
		return r.Update(ctx, existingPG)
	}
	return nil
}

// reconcileLWS creates or updates the LeaderWorkerSet(s) for a role
// Creates one LWS per replica to support fine-grained gang scheduling
// Also cleans up orphan LWS when replicas are scaled down
func (r *InferenceServiceReconciler) reconcileLWS(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role) error {
	log := logf.FromContext(ctx)

	replicas := scheduling.GetReplicaCount(role)
	needsGangScheduling := scheduling.NeedsGangSchedulingForRole(inferSvc, role)

	// Track desired LWS names for cleanup
	desiredLWSNames := make(map[string]bool)

	// Create one LWS per replica to enable fine-grained gang scheduling
	for i := int32(0); i < replicas; i++ {
		// Prepare LWS config
		config := workload.LWSConfig{
			NeedsGangScheduling: needsGangScheduling,
			ReplicaIndex:        &i,
		}

		// Set PodGroup name and task name for gang scheduling
		if needsGangScheduling {
			config.PodGroupName = scheduling.GeneratePodGroupName(inferSvc.Name)
			// Task name format: {roleName}-{replicaIndex} to match minTaskMember keys
			config.TaskName = scheduling.GenerateTaskName(role.Name, i)
		}

		// Build LWS
		lws := workload.BuildLWS(inferSvc, role, config)
		desiredLWSNames[lws.Name] = true

		// Set owner reference
		if err := controllerutil.SetControllerReference(inferSvc, lws, r.Scheme); err != nil {
			return fmt.Errorf("failed to set owner reference on LWS: %w", err)
		}

		// Create or update
		existingLWS := &lwsv1.LeaderWorkerSet{}
		err := r.Get(ctx, types.NamespacedName{Name: lws.Name, Namespace: lws.Namespace}, existingLWS)
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Creating LWS", "name", lws.Name, "role", role.Name, "replica", i)
				if err := r.Create(ctx, lws); err != nil {
					return err
				}
				continue
			}
			return err
		}

		// Update only if revision changed
		if existingLWS.Labels[workload.LabelRevision] != lws.Labels[workload.LabelRevision] {
			existingLWS.Labels = lws.Labels
			existingLWS.Spec = lws.Spec
			log.V(1).Info("Updating LWS", "name", lws.Name, "role", role.Name, "replica", i)
			if err := r.Update(ctx, existingLWS); err != nil {
				return err
			}
		}
	}

	// Cleanup orphan LWS (when replicas are scaled down)
	if err := r.cleanupOrphanLWS(ctx, inferSvc, role, desiredLWSNames); err != nil {
		log.Error(err, "Failed to cleanup orphan LWS", "role", role.Name)
		return err
	}

	return nil
}

// cleanupOrphanLWS deletes LWS that are no longer needed (e.g., when replicas scaled down)
func (r *InferenceServiceReconciler) cleanupOrphanLWS(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role, desiredLWSNames map[string]bool) error {
	log := logf.FromContext(ctx)

	// List all LWS for this role
	lwsList := &lwsv1.LeaderWorkerSetList{}
	listOpts := []client.ListOption{
		client.InNamespace(inferSvc.Namespace),
		client.MatchingLabels{
			workload.LabelService:  inferSvc.Name,
			workload.LabelRoleName: role.Name,
		},
	}

	if err := r.List(ctx, lwsList, listOpts...); err != nil {
		return fmt.Errorf("failed to list LWS for cleanup: %w", err)
	}

	// Delete orphan LWS
	for _, existingLWS := range lwsList.Items {
		if !desiredLWSNames[existingLWS.Name] {
			log.Info("Deleting orphan LWS", "name", existingLWS.Name, "role", role.Name)
			if err := r.Delete(ctx, &existingLWS); err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("failed to delete orphan LWS %s: %w", existingLWS.Name, err)
				}
			}
		}
	}

	return nil
}

// reconcileRouter creates or updates all router-related resources
func (r *InferenceServiceReconciler) reconcileRouter(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role, workerRoles []fusioninferiov1alpha1.Role) error {
	log := logf.FromContext(ctx)

	// 1. Create ServiceAccount for EPP
	if err := r.reconcileEPPServiceAccount(ctx, inferSvc); err != nil {
		return fmt.Errorf("failed to reconcile EPP ServiceAccount: %w", err)
	}

	// 2. Create EPP ConfigMap
	if err := r.reconcileEPPConfigMap(ctx, inferSvc, role); err != nil {
		return fmt.Errorf("failed to reconcile EPP ConfigMap: %w", err)
	}

	// 3. Create EPP Deployment
	if err := r.reconcileEPPDeployment(ctx, inferSvc); err != nil {
		return fmt.Errorf("failed to reconcile EPP Deployment: %w", err)
	}

	// 4. Create EPP Service
	if err := r.reconcileEPPService(ctx, inferSvc); err != nil {
		return fmt.Errorf("failed to reconcile EPP Service: %w", err)
	}

	// 5. Create InferencePool
	if err := r.reconcileInferencePool(ctx, inferSvc, workerRoles); err != nil {
		return fmt.Errorf("failed to reconcile InferencePool: %w", err)
	}

	// 6. Create HTTPRoute
	if err := r.reconcileHTTPRoute(ctx, inferSvc, role); err != nil {
		return fmt.Errorf("failed to reconcile HTTPRoute: %w", err)
	}

	log.Info("Router resources reconciled", "role", role.Name)
	return nil
}

func (r *InferenceServiceReconciler) reconcileEPPServiceAccount(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService) error {
	sa := router.BuildEPPServiceAccount(inferSvc)

	if err := controllerutil.SetControllerReference(inferSvc, sa, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.ServiceAccount{}
	err := r.Get(ctx, types.NamespacedName{Name: sa.Name, Namespace: sa.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, sa)
		}
		return err
	}
	return nil // ServiceAccount doesn't need updates
}

func (r *InferenceServiceReconciler) reconcileEPPConfigMap(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role) error {
	cm := router.BuildEPPConfigMap(inferSvc, role)

	if err := controllerutil.SetControllerReference(inferSvc, cm, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, cm)
		}
		return err
	}

	// Update only if revision changed
	if existing.Labels[workload.LabelRevision] != cm.Labels[workload.LabelRevision] {
		existing.Labels = cm.Labels
		existing.Data = cm.Data
		return r.Update(ctx, existing)
	}
	return nil
}

func (r *InferenceServiceReconciler) reconcileEPPDeployment(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService) error {
	deploy := router.BuildEPPDeployment(inferSvc)

	if err := controllerutil.SetControllerReference(inferSvc, deploy, r.Scheme); err != nil {
		return err
	}

	existing := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, deploy)
		}
		return err
	}

	// Update only if revision changed
	if existing.Labels[workload.LabelRevision] != deploy.Labels[workload.LabelRevision] {
		existing.Labels = deploy.Labels
		existing.Spec = deploy.Spec
		return r.Update(ctx, existing)
	}
	return nil
}

func (r *InferenceServiceReconciler) reconcileEPPService(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService) error {
	svc := router.BuildEPPService(inferSvc)

	if err := controllerutil.SetControllerReference(inferSvc, svc, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, svc)
		}
		return err
	}

	// Update only if revision changed
	if existing.Labels[workload.LabelRevision] != svc.Labels[workload.LabelRevision] {
		existing.Labels = svc.Labels
		existing.Spec.Ports = svc.Spec.Ports
		existing.Spec.Selector = svc.Spec.Selector
		return r.Update(ctx, existing)
	}
	return nil
}

func (r *InferenceServiceReconciler) reconcileInferencePool(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService, workerRoles []fusioninferiov1alpha1.Role) error {
	pool := router.BuildInferencePool(inferSvc, workerRoles)

	if err := controllerutil.SetControllerReference(inferSvc, pool, r.Scheme); err != nil {
		return err
	}

	existing := &inferenceapi.InferencePool{}
	err := r.Get(ctx, types.NamespacedName{Name: pool.Name, Namespace: pool.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, pool)
		}
		return err
	}

	// Update only if revision changed
	if existing.Labels[workload.LabelRevision] != pool.Labels[workload.LabelRevision] {
		existing.Labels = pool.Labels
		existing.Spec = pool.Spec
		return r.Update(ctx, existing)
	}
	return nil
}

func (r *InferenceServiceReconciler) reconcileHTTPRoute(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role) error {
	httpRoute := router.BuildHTTPRoute(inferSvc, role)

	if err := controllerutil.SetControllerReference(inferSvc, httpRoute, r.Scheme); err != nil {
		return err
	}

	existing := &gatewayv1.HTTPRoute{}
	err := r.Get(ctx, types.NamespacedName{Name: httpRoute.Name, Namespace: httpRoute.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, httpRoute)
		}
		return err
	}

	// Update only if revision changed
	if existing.Labels[workload.LabelRevision] != httpRoute.Labels[workload.LabelRevision] {
		existing.Labels = httpRoute.Labels
		existing.Spec = httpRoute.Spec
		return r.Update(ctx, existing)
	}
	return nil
}

// updateComponentStatus updates the InferenceService component status based on LWS states
// Note: This only updates component status, not conditions (conditions are updated via setXxxCondition functions)
func (r *InferenceServiceReconciler) updateComponentStatus(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService) error {
	log := logf.FromContext(ctx)

	components := make(map[string]fusioninferiov1alpha1.ComponentStatus)

	for _, role := range inferSvc.Spec.Roles {
		// Skip router roles for now
		if role.ComponentType == fusioninferiov1alpha1.ComponentTypeRouter {
			continue
		}

		nodesPerReplica := scheduling.GetNodeCount(role)
		desiredReplicas := scheduling.GetReplicaCount(role)

		// Aggregate status from per-replica LWS instances
		status := r.aggregateLWSStatus(ctx, inferSvc, role)

		status.NodesPerReplica = nodesPerReplica
		status.DesiredReplicas = desiredReplicas
		status.TotalPods = desiredReplicas * nodesPerReplica
		status.LastUpdateTime = &metav1.Time{Time: metav1.Now().Time}

		components[role.Name] = status
	}

	inferSvc.Status.Components = components

	log.V(1).Info("Updating InferenceService component status", "components", len(components))
	return r.Status().Update(ctx, inferSvc)
}

// checkAllComponentsReady checks if all components are in Running phase
func (r *InferenceServiceReconciler) checkAllComponentsReady(inferSvc *fusioninferiov1alpha1.InferenceService) bool {
	if len(inferSvc.Status.Components) == 0 {
		return false
	}

	for _, status := range inferSvc.Status.Components {
		if status.Phase != fusioninferiov1alpha1.ComponentPhaseRunning {
			return false
		}
	}
	return true
}

// aggregateLWSStatus aggregates status from all per-replica LWS instances for a role
func (r *InferenceServiceReconciler) aggregateLWSStatus(ctx context.Context, inferSvc *fusioninferiov1alpha1.InferenceService, role fusioninferiov1alpha1.Role) fusioninferiov1alpha1.ComponentStatus {
	desiredReplicas := scheduling.GetReplicaCount(role)
	nodesPerReplica := scheduling.GetNodeCount(role)

	var readyReplicas int32
	var totalReadyPods int32
	allPending := true
	anyRunning := false

	for i := int32(0); i < desiredReplicas; i++ {
		lwsName := workload.GenerateLWSNameWithIndex(inferSvc.Name, role.Name, &i)
		lws := &lwsv1.LeaderWorkerSet{}
		err := r.Get(ctx, types.NamespacedName{Name: lwsName, Namespace: inferSvc.Namespace}, lws)
		if err != nil {
			continue
		}

		// Each per-replica LWS has replicas=1
		if lws.Status.ReadyReplicas >= 1 {
			readyReplicas++
			anyRunning = true
		}
		if lws.Status.Replicas > 0 {
			allPending = false
		}
		totalReadyPods += lws.Status.ReadyReplicas * nodesPerReplica
	}

	// Determine phase
	var phase fusioninferiov1alpha1.ComponentPhase
	if readyReplicas >= desiredReplicas {
		phase = fusioninferiov1alpha1.ComponentPhaseRunning
	} else if anyRunning || !allPending {
		phase = fusioninferiov1alpha1.ComponentPhaseDeploying
	} else {
		phase = fusioninferiov1alpha1.ComponentPhasePending
	}

	return fusioninferiov1alpha1.ComponentStatus{
		ReadyReplicas: readyReplicas,
		ReadyPods:     totalReadyPods,
		Phase:         phase,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *InferenceServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fusioninferiov1alpha1.InferenceService{}).
		Owns(&lwsv1.LeaderWorkerSet{}).
		Owns(&schedulingv1beta1.PodGroup{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&appsv1.Deployment{}).
		Owns(&inferenceapi.InferencePool{}).
		Owns(&gatewayv1.HTTPRoute{}).
		Named("inferenceservice").
		Complete(r)
}
