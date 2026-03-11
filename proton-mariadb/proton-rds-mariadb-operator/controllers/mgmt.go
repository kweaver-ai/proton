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

	rbacv1 "k8s.io/api/rbac/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type MgmtManager struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func (m *MgmtManager) Clear(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	return nil
}

func (m *MgmtManager) Deploy(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {

	lb_svc := m.constructLBService(o)
	err := m.createOrUpdate(ctx, o, lb_svc, true)
	if err != nil {
		return err
	}

	cm := m.constructCm(o)
	err = m.createOrUpdate(ctx, o, cm, true)
	if err != nil {
		return err
	}

	cr := m.constructClusterRole(o)
	err = m.createOrUpdate(ctx, o, cr, false)
	if err != nil {
		return err
	}

	sa := m.constructServiceAccount(o)
	err = m.createOrUpdate(ctx, o, sa, true)
	if err != nil {
		return err
	}

	crb := m.constructClusterRoleBinding(o)
	err = m.createOrUpdate(ctx, o, crb, false)
	if err != nil {
		return err
	}

	app := m.constructDeployment(o)
	err = m.createOrUpdate(ctx, o, app, true)
	if err != nil {
		return err
	}

	return err
}

func (m *MgmtManager) constructCm(o *rdsv1.RDSMariaDBCluster) *corev1.ConfigMap {
	tmpl := map[string]interface{}{
		"logLevel": "error",
		"lang":     "zh_CN",
	}
	for k, v := range o.Spec.Mgmt.Conf {
		switch v.Type {
		case intstr.Int:
			tmpl[k] = v.IntVal
		case intstr.String:
			tmpl[k] = v.StrVal
		}
	}

	cnf := ""
	for k, v := range tmpl {
		switch value := v.(type) {
		case string:
			cnf = cnf + fmt.Sprintf("%s : %s\n", k, value)
		case int, int32:
			cnf = cnf + fmt.Sprintf("%s : %d\n", k, value)
		}

	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mgmt-config",
			Namespace: o.GetNamespace(),
		},
		Data: map[string]string{
			"rds-mgmt.yaml": cnf,
		},
	}

}

func (m *MgmtManager) constructClusterRole(o *rdsv1.RDSMariaDBCluster) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: o.GetName() + "-mgmt-role",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"",
					"apps",
					"batch",
				},
				Resources: []string{
					"persistentvolumes",
					"jobs",
					"pods",
					"pods/exec",
					"statefulsets",
					"statefulsets/scale",
				},
				Verbs: []string{
					"create",
					"delete",
					"get",
					"list",
					"update",
				},
			},
		},
	}

}

func (m *MgmtManager) constructServiceAccount(o *rdsv1.RDSMariaDBCluster) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.GetName() + "-mgmt",
			Namespace: o.GetNamespace(),
		},
	}
}

func (m *MgmtManager) constructClusterRoleBinding(o *rdsv1.RDSMariaDBCluster) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: o.GetName() + "-mgmt-rolebinding",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      o.GetName() + "-mgmt",
				Namespace: o.GetNamespace(),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     o.GetName() + "-mgmt-role",
		},
	}
}

