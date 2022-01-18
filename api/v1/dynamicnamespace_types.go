/*
Copyright 2022.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DynamicNamespaceSpec defines the desired state of DynamicNamespace
type DynamicNamespaceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Если установлено, то создастся ServiceAccount с правами на Namespace
	CreateSA            bool `json:"createSA,omitempty"`
	CreateResourceQuota bool `json:"createResourceQuota,omitempty"`
}

// DynamicNamespaceStatus defines the observed state of DynamicNamespace
type DynamicNamespaceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Enum=ACTIVE;ERROR
	// Код статуса
	Code string `json:"code"`

	// Информация о состоянии ресурса
	Message string `json:"message"`
}

// +kubebuilder:printcolumn:name="Status",description="Текущий статус ресурса",type=string,JSONPath=`.status.code`
// +kubebuilder:printcolumn:name="Message",description="Сообщение о статусе ресурса",type=string,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Timestamp",description="Дата создания",type=string,JSONPath=`.metadata.creationTimestamp`

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:shortName=dn
// +kubebuilder:k8s:openapi-gen=true

// DynamicNamespace is the Schema for the dynamicnamespaces API
type DynamicNamespace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DynamicNamespaceSpec   `json:"spec,omitempty"`
	Status DynamicNamespaceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DynamicNamespaceList contains a list of DynamicNamespace
type DynamicNamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DynamicNamespace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DynamicNamespace{}, &DynamicNamespaceList{})
}
