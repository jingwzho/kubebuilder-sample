/*

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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	samplecontrollerv1alpha1 "github.com/govargo/sample-controller-kubebuilder/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// MachineReconciler reconciles a Machine object
type MachineReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=samplecontroller.k8s.io,resources=machines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=samplecontroller.k8s.io,resources=machines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *MachineReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	openStackMachine := &samplecontrollerv1alpha1.Machine{}

	err := r.Client.Get(ctx, req.NamespacedName, openStackMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deleted machines
	if !openStackMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, openStackMachine)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, openStackMachine)
}

func (r *MachineReconciler) reconcileDelete(req ctrl.Request, openStackMachine *samplecontrollerv1alpha1.Machine) (ctrl.Result, error) {
	instanceStatus, err := computeService.GetInstanceStatusByName(openStackMachine, openStackMachine.Spec.Name)
	logger := r.Log.WithValues("machine", req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}
	if instanceStatus == nil {
		logger.Info("Skipped deleting machine that is already deleted")
	} else {
		if err := computeService.DeleteInstance(openStackMachine, instanceStatus); err != nil {
			handleUpdateMachineError(logger, openStackMachine, errors.Errorf("error deleting OpenStack instance %s with ID %s: %v", instanceStatus.Name(), instanceStatus.ID(), err))
			return ctrl.Result{}, nil
		}
	}

	controllerutil.RemoveFinalizer(openStackMachine, samplecontrollerv1alpha1.MachineFinalizer)
	logger.Info("Reconciled Machine delete successfully")
	return ctrl.Result{}, nil
}

func (r *MachineReconciler) reconcileNormal(req ctrl.Request, openStackMachine *samplecontrollerv1alpha1.Machine) (ctrl.Result, error) {

	controllerutil.AddFinalizer(openStackMachine, samplecontrollerv1alpha1.MachineFinalizer)

	instanceStatus, err := computeService.GetInstanceStatusByName(openStackMachine, openStackMachine.Spec.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if instanceStatus == nil {
		logger.Info("Machine not exist, Creating Machine", "Machine", openStackMachine.Spec.Name)
		instanceStatus, err = computeService.CreateInstance(openStackCluster, machine, openStackMachine, cluster.Name, userData)
		if err != nil {
			return ctrl.Result{}, errors.Errorf("error creating Openstack instance: %v", err)
		}
	}

	return ctrl.Result{}, nil
}

var (
	deploymentOwnerKey = ".metadata.controller"
	apiGVStr           = samplecontrollerv1alpha1.GroupVersion.String()
)

// setup with controller manager
func (r *MachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// define to watch targets...Machine resource and owned Deployment
	return ctrl.NewControllerManagedBy(mgr).
		For(&samplecontrollerv1alpha1.Machine{}).
		Complete(r)
}
