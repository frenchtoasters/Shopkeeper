/*
Copyright 2024.

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
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	interactionsv1alpha1 "frenchtoasters.io/shopkeeper/api/v1alpha1"
)

// TaskReconciler reconciles a Task object
type TaskReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ReconcileTimeout time.Duration
}

//+kubebuilder:rbac:groups=interactions.frenchtoasters.io,resources=tasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=interactions.frenchtoasters.io,resources=tasks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=interactions.frenchtoasters.io,resources=tasks/finalizers,verbs=update

func (r *TaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	ctx, cancel := context.WithTimeout(ctx, r.ReconcileTimeout)
	defer cancel()

	log := log.FromContext(ctx)
	shopkeeperTask := &interactionsv1alpha1.Task{}
	err := r.Get(ctx, req.NamespacedName, shopkeeperTask)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ShopkeeperTask resource not found or already deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Unable to fetch ShopkeeperTask resource")
		return ctrl.Result{}, err
	}

	task, err := GetTaskByName(ctx, r.Client, shopkeeperTask.ObjectMeta.Namespace, shopkeeperTask.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ShopkeeperTask resource not found or already deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get keycloak")
		return ctrl.Result{}, err
	}

	if IsPaused(task, shopkeeperTask) {
		log.Info("ShopkeeperTask of linked Shopkeeper is marked as paused. Won't reconcile.")
		return ctrl.Result{}, nil
	}

	if !shopkeeperTask.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, task)
	}

	return r.reconcile(ctx, task)
}

func (r *TaskReconciler) reconcile(ctx context.Context, taskScope *interactionsv1alpha1.Task) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling ShopkeeperTask")

	taskScope.Status.Ready = false
	taskScope.Status.FailureMessage = Pointer("")

	helper, err := patch.NewHelper(taskScope, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	controllerutil.AddFinalizer(taskScope, interactionsv1alpha1.ShopkeeperTaskFinalizer)
	if err := (*patch.Helper).Patch(helper, context.TODO(), taskScope); err != nil {
		return ctrl.Result{}, err
	}

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	jobConfig := TransformJob(taskScope)

	job, err := clientset.BatchV1().Jobs(taskScope.Namespace).Apply(ctx, &jobConfig, v1.ApplyOptions{})
	if err != nil {
		taskScope.Status.FailureMessage = err
		if err := (*patch.Helper).Patch(helper, context.TODO(), taskScope); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	taskScope.Status.JobId = job.GetSelfLink()
	taskScope.Status.Ready = true
	if err := (*patch.Helper).Patch(helper, context.TODO(), taskScope); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *TaskReconciler) reconcileDelete(ctx context.Context, taskScope *interactionsv1alpha1.Task) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling Delete ShopkeeperTask")

	if err := r.Client.Delete(ctx, taskScope); err != nil {
		log.Error(err, "delete error")
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(taskScope, interactionsv1alpha1.ShopkeeperTaskFinalizer)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&interactionsv1alpha1.Task{}).
		Complete(r)
}
