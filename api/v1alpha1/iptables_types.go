/*
Copyright 2020 The KubeSphere Authors.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubesphere.io/edge-watcher/pkg/utils"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// IPTablesSpec defines the desired state of IPTables
type IPTablesSpec struct {
	// IPtables Daemonset image.
	Image string `json:"image,omitempty"`
	// IPtables Daemonset image pull policy.
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// NodeSelector
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Tolerations
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// IPTablesStatus defines the observed state of IPTables
type IPTablesStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// IPTables is the Schema for the iptables API
type IPTables struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPTablesSpec   `json:"spec,omitempty"`
	Status IPTablesStatus `json:"status,omitempty"`
}

// IsBeingDeleted returns true if a deletion timestamp is set
func (it *IPTables) IsBeingDeleted() bool {
	return !it.ObjectMeta.DeletionTimestamp.IsZero()
}

// IPTablesFinalizerName is the name of the iptables finalizer
const IPTablesFinalizerName = "iptables.kubeedge.kubesphere.io"

// HasFinalizer returns true if the item has the specified finalizer
func (it *IPTables) HasFinalizer(finalizerName string) bool {
	return utils.ContainString(it.ObjectMeta.Finalizers, finalizerName)
}

// AddFinalizer adds the specified finalizer
func (it *IPTables) AddFinalizer(finalizerName string) {
	it.ObjectMeta.Finalizers = append(it.ObjectMeta.Finalizers, finalizerName)
}

// RemoveFinalizer removes the specified finalizer
func (it *IPTables) RemoveFinalizer(finalizerName string) {
	it.ObjectMeta.Finalizers = utils.RemoveString(it.ObjectMeta.Finalizers, finalizerName)
}

// +kubebuilder:object:root=true

// IPTablesList contains a list of IPTables
type IPTablesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPTables `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IPTables{}, &IPTablesList{})
}
