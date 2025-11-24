// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	metalv1 "github.com/siderolabs/sidero/app/sidero-controller-manager/api/v1alpha2"
	"github.com/siderolabs/sidero/app/sidero-controller-manager/pkg/constants"
	"github.com/siderolabs/sidero/app/sidero-controller-manager/pkg/managementapi"
)

const (
	adoptedServerFinalizer = "metal.sidero.dev/adopted-server"
	healthCheckInterval    = 30 * time.Second
	syncInterval           = 5 * time.Minute
)

// AdoptedServerReconciler reconciles an AdoptedServer object.
type AdoptedServerReconciler struct {
	client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	APIReader client.Reader
	Recorder  record.EventRecorder
}

// +kubebuilder:rbac:groups=metal.sidero.dev,resources=adoptedservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal.sidero.dev,resources=adoptedservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=metal.sidero.dev,resources=adoptedservers/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *AdoptedServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the AdoptedServer instance
	as := &metalv1.AdoptedServer{}
	if err := r.APIReader.Get(ctx, req.NamespacedName, as); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("AdoptedServer not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get AdoptedServer")
		return ctrl.Result{}, err
	}

	// Create patch helper
	patchHelper, err := patch.NewHelper(as, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Defer patch to ensure status is always updated
	defer func() {
		if err := patchHelper.Patch(ctx, as, patch.WithOwnedConditions{
			Conditions: []clusterv1.ConditionType{
				metalv1.ConditionConnected,
				metalv1.ConditionAdopted,
				metalv1.ConditionSideroLinkReady,
				metalv1.ConditionManagementAPISync,
			},
		}); err != nil {
			logger.Error(err, "Failed to patch AdoptedServer")
		}
	}()

	// Get object reference for events
	asRef, err := reference.GetReference(r.Scheme, as)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !as.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, as, asRef)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(as, adoptedServerFinalizer) {
		controllerutil.AddFinalizer(as, adoptedServerFinalizer)
		if err := patchHelper.Patch(ctx, as); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check if accepted
	if !as.Spec.Accepted {
		logger.Info("AdoptedServer not accepted, skipping reconciliation")
		as.Status.Ready = false
		conditions.MarkFalse(as, metalv1.ConditionAdopted, "NotAccepted", clusterv1.ConditionSeverityInfo, "Server not accepted for management")
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueAfter}, nil
	}

	// Reconcile the AdoptedServer
	return r.reconcileNormal(ctx, as, asRef)
}

