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
	"bytes"
	"context"
	"fmt"
	"reflect"
	"text/template"

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

type HAProxyManager struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func (h *HAProxyManager) Clear(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	return nil
}

func (h *HAProxyManager) Deploy(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	if o.Spec.HAProxy == nil || !o.Spec.HAProxy.Enabled {
		return nil
	}
	svc := h.constructLBService(o)
	err := h.createOrUpdate(ctx, o, svc)
	if err != nil {
		return err
	}

	cm := h.constructCm(o)
	err = h.createOrUpdate(ctx, o, cm)
	if err != nil {
		return err
	}

	app := h.constructDeployment(o)
	err = h.createOrUpdate(ctx, o, app)
	if err != nil {
		return err
	}

	return err
}

func (h *HAProxyManager) constructCm(o *rdsv1.RDSMariaDBCluster) *corev1.ConfigMap {
	cfgTmpl := `
global
	daemon
	gid 1001
	uid 1001
	maxconn 30000
	log stdout format raw local0
	log stderr format raw local0 notice
	insecure-fork-wanted
	insecure-setuid-wanted
	external-check

defaults
	mode tcp
	log global
	option tcplog
	timeout connect 5000ms
	timeout client 3600s
	timeout server 3600s

listen mysqlwrite
	mode tcp
	bind 0.0.0.0:6444
	balance first
	option external-check
	external-check command /health_check.sh
	external-check path "/usr/bin:/bin"
{{range $i, $v := .Replica}}
	server mysql{{ $i }} {{ $.RDSReleaseName }}-mariadb-{{ $i }}.{{ $.RDSReleaseName }}-mariadb.{{ $.RDSNamespace }}.svc.cluster.local:3306  check inter 1s rise 2 fall 2 resolvers mydns resolve-prefer ipv4
{{end}}
listen mysqlread
	mode tcp
	bind 0.0.0.0:6445
	balance leastconn
	option external-check
	external-check command /health_check.sh
	external-check path "/usr/bin:/bin"
{{range $i, $v := .Replica}}
	server mysql{{ $i }} {{ $.RDSReleaseName }}-mariadb-{{ $i }}.{{ $.RDSReleaseName }}-mariadb.{{ $.RDSNamespace }}.svc.cluster.local:3306  check inter 1s rise 2 fall 2 resolvers mydns resolve-prefer ipv4
{{end}}
listen stats
	mode http
	bind 0.0.0.0:1080
	stats enable
	stats refresh 10s
	stats uri /hamonitor
	stats hide-version
resolvers mydns
	parse-resolv-conf
	resolve_retries       3
	timeout retry        1s
	hold other           1s
	hold refused         1s
	hold nx              1s
	hold timeout         1s
	hold valid           1s
`
	tmpl := template.New("cfg")
	tmpl, err := tmpl.Parse(cfgTmpl)
	if err != nil {
		panic(err)
	}
	buf := new(bytes.Buffer)
	server := struct {
		RDSReleaseName string
		RDSNamespace   string
		Replica        []int
	}{
		o.GetName(),
		o.GetNamespace(),
		make([]int, *o.Spec.Replicas),
	}
	err = tmpl.Execute(buf, server)
	if err != nil {
		panic(err)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "haproxy-config",
			Namespace: o.GetNamespace(),
		},
		Data: map[string]string{
			"haproxy.cfg": buf.String(),
		},
	}

}

