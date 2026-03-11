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

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rdsv1 "proton-rds-mariadb-operator/api/v1"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type StorageManager struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func (s *StorageManager) Deploy(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	pvs := s.constructPV(o)
	for _, pv := range pvs {
		err := s.createOrUpdate(ctx, o, pv, false)
		if err != nil {
			return err
		}
	}
	pvcs := s.constructPVC(o)
	for _, pvc := range pvcs {
		err := s.createOrUpdate(ctx, o, pvc, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *StorageManager) constructPV(o *rdsv1.RDSMariaDBCluster) []*corev1.PersistentVolume {
	pvs := []*corev1.PersistentVolume{}
	if o.Spec.Mariadb.Storage.StorageClassName != "" {
		return pvs
	}
	var i int32 = 0
	quantity, _ := resource.ParseQuantity(o.Spec.Mariadb.Storage.Capacity)
	for i < *o.Spec.Replicas {
		pv := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: o.GetNamespace() + "-" + o.GetName() + "-mariadb-pv-" + strconv.Itoa(int(i)),
				Labels: map[string]string{
					"podindex": strconv.Itoa(int(i)),
					"app":      o.GetName() + "-mariadb",
				},
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					Local: &corev1.LocalVolumeSource{
						Path: o.Spec.Mariadb.Storage.VolumeSpec[i].Path,
					},
				},
				NodeAffinity: &corev1.VolumeNodeAffinity{
					Required: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/hostname",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{o.Spec.Mariadb.Storage.VolumeSpec[i].Host},
									},
								},
							},
						},
					},
				},
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Capacity: corev1.ResourceList{
					corev1.ResourceStorage: quantity,
				},
			},
		}
		pvs = append(pvs, pv)
		i++
	}

	return pvs
}

func (s *StorageManager) constructPVC(o *rdsv1.RDSMariaDBCluster) []*corev1.PersistentVolumeClaim {
	pvcs := []*corev1.PersistentVolumeClaim{}
	if o.Spec.Mariadb.Storage.StorageClassName != "" {
		return pvcs
	}
	var i int32 = 0
	tmp := ""
	quantity, _ := resource.ParseQuantity(o.Spec.Mariadb.Storage.Capacity)
	for i < *o.Spec.Replicas {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mariadb-datadir-" + o.GetName() + "-mariadb-" + strconv.Itoa(int(i)),
				Namespace: o.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &tmp,
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: quantity,
					},
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"podindex": strconv.Itoa(int(i)),
						"app":      o.GetName() + "-mariadb",
					},
				},
			},
		}
		pvcs = append(pvcs, pvc)
		i++
	}

	return pvcs
}

func (s *StorageManager) createOrUpdate(ctx context.Context, owns, obj client.Object, setController bool) error {
	log := log.FromContext(ctx)
	metaAccessor, ok := obj.(metav1.ObjectMetaAccessor)
	if !ok {
		return fmt.Errorf("can't convert object to ObjectMetaAccessor")
	}

	objectMeta := metaAccessor.GetObjectMeta()
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = reflect.Indirect(val)
	}
	oldObject := reflect.New(val.Type()).Interface().(client.Object)

	err := s.Client.Get(context.Background(), types.NamespacedName{
		Name:      objectMeta.GetName(),
		Namespace: objectMeta.GetNamespace(),
	}, oldObject)

	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	if k8serrors.IsNotFound(err) {
		if setController {
			err = ctrl.SetControllerReference(owns, obj, s.Scheme)
			if err != nil {
				return err
			}
		}
		return s.Client.Create(ctx, obj)
	}

	oldObjectMeta := oldObject.(metav1.ObjectMetaAccessor).GetObjectMeta()
	objectMeta.SetResourceVersion(oldObjectMeta.GetResourceVersion())
	if setController {
		err = ctrl.SetControllerReference(owns, obj, s.Scheme)
		if err != nil {
			return err
		}
	}

	err = s.Client.Update(context.TODO(), obj)
	if k8serrors.IsInvalid(err) {
		log.Info(err.Error())
		return nil
	}
	return err
}

func (s *StorageManager) Clear(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	if o.Spec.Mariadb.Storage.StorageClassName != "" {
		return nil
	}

	var i int = 0
	storage := resource.Quantity{Format: resource.BinarySI}
	storage.Set(4096)
	for i < len(o.Spec.Mariadb.Storage.VolumeSpec) {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mariadb-datadir-" + o.GetName() + "-mariadb-" + strconv.Itoa(int(i)),
				Namespace: o.Namespace,
			}}
		err := s.Client.Get(ctx, client.ObjectKey{
			Name:      "mariadb-datadir-" + o.GetName() + "-mariadb-" + strconv.Itoa(int(i)),
			Namespace: o.Namespace,
		}, pvc)
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
		if err == nil {
			err = s.Client.Delete(ctx, pvc)
			if err != nil {
				return err
			}
		}

		pv := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: o.GetNamespace() + "-" + o.GetName() + "-mariadb-pv-" + strconv.Itoa(int(i)),
			},
		}
		err = s.Client.Get(ctx, client.ObjectKey{
			Name: o.GetNamespace() + "-" + o.GetName() + "-mariadb-pv-" + strconv.Itoa(int(i)),
		}, pv)
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
		if err == nil {
			err = s.Client.Delete(ctx, pv)
			if err != nil {
				return err
			}
		}
		i++
	}
	return nil
}

func (s *StorageManager) Init(client client.Client, scheme *runtime.Scheme) {
	s.Client = client
	s.Scheme = scheme
}
