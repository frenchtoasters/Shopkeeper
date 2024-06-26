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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// PausedAnnotation is an annotation that can be applied to any ShopkeeperTask
	// object to prevent a controller from processing a resource.
	//
	// Controllers working with ShopkeeperTask API objects must check the existence of this annotation
	// on the reconciled object.
	PausedAnnotation = "shopkeeper.x-k8s.io/paused"

	// ShopkeeperTaskFinalizer is the finalizer used by the cluster controller to
	// cleanup the cluster resources when a ShopkeeperTask is being deleted.
	ShopkeeperTaskFinalizer = "task.shopkeeper.x-k8s.io"
)

// TaskSpec defines the desired state of Task
type TaskSpec struct {
	Name           string   `json:"name,omitempty"`
	Image          string   `json:"image,omitempty"`
	Tag            string   `default:"latest" json:"tag,omitempty"`
	ServiceAccount string   `default:"default" json:"service_account,omitempty"`
	Command        []string `json:"command,omitempty"`
	Paused         bool     `json:"paused,omitempty"`
}

// TaskStatus defines the observed state of Task
type TaskStatus struct {
	Ready          bool    `json:"ready,omitempty"`
	JobStatus      string  `json:"job_status,omitempty"`
	FailureMessage *string `json:"failureMessage,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Task is the Schema for the tasks API
type Task struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TaskSpec   `json:"spec,omitempty"`
	Status TaskStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TaskList contains a list of Task
type TaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Task `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Task{}, &TaskList{})
}