func (r *AdoptedServerReconciler) reconcileNormal(ctx context.Context, as *metalv1.AdoptedServer, asRef *corev1.ObjectReference) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling AdoptedServer")

	// Mark as adopted
	conditions.MarkTrue(as, metalv1.ConditionAdopted)

	// Step 1: Check Talos API connectivity
	connected, err := r.checkTalosConnectivity(ctx, as)
	if err != nil {
		logger.Error(err, "Failed to check Talos connectivity")
		conditions.MarkFalse(as, metalv1.ConditionConnected, "ConnectivityCheckFailed", clusterv1.ConditionSeverityError, err.Error())
		as.Status.Connected = false
		as.Status.Ready = false
		r.Recorder.Event(asRef, corev1.EventTypeWarning, "ConnectivityCheckFailed", fmt.Sprintf("Failed to check Talos API connectivity: %s", err.Error()))
		return ctrl.Result{RequeueAfter: healthCheckInterval}, nil
	}

	if !connected {
		logger.Info("Talos API not reachable")
		conditions.MarkFalse(as, metalv1.ConditionConnected, "Unreachable", clusterv1.ConditionSeverityWarning, "Talos API endpoint is not reachable")
		as.Status.Connected = false
		as.Status.Ready = false
		r.Recorder.Event(asRef, corev1.EventTypeWarning, "Unreachable", "Talos API endpoint is not reachable")
		return ctrl.Result{RequeueAfter: healthCheckInterval}, nil
	}

	// Connection successful
	logger.Info("Talos API is reachable")
	conditions.MarkTrue(as, metalv1.ConditionConnected)
	as.Status.Connected = true
	now := metav1.Now()
	as.Status.LastContactTime = &now

	// Step 2: Gather node information
	if err := r.gatherNodeInfo(ctx, as); err != nil {
		logger.Error(err, "Failed to gather node information")
		// Don't fail reconciliation, just log the error
	}

	// Step 3: Perform health check
	if err := r.performHealthCheck(ctx, as); err != nil {
		logger.Error(err, "Failed to perform health check")
		// Don't fail reconciliation, just log the error
	}

	// Step 4: Setup SideroLink if enabled
	if as.Spec.SideroLink != nil && as.Spec.SideroLink.Enabled {
		if err := r.setupSideroLink(ctx, as); err != nil {
			logger.Error(err, "Failed to setup SideroLink")
			conditions.MarkFalse(as, metalv1.ConditionSideroLinkReady, "SetupFailed", clusterv1.ConditionSeverityWarning, err.Error())
			r.Recorder.Event(asRef, corev1.EventTypeWarning, "SideroLinkSetupFailed", fmt.Sprintf("Failed to setup SideroLink: %s", err.Error()))
		} else {
			conditions.MarkTrue(as, metalv1.ConditionSideroLinkReady)
		}
	} else {
		conditions.Delete(as, metalv1.ConditionSideroLinkReady)
	}

	// Step 5: Sync with Management API if enabled
	if as.Spec.ManagementAPI != nil && as.Spec.ManagementAPI.Enabled {
		if err := r.syncWithManagementAPI(ctx, as); err != nil {
			logger.Error(err, "Failed to sync with Management API")
			conditions.MarkFalse(as, metalv1.ConditionManagementAPISync, "SyncFailed", clusterv1.ConditionSeverityWarning, err.Error())
			if as.Status.ManagementAPIStatus == nil {
				as.Status.ManagementAPIStatus = &metalv1.ManagementAPIStatus{}
			}
			as.Status.ManagementAPIStatus.Synced = false
			as.Status.ManagementAPIStatus.Error = err.Error()
			r.Recorder.Event(asRef, corev1.EventTypeWarning, "ManagementAPISyncFailed", fmt.Sprintf("Failed to sync with Management API: %s", err.Error()))
		} else {
			conditions.MarkTrue(as, metalv1.ConditionManagementAPISync)
			if as.Status.ManagementAPIStatus == nil {
				as.Status.ManagementAPIStatus = &metalv1.ManagementAPIStatus{}
			}
			as.Status.ManagementAPIStatus.Synced = true
			as.Status.ManagementAPIStatus.Error = ""
			syncTime := metav1.Now()
			as.Status.ManagementAPIStatus.LastSyncTime = &syncTime
		}
	} else {
		conditions.Delete(as, metalv1.ConditionManagementAPISync)
	}

	// Mark as ready if all critical checks pass
	as.Status.Ready = as.Status.Connected

	if as.Status.Ready {
		r.Recorder.Event(asRef, corev1.EventTypeNormal, "Ready", "AdoptedServer is ready and being monitored")
	}

	// Requeue for periodic health checks
	return ctrl.Result{RequeueAfter: healthCheckInterval}, nil
}

