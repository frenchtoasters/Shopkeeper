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

	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	interactionsv1alpha1 "frenchtoasters.io/shopkeeper/api/v1alpha1"
	"frenchtoasters.io/shopkeeper/scope"
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
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *TaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {

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

	shopkeeperScope, err := scope.NewShopkeeperScope(ctx, scope.ShopkeeperScopeParams{
		Client: r.Client,
		Task:   task,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	defer func() {
		if err := shopkeeperScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !shopkeeperTask.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, shopkeeperScope)
	}

	return r.reconcile(ctx, shopkeeperScope)
}

func (r *TaskReconciler) reconcile(ctx context.Context, shopkeeperScope *scope.ShopkeeperScope) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling ShopkeeperTask")

	shopkeeperScope.Task.Status.Ready = false
	shopkeeperScope.Task.Status.FailureMessage = Pointer("")

	controllerutil.AddFinalizer(shopkeeperScope.Task, interactionsv1alpha1.ShopkeeperTaskFinalizer)
	if err := shopkeeperScope.PatchObject(); err != nil {
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

	if shopkeeperScope.Task.Status.JobStatus == "" {
		jobConfig := TransformJob(shopkeeperScope.Task)

		job, err := clientset.BatchV1().Jobs(shopkeeperScope.Task.Namespace).Apply(ctx, &jobConfig, v1.ApplyOptions{FieldManager: "application/apply-patch"})
		if err != nil {
			shopkeeperScope.Task.Status.FailureMessage = Pointer(err.Error())
			if err := shopkeeperScope.PatchObject(); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, err
		}

		shopkeeperScope.Task.Status.JobStatus = job.Status.String()
		shopkeeperScope.Task.Status.Ready = true
	}

	return ctrl.Result{}, nil
}

func (r *TaskReconciler) reconcileDelete(ctx context.Context, shopkeeperScope *scope.ShopkeeperScope) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling Delete ShopkeeperTask")

	if err := r.Client.Delete(ctx, shopkeeperScope.Task); err != nil {
		log.Error(err, "delete error")
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(shopkeeperScope.Task, interactionsv1alpha1.ShopkeeperTaskFinalizer)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&interactionsv1alpha1.Task{}).
		Complete(r)
}
