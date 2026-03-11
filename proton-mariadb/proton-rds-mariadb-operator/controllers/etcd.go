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

type EtcdManager struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func (e *EtcdManager) Clear(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	return nil
}

func (e *EtcdManager) Deploy(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	svc := e.constructLBService(o)
	err := e.createOrUpdate(ctx, o, svc)
	if err != nil {
		return err
	}

	app := e.constructDeployment(o)
	err = e.createOrUpdate(ctx, o, app)
	if err != nil {
		return err
	}

	return err
}

func (e *EtcdManager) constructDeployment(o *rdsv1.RDSMariaDBCluster) *appsv1.Deployment {
	var n int32 = 1
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.GetName() + "-etcd",
			Namespace: o.GetNamespace(),
			Labels: map[string]string{
				"app": o.GetName() + "-etcd",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &n,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": o.Name + "-etcd",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: o.Name + "-etcd",
					Labels: map[string]string{
						"app": o.Name + "-etcd",
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
							Name:            "etcd",
							Image:           o.Spec.Etcd.Image,
							ImagePullPolicy: o.Spec.Etcd.ImagePullPolicy,
							Ports: []corev1.ContainerPort{
								{
									Name:          "etcd-client",
									ContainerPort: 2379,
								},
								{
									Name:          "etcd-peer",
									ContainerPort: 2380,
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											IntVal: 2379,
										},
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       5,
								TimeoutSeconds:      2,
								FailureThreshold:    30,
							},
							Command: []string{
								"/bin/sh",
								"-ecx",
								fmt.Sprintf(
									"sleep 15 \n"+
										"exec etcd --name ${HOSTNAME} "+
										"--listen-peer-urls http://0.0.0.0:2380 "+
										"--listen-client-urls http://0.0.0.0:2379 "+
										"--advertise-client-urls http://%s-etcd-cluster:2379 "+
										"--initial-advertise-peer-urls http://%s-etcd-cluster:2380 "+
										"--initial-cluster-token etcd-cluster-1 "+
										"--initial-cluster ${HOSTNAME}=http://%s-etcd-cluster:2380 "+
										"--data-dir /var/lib/etcd "+
										"--initial-cluster-state new ", o.GetName(), o.GetName(), o.GetName(),
								),
							},
						},
					},
				},
			},
		},
	}
}

func (e *EtcdManager) constructLBService(o *rdsv1.RDSMariaDBCluster) *corev1.Service {
	if o.Spec.Mgmt.Service.EnableDualStack || o.Spec.Mariadb.Service.EnableDualStack {
		var ipPolicy corev1.IPFamilyPolicyType = corev1.IPFamilyPolicyRequireDualStack
		return &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Servivce",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.GetName() + "-etcd-cluster",
				Namespace: o.GetNamespace(),
			},
			Spec: corev1.ServiceSpec{
				IPFamilies: []corev1.IPFamily{
					corev1.IPv6Protocol,
					corev1.IPv4Protocol,
				},
				IPFamilyPolicy: &ipPolicy,
				Selector:       map[string]string{"app": o.GetName() + "-etcd"},
				Ports: []corev1.ServicePort{
					{
						Name: "etcd-client",
						Port: 2379,
						TargetPort: intstr.IntOrString{
							IntVal: 2379,
						},
					},
					{
						Name: "etcd-peer",
						Port: 2380,
						TargetPort: intstr.IntOrString{
							IntVal: 2380,
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
			Name:      o.GetName() + "-etcd-cluster",
			Namespace: o.GetNamespace(),
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": o.GetName() + "-etcd"},
			Ports: []corev1.ServicePort{
				{
					Name: "etcd-client",
					Port: 2379,
					TargetPort: intstr.IntOrString{
						IntVal: 2379,
					},
				},
				{
					Name: "etcd-peer",
					Port: 2380,
					TargetPort: intstr.IntOrString{
						IntVal: 2380,
					},
				},
			},
		},
	}

}

func (e *EtcdManager) createOrUpdate(ctx context.Context, owns, obj client.Object) error {
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

func (e *EtcdManager) Init(client client.Client, clientCMD *ClientCMD, scheme *runtime.Scheme) {
	e.Client = client
	e.Scheme = scheme
}

func init() {
	deployer = append(deployer, &EtcdManager{})
}