func (r *AdoptedServerReconciler) reconcileDelete(ctx context.Context, as *metalv1.AdoptedServer, asRef *corev1.ObjectReference) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Deleting AdoptedServer")

	if !controllerutil.ContainsFinalizer(as, adoptedServerFinalizer) {
		return ctrl.Result{}, nil
	}

	// Cleanup: unregister from Management API if registered
	if as.Spec.ManagementAPI != nil && as.Spec.ManagementAPI.Enabled {
		if err := r.unregisterFromManagementAPI(ctx, as); err != nil {
			logger.Error(err, "Failed to unregister from Management API")
			r.Recorder.Event(asRef, corev1.EventTypeWarning, "UnregisterFailed", fmt.Sprintf("Failed to unregister from Management API: %s", err.Error()))
			// Continue with deletion even if unregister fails
		}
	}

	// Cleanup: teardown SideroLink if configured
	if as.Spec.SideroLink != nil && as.Spec.SideroLink.Enabled {
		if err := r.teardownSideroLink(ctx, as); err != nil {
			logger.Error(err, "Failed to teardown SideroLink")
			r.Recorder.Event(asRef, corev1.EventTypeWarning, "SideroLinkTeardownFailed", fmt.Sprintf("Failed to teardown SideroLink: %s", err.Error()))
			// Continue with deletion even if teardown fails
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(as, adoptedServerFinalizer)
	if err := r.Client.Update(ctx, as); err != nil {
		return ctrl.Result{}, err
	}

	r.Recorder.Event(asRef, corev1.EventTypeNormal, "Deleted", "AdoptedServer deleted successfully")
	return ctrl.Result{}, nil
}

// checkTalosConnectivity checks if the Talos API endpoint is reachable.
// TODO: Implement actual talosctl connectivity check using talosctl or gRPC client
func (r *AdoptedServerReconciler) checkTalosConnectivity(ctx context.Context, as *metalv1.AdoptedServer) (bool, error) {
	logger := log.FromContext(ctx)
	logger.Info("Checking Talos connectivity", "endpoint", as.Spec.Talos.Endpoint)

	// Placeholder implementation
	// In a real implementation, this would:
	// 1. Use talosctl or gRPC client to connect to the endpoint
	// 2. Execute a simple command like `talosctl version` or gRPC health check
	// 3. Return true if successful, false otherwise

	// For now, assume connectivity is successful
	// TODO: Implement actual connectivity check
	return true, nil
}

// gatherNodeInfo collects information from the Talos node.
// TODO: Implement using talosctl or gRPC API
func (r *AdoptedServerReconciler) gatherNodeInfo(ctx context.Context, as *metalv1.AdoptedServer) error {
	logger := log.FromContext(ctx)
	logger.Info("Gathering node information", "endpoint", as.Spec.Talos.Endpoint)

	// Placeholder implementation
	// In a real implementation, this would:
	// 1. Query the node for system information
	// 2. Update as.Status.NodeInfo with the gathered data
	// 3. Update as.Status.Addresses with discovered IPs

	// For now, create basic node info from spec
	if as.Status.NodeInfo == nil {
		as.Status.NodeInfo = &metalv1.NodeInfo{
			OperatingSystem: "talos",
		}
	}

	// TODO: Implement actual node info gathering
	return nil
}

// performHealthCheck checks the health of the Talos node.
// TODO: Implement using talosctl health check or gRPC API
func (r *AdoptedServerReconciler) performHealthCheck(ctx context.Context, as *metalv1.AdoptedServer) error {
	logger := log.FromContext(ctx)
	logger.Info("Performing health check", "endpoint", as.Spec.Talos.Endpoint)

	// Placeholder implementation
	// In a real implementation, this would:
	// 1. Execute health checks against the node
	// 2. Check etcd health, kubelet status, etc.
	// 3. Update as.Status.Health with results

	if as.Status.Health == nil {
		as.Status.Health = &metalv1.HealthStatus{}
	}

	now := metav1.Now()
	as.Status.Health.Status = "healthy"
	as.Status.Health.LastCheckTime = &now
	as.Status.Health.Message = "All systems operational"

	// TODO: Implement actual health check
	return nil
}

// setupSideroLink configures SideroLink monitoring for the adopted server.
// TODO: Implement SideroLink setup
func (r *AdoptedServerReconciler) setupSideroLink(ctx context.Context, as *metalv1.AdoptedServer) error {
	logger := log.FromContext(ctx)
	logger.Info("Setting up SideroLink", "endpoint", as.Spec.Talos.Endpoint)

	// Placeholder implementation
	// In a real implementation, this would:
	// 1. Generate or retrieve SideroLink Wireguard keys
	// 2. Assign IPv6 address from the SideroLink pool
	// 3. Configure the Talos node with SideroLink settings
	// 4. Update as.Spec.SideroLink and as.Status.SideroLinkStatus

	// TODO: Implement actual SideroLink setup
	return nil
}

// teardownSideroLink removes SideroLink configuration.
func (r *AdoptedServerReconciler) teardownSideroLink(ctx context.Context, as *metalv1.AdoptedServer) error {
	logger := log.FromContext(ctx)
	logger.Info("Tearing down SideroLink", "endpoint", as.Spec.Talos.Endpoint)

	// Placeholder implementation
	// TODO: Implement actual SideroLink teardown
	return nil
}

// syncWithManagementAPI synchronizes the adopted server with the Management API.
func (r *AdoptedServerReconciler) syncWithManagementAPI(ctx context.Context, as *metalv1.AdoptedServer) error {
	logger := log.FromContext(ctx)
	logger.Info("Syncing with Management API", "endpoint", as.Spec.ManagementAPI.Endpoint)

	// Create Management API client
	client, err := managementapi.NewClient(as.Spec.ManagementAPI.Endpoint)
	if err != nil {
		return errors.Wrap(err, "failed to create Management API client")
	}
	defer client.Close()

	// Test connectivity with health check
	if _, err := client.HealthCheck(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to Management API")
	}

	// Create sync service and sync the adopted server
	syncService := managementapi.NewSyncService(client, logger)
	if err := syncService.SyncAdoptedServer(ctx, as); err != nil {
		return errors.Wrap(err, "failed to sync adopted server with Management API")
	}

	return nil
}

// unregisterFromManagementAPI removes the server from the Management API.
func (r *AdoptedServerReconciler) unregisterFromManagementAPI(ctx context.Context, as *metalv1.AdoptedServer) error {
	logger := log.FromContext(ctx)
	logger.Info("Unregistering from Management API", "endpoint", as.Spec.ManagementAPI.Endpoint)

	// Create Management API client
	client, err := managementapi.NewClient(as.Spec.ManagementAPI.Endpoint)
	if err != nil {
		logger.Error(err, "Failed to create Management API client during unregister")
		return err
	}
	defer client.Close()

	// Create sync service and unregister the adopted server
	syncService := managementapi.NewSyncService(client, logger)
	if err := syncService.UnregisterAdoptedServer(ctx, as); err != nil {
		logger.Error(err, "Failed to unregister adopted server from Management API")
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AdoptedServerReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&metalv1.AdoptedServer{}).
		Complete(r)
}