func (h *HAProxyManager) constructDeployment(o *rdsv1.RDSMariaDBCluster) *appsv1.Deployment {
	var tmp int64 = 1001
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.GetName() + "-haproxy",
			Namespace: o.GetNamespace(),
			Labels: map[string]string{
				"app": o.GetName() + "-haproxy",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: o.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": o.Name + "-haproxy",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: o.Name + "-haproxy",
					Labels: map[string]string{
						"app": o.Name + "-haproxy",
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
													Values:   []string{o.Name + "-haproxy"},
												},
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
						PodAffinity: &corev1.PodAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 1,
									PodAffinityTerm: corev1.PodAffinityTerm{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "app",
													Operator: metav1.LabelSelectorOpIn,
													Values:   []string{o.Name + "-mariadb"},
												},
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
						NodeAffinity: o.Spec.HAProxy.NodeAffinity,
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  &tmp,
						RunAsGroup: &tmp,
					},
					Containers: []corev1.Container{
						{
							Name:            "haproxy",
							Image:           o.Spec.HAProxy.Image,
							ImagePullPolicy: o.Spec.HAProxy.ImagePullPolicy,
							Ports: []corev1.ContainerPort{
								{
									Name:          "first",
									ContainerPort: 6444,
								},
								{
									Name:          "leastconn",
									ContainerPort: 6445,
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.IntOrString{
											IntVal: 6444,
										},
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       20,
								TimeoutSeconds:      2,
								FailureThreshold:    3,
							},
							Resources: o.Spec.HAProxy.Resources,
							Args: []string{
								"-f",
								"/usr/local/etc/haproxy/haproxy.cfg",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "haproxy-config",
									MountPath: "/usr/local/etc/haproxy/haproxy.cfg",
									SubPath:   "haproxy.cfg",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "haproxy-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "haproxy-config",
									},
									Items: []corev1.KeyToPath{
										{
											Key:  "haproxy.cfg",
											Path: "haproxy.cfg",
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

func (h *HAProxyManager) constructLBService(o *rdsv1.RDSMariaDBCluster) *corev1.Service {
	if o.Spec.HAProxy.Service.EnableDualStack {
		var ipPolicy corev1.IPFamilyPolicyType = corev1.IPFamilyPolicyRequireDualStack
		return &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Servivce",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.GetName() + "-haproxy-cluster",
				Namespace: o.GetNamespace(),
			},
			Spec: corev1.ServiceSpec{
				IPFamilies: []corev1.IPFamily{
					corev1.IPv6Protocol,
					corev1.IPv4Protocol,
				},
				IPFamilyPolicy: &ipPolicy,
				Selector:       map[string]string{"app": o.GetName() + "-haproxy"},
				Ports: []corev1.ServicePort{
					{
						Name: "single-master-port",
						Port: int32(o.Spec.HAProxy.Service.SingleMasterPort),
						TargetPort: intstr.IntOrString{
							IntVal: 6444,
						},
					},
					{
						Name: "multi-master-port",
						Port: int32(o.Spec.HAProxy.Service.MultiMasterPort),
						TargetPort: intstr.IntOrString{
							IntVal: 6445,
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
			Name:      o.GetName() + "-haproxy-cluster",
			Namespace: o.GetNamespace(),
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": o.GetName() + "-haproxy"},
			Ports: []corev1.ServicePort{
				{
					Name: "single-master-port",
					Port: int32(o.Spec.HAProxy.Service.SingleMasterPort),
					TargetPort: intstr.IntOrString{
						IntVal: 6444,
					},
				},
				{
					Name: "multi-master-port",
					Port: int32(o.Spec.HAProxy.Service.MultiMasterPort),
					TargetPort: intstr.IntOrString{
						IntVal: 6445,
					},
				},
			},
		},
	}

}

func (h *HAProxyManager) createOrUpdate(ctx context.Context, owns, obj client.Object) error {
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

	err := h.Client.Get(context.Background(), types.NamespacedName{
		Name:      objectMeta.GetName(),
		Namespace: objectMeta.GetNamespace(),
	}, oldObject)

	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	if k8serrors.IsNotFound(err) {
		err = ctrl.SetControllerReference(owns, obj, h.Scheme)
		if err != nil {
			return err
		}
		return h.Client.Create(ctx, obj)
	}

	oldObjectMeta := oldObject.(metav1.ObjectMetaAccessor).GetObjectMeta()
	objectMeta.SetResourceVersion(oldObjectMeta.GetResourceVersion())
	switch object := obj.(type) {
	case *corev1.Service:
		object.Spec.ClusterIP = oldObject.(*corev1.Service).Spec.ClusterIP
	}
	err = ctrl.SetControllerReference(owns, obj, h.Scheme)
	if err != nil {
		return err
	}

	return h.Client.Update(context.TODO(), obj)
}

func (h *HAProxyManager) Init(client client.Client, clientCMD *ClientCMD, scheme *runtime.Scheme) {
	h.Client = client
	h.Scheme = scheme
}

func init() {
	deployer = append(deployer, &HAProxyManager{})
}
