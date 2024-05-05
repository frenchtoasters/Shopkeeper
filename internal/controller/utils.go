package controller

import (
	"context"
	"fmt"

	interactionsv1alpha1 "frenchtoasters.io/shopkeeper/api/v1alpha1"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/batch/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"

	v1 "k8s.io/api/core/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
)

// Pointer returns the pointer of any type
func Pointer[T any](t T) *T {
	return &t
}

// hasAnnotation returns true if the object has the specified annotation.
func hasAnnotation(o metav1.Object, annotation string) bool {
	annotations := o.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[annotation]
	return ok
}

// HasPaused returns true if the object has the `paused` annotation.
func HasPaused(o metav1.Object) bool {
	return hasAnnotation(o, interactionsv1alpha1.PausedAnnotation)
}

// IsPaused returns true if the Shopkeeper is paused or the object has the `paused` annotation.
func IsPaused(keycloak *interactionsv1alpha1.Task, o metav1.Object) bool {
	if keycloak.Spec.Paused {
		return true
	}
	return HasPaused(o)
}

// GetTaskByName finds and return a ShopkeeperTask object using the specified params.
func GetTaskByName(ctx context.Context, c client.Client, namespace, name string) (*interactionsv1alpha1.Task, error) {
	task := &interactionsv1alpha1.Task{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := c.Get(ctx, key, task); err != nil {
		return nil, errors.Wrapf(err, "failed to get Shopkeeper/%s", name)
	}

	return task, nil
}

// TransformJob
func TransformJob(task *interactionsv1alpha1.Task) applyv1.JobApplyConfiguration {
	return applyv1.JobApplyConfiguration{
		TypeMetaApplyConfiguration: applymetav1.TypeMetaApplyConfiguration{
			Kind:       Pointer("Job"),
			APIVersion: Pointer("batch/v1"),
		},
		ObjectMetaApplyConfiguration: &applymetav1.ObjectMetaApplyConfiguration{
			Name: &task.Spec.Name,
		},
		Spec: &applyv1.JobSpecApplyConfiguration{
			Template: &applycorev1.PodTemplateSpecApplyConfiguration{
				Spec: &applycorev1.PodSpecApplyConfiguration{
					ServiceAccountName: &task.Spec.ServiceAccount,
					Containers: []applycorev1.ContainerApplyConfiguration{
						{
							Name:    &task.Spec.Name,
							Image:   Pointer(fmt.Sprintf("%s:%s", task.Spec.Image, task.Spec.Tag)),
							Command: task.Spec.Command,
						},
					},
					RestartPolicy: Pointer(v1.RestartPolicyNever),
				},
			},
		},
	}
}
