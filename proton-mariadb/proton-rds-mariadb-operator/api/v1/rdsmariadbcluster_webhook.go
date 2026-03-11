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
	"math"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var rdsmariadbclusterlog = logf.Log.WithName("rdsmariadbcluster-resource")

func (r *RDSMariaDBCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-rds-proton-aishu-cn-v1-rdsmariadbcluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=rds.proton.aishu.cn,resources=rdsmariadbclusters,verbs=create;update,versions=v1,name=mrdsmariadbcluster.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &RDSMariaDBCluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *RDSMariaDBCluster) Default() {
	rdsmariadbclusterlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-rds-proton-aishu-cn-v1-rdsmariadbcluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=rds.proton.aishu.cn,resources=rdsmariadbclusters,verbs=create;update,versions=v1,name=vrdsmariadbcluster.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &RDSMariaDBCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *RDSMariaDBCluster) ValidateCreate() error {
	rdsmariadbclusterlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	var allErrs field.ErrorList
	err := r.validateSpec()
	if err != nil {
		allErrs = append(allErrs, err)

	}

	if len(allErrs) == 0 {
		return nil
	}
	return k8serrors.NewInvalid(
		schema.GroupKind{Group: "rds", Kind: "RDSMariaDBCluster"},
		r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *RDSMariaDBCluster) ValidateUpdate(old runtime.Object) error {
	rdsmariadbclusterlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	oldR := old.(*RDSMariaDBCluster)

	if oldR.Spec.Mariadb.Storage.Capacity != r.Spec.Mariadb.Storage.Capacity {
		oldQuantity, _ := resource.ParseQuantity(oldR.Spec.Mariadb.Storage.Capacity)
		newQuantity, _ := resource.ParseQuantity(r.Spec.Mariadb.Storage.Capacity)
		if newQuantity.Cmp(oldQuantity) < 0 {
			return field.Forbidden(
				field.NewPath("spec").Child("mariadb").Child("storage").Child("capacity"),
				"fileld can not be less than previous value")
		}
	}

	if oldR.Spec.Mariadb.Storage.StorageClassName != r.Spec.Mariadb.Storage.StorageClassName {
		return field.Forbidden(
			field.NewPath("spec").Child("mariadb").Child("storage").Child("storageClassName"),
			"fileld is immutable after creation")
	}

	if r.Spec.Mariadb.Storage.StorageClassName == "" {
		if int(*r.Spec.Replicas) > len(r.Spec.Mariadb.Storage.VolumeSpec) {
			return field.Invalid(
				field.NewPath("spec").Child("mariadb").Child("storage").Child("volumeSpec"),
				r.Spec.Mariadb.Storage.VolumeSpec,
				"invalid spec: replicas != len(volume)")
		}

		var i int = 0
		for i < int(math.Min(float64(*oldR.Spec.Replicas), float64(*r.Spec.Replicas))) {
			if oldR.Spec.Mariadb.Storage.VolumeSpec[i] != r.Spec.Mariadb.Storage.VolumeSpec[i] {
				return field.Forbidden(
					field.NewPath("spec").Child("mariadb").Child("storage").Child("volumeSpec").Index(i),
					"fileld is immutable after creation")
			}
			i++
		}
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *RDSMariaDBCluster) ValidateDelete() error {
	rdsmariadbclusterlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
