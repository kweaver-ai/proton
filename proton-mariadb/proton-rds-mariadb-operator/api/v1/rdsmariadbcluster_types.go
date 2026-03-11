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
	//appsv1 "k8s.io/api/apps/v1"
	//corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RDSMariaDBClusterSpec defines the desired state of RDSMariaDBCluster
type RDSMariaDBClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	SecretName string    `json:"secretName,omitempty"`
	Replicas   *int32    `json:"replicas,omitempty"`
	Etcd       *Etcd     `json:"etcd,omitempty" `
	Exporter   *Exporter `json:"exporter,omitempty" `
	Mgmt       *Mgmt     `json:"mgmt,omitempty" `
	Mariadb    *Mariadb  `json:"mariadb,omitempty" `
	HAProxy    *HAProxy  `json:"haproxy,omitempty" `
}

// RDSMariaDBClusterStatus defines the observed state of RDSMariaDBCluster
type RDSMariaDBClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	LastAppliedConfiguration RDSMariaDBClusterSpec `json:"lastAppliedConfiguration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RDSMariaDBCluster is the Schema for the rdsmariadbclusters API
type RDSMariaDBCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RDSMariaDBClusterSpec   `json:"spec,omitempty"`
	Status RDSMariaDBClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RDSMariaDBClusterList contains a list of RDSMariaDBCluster
type RDSMariaDBClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RDSMariaDBCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RDSMariaDBCluster{}, &RDSMariaDBClusterList{})
}

func (r *RDSMariaDBCluster) validateSpec() *field.Error {
	if r.Spec.Mariadb.Storage.StorageClassName == "" && int(*r.Spec.Replicas) > len(r.Spec.Mariadb.Storage.VolumeSpec) {
		return field.Invalid(
			field.NewPath("spec").Child("mariadb").Child("storage").Child("volumeSpec"),
			r.Spec.Mariadb.Storage.VolumeSpec,
			"invalid spec: replicas != len(volume)")
	}

	if r.Spec.Etcd == nil {
		return field.Invalid(
			field.NewPath("spec").Child("etcd"),
			r.Spec.Etcd,
			"invalid spec: etcd == nil")
	}

	return nil
}
