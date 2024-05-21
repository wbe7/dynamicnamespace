package v1

import (
	"k8s.io/api/core/v1"
	"k8s.io/api/rbac/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DynamicNamespaceSpec defines the desired state of DynamicNamespace
type DynamicNamespaceSpec struct {
	// +kubebuilder:default:={cpu: "100m", ephemeral-storage: "100Mi", memory: "100Mi"}
	// +optional
	CreateQuota v1.ResourceList `json:"createQuota,omitempty"`

	// +optional
	RoleBindingSubjects []v1beta1.Subject `json:"roleBindingSubjects,omitempty"`
}

// DynamicNamespaceStatus defines the observed state of DynamicNamespace
type DynamicNamespaceStatus struct {
	// +kubebuilder:validation:Enum=ACTIVE;ERROR
	// Код статуса
	Code string `json:"code"`

	// Информация о состоянии ресурса
	Message string `json:"message"`
}

// +kubebuilder:printcolumn:name="Status",description="Текущий статус ресурса",type=string,JSONPath=`.status.code`
// +kubebuilder:printcolumn:name="Message",description="Сообщение о статусе ресурса",type=string,JSONPath=`.status.message`
// +kubebuilder:printcolumn:name="Timestamp",description="Дата создания",type=string,JSONPath=`.metadata.creationTimestamp`

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=dn
// +kubebuilder:k8s:openapi-gen=true

// DynamicNamespace is the Schema for the dynamicnamespaces API
type DynamicNamespace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DynamicNamespaceSpec   `json:"spec,omitempty"`
	Status DynamicNamespaceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DynamicNamespaceList contains a list of DynamicNamespace
type DynamicNamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DynamicNamespace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DynamicNamespace{}, &DynamicNamespaceList{})
}
