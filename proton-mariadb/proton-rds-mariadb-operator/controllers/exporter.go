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

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rdsv1 "proton-rds-mariadb-operator/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type ExporterManager struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func (e *ExporterManager) Clear(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	return nil
}

func (e *ExporterManager) Deploy(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	svc := e.constructService(o)
	err := e.createOrUpdate(ctx, o, svc)
	if err != nil {
		return err
	}

	app := e.constructSts(o)
	err = e.createOrUpdate(ctx, o, app)
	if err != nil {
		return err
	}

	return err
}

func (e *ExporterManager) constructSts(o *rdsv1.RDSMariaDBCluster) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Statefulset",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.GetName() + "-exporter",
			Namespace: o.GetNamespace(),
			Labels: map[string]string{
				"app": o.GetName() + "-exporter",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         o.GetName() + "-exporter",
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Replicas:            o.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": o.Name + "-exporter",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: o.Name + "-exporter",
					Labels: map[string]string{
						"app": o.Name + "-exporter",
					},
				},
				Spec: corev1.PodSpec{
					DNSConfig: &corev1.PodDNSConfig{
						Options: []corev1.PodDNSConfigOption{
							{
								Name: "single-request-reopen",
							},
							{
								Name: "use-vc",
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "exporter",
							Image:           o.Spec.Exporter.Image,
							ImagePullPolicy: o.Spec.Exporter.ImagePullPolicy,
							Ports: []corev1.ContainerPort{
								{
									Name:          "exporter",
									ContainerPort: 9104,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name:  "DB_POD_NAME_PRE",
									Value: o.GetName() + "-mariadb",
								},
								{
									Name:  "DB_SVC",
									Value: o.GetName() + "-mariadb",
								},
								{
									Name:  "MYSQL_PORT",
									Value: "3306",
								},
								{
									Name: "MYSQL_ROOT_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: o.Spec.SecretName,
											},
											Key: "username",
										},
									},
								},
								{
									Name: "MYSQL_ROOT_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: o.Spec.SecretName,
											},
											Key: "password",
										},
									},
								},
							},
							Args: []string{
								"--collect.binlog_size",
								"--collect.slave_status",
								"--collect.info_schema.processlist",
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											IntVal: 9104,
										},
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       3,
								TimeoutSeconds:      2,
								FailureThreshold:    15,
							},
						},
					},
				},
			},
		},
	}
}

func (e *ExporterManager) constructService(o *rdsv1.RDSMariaDBCluster) *corev1.Service {
	if o.Spec.Mgmt.Service.EnableDualStack || o.Spec.Mariadb.Service.EnableDualStack {
		var ipPolicy corev1.IPFamilyPolicyType = corev1.IPFamilyPolicyRequireDualStack
		return &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Servivce",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.GetName() + "-exporter",
				Namespace: o.GetNamespace(),
				Annotations: map[string]string{
					"prometheus.io/scrape": "true",
					"prometheus.io/port":   "9104",
				},
			},
			Spec: corev1.ServiceSpec{
				IPFamilies: []corev1.IPFamily{
					corev1.IPv6Protocol,
					corev1.IPv4Protocol,
				},
				IPFamilyPolicy: &ipPolicy,
				Selector:       map[string]string{"app": o.GetName() + "-exporter"},
				ClusterIP:      "None",
				Ports: []corev1.ServicePort{
					{
						Name: "exporter",
						Port: 9104,
						TargetPort: intstr.IntOrString{
							IntVal: 9104,
						},
					},
				},
			},
		}
	}
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Servivce",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.GetName() + "-exporter",
			Namespace: o.GetNamespace(),
			Annotations: map[string]string{
				"prometheus.io/scrape": "true",
				"prometheus.io/port":   "9104",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector:  map[string]string{"app": o.GetName() + "-exporter"},
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{
					Name: "exporter",
					Port: 9104,
					TargetPort: intstr.IntOrString{
						IntVal: 9104,
					},
				},
			},
		},
	}

}

func (e *ExporterManager) createOrUpdate(ctx context.Context, owns, obj client.Object) error {
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

	err := e.Client.Get(context.Background(), types.NamespacedName{
		Name:      objectMeta.GetName(),
		Namespace: objectMeta.GetNamespace(),
	}, oldObject)

	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	if k8serrors.IsNotFound(err) {
		err = ctrl.SetControllerReference(owns, obj, e.Scheme)
		if err != nil {
			return err
		}
		return e.Client.Create(ctx, obj)
	}

	oldObjectMeta := oldObject.(metav1.ObjectMetaAccessor).GetObjectMeta()
	objectMeta.SetResourceVersion(oldObjectMeta.GetResourceVersion())
	switch object := obj.(type) {
	case *corev1.Service:
		object.Spec.ClusterIP = oldObject.(*corev1.Service).Spec.ClusterIP
	}
	err = ctrl.SetControllerReference(owns, obj, e.Scheme)
	if err != nil {
		return err
	}

	return e.Client.Update(context.TODO(), obj)
}

func (e *ExporterManager) Init(client client.Client, clientCMD *ClientCMD, scheme *runtime.Scheme) {
	e.Client = client
	e.Scheme = scheme
}

func init() {
	deployer = append(deployer, &ExporterManager{})
}