func (m *MgmtManager) constructDeployment(o *rdsv1.RDSMariaDBCluster) *appsv1.Deployment {
	var n int32 = 2
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.GetName() + "-mgmt",
			Namespace: o.GetNamespace(),
			Labels: map[string]string{
				"app": o.GetName() + "-mgmt",
			},
		},

		Spec: appsv1.DeploymentSpec{
			Replicas: &n,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": o.Name + "-mgmt",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: o.Name + "-mgmt",
					Labels: map[string]string{
						"app": o.Name + "-mgmt",
					},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 1,
									PodAffinityTerm: corev1.PodAffinityTerm{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "app",
													Operator: metav1.LabelSelectorOpIn,
													Values:   []string{o.Name + "-mgmt"},
												},
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
					},
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
					ServiceAccountName: o.GetName() + "-mgmt",
					Containers: []corev1.Container{
						{
							Name:            "mgmt",
							Image:           o.Spec.Mgmt.Image,
							ImagePullPolicy: o.Spec.Mgmt.ImagePullPolicy,
							Ports: []corev1.ContainerPort{
								{
									Name:          "mgmt",
									ContainerPort: 8888,
								},
							},
							Env: []corev1.EnvVar{
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
								{
									Name:  "MYSQL_HOST",
									Value: o.GetName() + "-mariadb-cluster",
								},
								{
									Name:  "MYSQL_PORT",
									Value: fmt.Sprint(o.Spec.Mariadb.Service.Port),
								},
								{
									Name:  "NAMESPACE",
									Value: o.GetNamespace(),
								},
								{
									Name:  "CR_NAME",
									Value: o.GetName(),
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Port: intstr.IntOrString{
											IntVal: 8888,
										},
										Path: "/api/proton-rds-mgmt/v2/health",
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       5,
								TimeoutSeconds:      2,
								FailureThreshold:    15,
							},
							Resources: o.Spec.Mgmt.Resources,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "mgmt-config",
									MountPath: "/etc/rds-mgmt-conf/",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "mgmt-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "mgmt-config",
									},
									Items: []corev1.KeyToPath{
										{
											Key:  "rds-mgmt.yaml",
											Path: "rds-mgmt.yaml",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (m *MgmtManager) constructLBService(o *rdsv1.RDSMariaDBCluster) *corev1.Service {
	if o.Spec.Mgmt.Service.EnableDualStack {
		var ipPolicy corev1.IPFamilyPolicyType = corev1.IPFamilyPolicyRequireDualStack
		return &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Servivce",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.GetName() + "-mgmt-cluster",
				Namespace: o.GetNamespace(),
			},
			Spec: corev1.ServiceSpec{
				IPFamilies: []corev1.IPFamily{
					corev1.IPv6Protocol,
					corev1.IPv4Protocol,
				},
				IPFamilyPolicy: &ipPolicy,
				Selector:       map[string]string{"app": o.GetName() + "-mgmt"},

				Ports: []corev1.ServicePort{
					{
						Name: "mgmt",
						Port: int32(o.Spec.Mgmt.Service.Port),
						TargetPort: intstr.IntOrString{
							IntVal: 8888,
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
			Name:      o.GetName() + "-mgmt-cluster",
			Namespace: o.GetNamespace(),
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": o.GetName() + "-mgmt"},
			Ports: []corev1.ServicePort{
				{
					Name: "mgmt",
					Port: int32(o.Spec.Mgmt.Service.Port),
					TargetPort: intstr.IntOrString{
						IntVal: 8888,
					},
				},
			},
		},
	}
}

func (m *MgmtManager) createOrUpdate(ctx context.Context, owns, obj client.Object, setController bool) error {
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

	err := m.Client.Get(context.Background(), types.NamespacedName{
		Name:      objectMeta.GetName(),
		Namespace: objectMeta.GetNamespace(),
	}, oldObject)

	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	if k8serrors.IsNotFound(err) {
		if setController {
			err = ctrl.SetControllerReference(owns, obj, m.Scheme)
			if err != nil {
				return err
			}
		}
		return m.Client.Create(ctx, obj)
	}

	oldObjectMeta := oldObject.(metav1.ObjectMetaAccessor).GetObjectMeta()
	objectMeta.SetResourceVersion(oldObjectMeta.GetResourceVersion())
	switch object := obj.(type) {
	case *corev1.Service:
		object.Spec.ClusterIP = oldObject.(*corev1.Service).Spec.ClusterIP
	}
	if setController {
		err = ctrl.SetControllerReference(owns, obj, m.Scheme)
		if err != nil {
			return err
		}
	}

	return m.Client.Update(context.TODO(), obj)
}

func (m *MgmtManager) Init(client client.Client, clientCMD *ClientCMD, scheme *runtime.Scheme) {
	m.Client = client
	m.Scheme = scheme
}

func init() {
	deployer = append(deployer, &MgmtManager{})
}
