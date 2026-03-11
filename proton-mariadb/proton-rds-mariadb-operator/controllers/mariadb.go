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
	"strconv"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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

type MariadbManager struct {
	Client    client.Client
	ClientCMD *ClientCMD
	Scheme    *runtime.Scheme
}

func (m *MariadbManager) Clear(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	return nil
}

func (m *MariadbManager) Deploy(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	svc := m.constructService(o)
	err := m.createOrUpdate(ctx, o, svc, true)
	if err != nil {
		return err
	}

	lb_svc := m.constructLBService(o)
	err = m.createOrUpdate(ctx, o, lb_svc, true)
	if err != nil {
		return err
	}

	cm := m.constructCm(o)
	err = m.createOrUpdate(ctx, o, cm, true)
	if err != nil {
		return err
	}

	app := m.constructSts(o)
	err = m.createOrUpdate(ctx, o, app, true)
	if err != nil {
		return err
	}

	if o.Status.LastAppliedConfiguration.Replicas != nil {
		if *o.Spec.Replicas > 1 && *o.Status.LastAppliedConfiguration.Replicas == 1 {
			err = m.RestartPod(o.GetName()+"-mariadb-0", o.GetNamespace())
		}
	}

	if o.Status.LastAppliedConfiguration.Mariadb != nil && o.Status.LastAppliedConfiguration.Mariadb.Conf != nil {
		go m.ModifyCnf(*o.DeepCopy())
	}

	return err
}

func (m *MariadbManager) constructCm(o *rdsv1.RDSMariaDBCluster) *corev1.ConfigMap {
	tmpl := map[string]interface{}{
		"max_connections":                    10000,
		"max_prepared_stmt_count":            65536,
		"local_infile":                       "OFF",
		"sql_mode":                           "ONLY_FULL_GROUP_BY,STRICT_ALL_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION",
		"wait_timeout":                       3600,
		"interactive_timeout":                3600,
		"idle_transaction_timeout":           60,
		"lower_case_table_names":             1,
		"innodb_buffer_pool_size":            "8G",
		"innodb_page_size":                   "16k",
		"innodb_max_dirty_pages_pct":         50,
		"innodb_io_capacity":                 5000,
		"innodb_io_capacity_max":             10000,
		"innodb_adaptive_hash_index":         "OFF",
		"innodb_flush_log_at_trx_commit":     2,
		"log_bin":                            "mysql-bin",
		"log_slave_updates":                  1,
		"binlog_format":                      "ROW",
		"sync_binlog":                        0,
		"expire_logs_days":                   7,
		"max_binlog_size":                    "500M",
		"binlog_cache_size":                  "128k",
		"#general_log":                       1,
		"general_log_file":                   "/var/lib/mysql/mysql-general.log",
		"log_error":                          "/var/lib/mysql/mysql-error.log",
		"slow_query_log":                     "true",
		"slow_query_log_file":                "/var/lib/mysql/mysql-slow.log",
		"long_query_time":                    1,
		"log_queries_not_using_indexes":      "OFF",
		"min_examined_row_limit":             0,
		"wsrep_on":                           "ON",
		"wsrep_auto_increment_control":       "ON",
		"wsrep_slave_threads":                16,
		"bind_address":                       "*",
		"wsrep_provider_options":             "\"gmcast.listen_addr=tcp://[::]:4567;ist.recv_bind=[::]:4568;gcs.fc_limit=80;gcs.fc_factor=0.8;gcache.size=5G;cert.optimistic_pa=NO\"",
		"wsrep_sync_wait":                    1,
		"wsrep_retry_autocommit":             10,
		"thread_handling":                    "pool-of-threads",
		"thread_pool_max_threads":            2000,
		"thread_pool_oversubscribe":          3,
		"extra_port":                         3310,
		"extra_max_connections":              8,
		"event_scheduler":                    "ON",
		"datadir":                            "/var/lib/mysql",
		"pid_file":                           "/var/lib/mysql/mysql.pid",
		"slave_connections_needed_for_purge": 0,
	}

	if *o.Spec.Replicas <= 1 {
		tmpl["wsrep_on"] = "OFF"
		tmpl["wsrep_sync_wait"] = 0
		tmpl["innodb_flush_log_at_trx_commit"] = 1
		tmpl["sync_binlog"] = 1
	}

	for k, v := range o.Spec.Mariadb.Conf {
		switch v.Type {
		case intstr.Int:
			tmpl[k] = v.IntVal
		case intstr.String:
			tmpl[k] = v.StrVal
		}
	}

	cnf := "[mysqld]\n"
	for k, v := range tmpl {
		switch value := v.(type) {
		case string:
			cnf = cnf + fmt.Sprintf("%s = %s\n", k, value)
		case int, int32:
			cnf = cnf + fmt.Sprintf("%s = %d\n", k, value)
		}

	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mariadb-config",
			Namespace: o.GetNamespace(),
		},
		Data: map[string]string{
			"mycustom.cnf": cnf,
		},
	}

}

func (m *MariadbManager) constructSts(o *rdsv1.RDSMariaDBCluster) *appsv1.StatefulSet {
	var tmp int64 = 1001
	quantity, _ := resource.ParseQuantity(o.Spec.Mariadb.Storage.Capacity)
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Statefulset",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.GetName() + "-mariadb",
			Namespace: o.GetNamespace(),
			Labels: map[string]string{
				"app": o.GetName() + "-mariadb",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         o.GetName() + "-mariadb",
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Replicas:            o.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": o.Name + "-mariadb",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: o.Name + "-mariadb",
					Labels: map[string]string{
						"app": o.Name + "-mariadb",
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
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: &tmp,
					},
					Containers: []corev1.Container{
						{
							Name:            "mariadb",
							Image:           o.Spec.Mariadb.Image,
							ImagePullPolicy: o.Spec.Mariadb.ImagePullPolicy,
							Ports: []corev1.ContainerPort{
								{
									Name:          "mariadb",
									ContainerPort: 3306,
								},
								{
									Name:          "sst",
									ContainerPort: 4444,
								},
								{
									Name:          "replication",
									ContainerPort: 4567,
								},
								{
									Name:          "ist",
									ContainerPort: 4568,
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
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name:  "DISCOVERY_SERVICE",
									Value: o.GetName() + "-etcd-cluster:2379",
								},
								{
									Name:  "SERVICE_NAME",
									Value: o.GetName() + "-mariadb",
								},
								{
									Name:  "STATEFULSET_NAME",
									Value: o.GetName() + "-mariadb",
								},
								{
									Name:  "CLUSTER_SIZE",
									Value: strconv.Itoa(int(*o.Spec.Replicas)),
								},
								{
									Name:  "CLUSTER_NAME",
									Value: "proton-rds",
								},
								{
									Name:  "TRACE",
									Value: "1",
								},
								{
									Name:  "SLEEP",
									Value: "10",
								},
								{
									Name:  "TZ",
									Value: "Asia/Shanghai",
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/healthcheck.sh",
											"--liveness",
										},
									},
								},
								InitialDelaySeconds: 60,
								PeriodSeconds:       5,
								TimeoutSeconds:      2,
								FailureThreshold:    50,
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/healthcheck.sh",
											"--readiness",
										},
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       2,
								TimeoutSeconds:      60,
								FailureThreshold:    10,
							},
							Resources: o.Spec.Mariadb.Resources,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "mariadb-config",
									MountPath: "/etc/mysql/mariadb.conf.d/",
								},
								{
									Name:      "mariadb-datadir",
									MountPath: "/var/lib/mysql",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "mariadb-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "mariadb-config",
									},
									Items: []corev1.KeyToPath{
										{
											Key:  "mycustom.cnf",
											Path: "mycustom.cnf",
										},
									},
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "mariadb-datadir",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: &o.Spec.Mariadb.Storage.StorageClassName,
						AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: quantity,
							},
						},
					},
				},
			},
		},
	}
}

func (m *MariadbManager) constructService(o *rdsv1.RDSMariaDBCluster) *corev1.Service {
	if o.Spec.Mariadb.Service.EnableDualStack {
		var ipPolicy corev1.IPFamilyPolicyType = corev1.IPFamilyPolicyRequireDualStack
		return &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Servivce",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.GetName() + "-mariadb",
				Namespace: o.GetNamespace(),
			},
			Spec: corev1.ServiceSpec{
				IPFamilies: []corev1.IPFamily{
					corev1.IPv6Protocol,
					corev1.IPv4Protocol,
				},
				IPFamilyPolicy:           &ipPolicy,
				PublishNotReadyAddresses: true,
				Selector:                 map[string]string{"app": o.GetName() + "-mariadb"},
				ClusterIP:                "None",
				Ports: []corev1.ServicePort{
					{
						Name: "mariadb",
						Port: 3306,
						TargetPort: intstr.IntOrString{
							IntVal: 3306,
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
			Name:      o.GetName() + "-mariadb",
			Namespace: o.GetNamespace(),
		},
		Spec: corev1.ServiceSpec{
			PublishNotReadyAddresses: true,
			Selector:                 map[string]string{"app": o.GetName() + "-mariadb"},
			ClusterIP:                "None",
			Ports: []corev1.ServicePort{
				{
					Name: "mariadb",
					Port: 3306,
					TargetPort: intstr.IntOrString{
						IntVal: 3306,
					},
				},
			},
		},
	}

}

func (m *MariadbManager) constructLBService(o *rdsv1.RDSMariaDBCluster) *corev1.Service {
	if o.Spec.Mariadb.Service.EnableDualStack {
		var ipPolicy corev1.IPFamilyPolicyType = corev1.IPFamilyPolicyRequireDualStack
		return &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Servivce",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.GetName() + "-mariadb-cluster",
				Namespace: o.GetNamespace(),
			},
			Spec: corev1.ServiceSpec{
				IPFamilies: []corev1.IPFamily{
					corev1.IPv6Protocol,
					corev1.IPv4Protocol,
				},
				IPFamilyPolicy: &ipPolicy,
				Selector:       map[string]string{"app": o.GetName() + "-mariadb"},
				Ports: []corev1.ServicePort{
					{
						Name: "mariadb",
						Port: int32(o.Spec.Mariadb.Service.Port),
						TargetPort: intstr.IntOrString{
							IntVal: 3306,
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
			Name:      o.GetName() + "-mariadb-cluster",
			Namespace: o.GetNamespace(),
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": o.GetName() + "-mariadb"},
			Ports: []corev1.ServicePort{
				{
					Name: "mariadb",
					Port: int32(o.Spec.Mariadb.Service.Port),
					TargetPort: intstr.IntOrString{
						IntVal: 3306,
					},
				},
			},
		},
	}

}

func (m *MariadbManager) createOrUpdate(ctx context.Context, owns, obj client.Object, setController bool) error {
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

func (m *MariadbManager) RestartPod(podName, namespace string) error {
	pod := &corev1.Pod{}
	err := m.Client.Get(context.TODO(), types.NamespacedName{
		Name:      podName,
		Namespace: namespace,
	}, pod)
	if k8serrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	err = m.Client.Delete(context.TODO(), pod)
	return err
}

func (m *MariadbManager) Init(client client.Client, clientCMD *ClientCMD, scheme *runtime.Scheme) {
	m.Client = client
	m.ClientCMD = clientCMD
	m.Scheme = scheme
}

func (m *MariadbManager) ModifyCnf(o rdsv1.RDSMariaDBCluster) {
	var i int32 = 0
	secretObj := corev1.Secret{}
	pod := &corev1.Pod{}
	err := m.Client.Get(context.TODO(),
		types.NamespacedName{
			Namespace: o.Namespace,
			Name:      o.Spec.SecretName,
		},
		&secretObj,
	)
	if err != nil {
		return
	}
	for i < *o.Spec.Replicas {
		err := m.Client.Get(context.TODO(), types.NamespacedName{
			Namespace: o.GetNamespace(),
			Name:      o.GetName() + "-mariadb-" + strconv.Itoa(int(i)),
		}, pod)
		if err != nil {
			continue
		}
		if isRunning, _ := m.ClientCMD.IsPodRunning(pod.GetNamespace(), pod.GetName()); isRunning {
			for k, v := range o.Spec.Mariadb.Conf {
				cmd := fmt.Sprintf("mariadb -u%s -p%s -e \"SET GLOBAL %s=%s;\"",
					string(secretObj.Data["username"]),
					string(secretObj.Data["password"]),
					k,
					v.StrVal,
				)
				if v.Type == intstr.Int {
					cmd = fmt.Sprintf("mariadb -u%s -p%s -e \"SET GLOBAL %s=%d;\"",
						string(secretObj.Data["username"]),
						string(secretObj.Data["password"]),
						k,
						v.IntVal,
					)
				}
				stderrBuf := &bytes.Buffer{}
				m.ClientCMD.Exec(pod, "mariadb", []string{"/bin/sh", "-c", cmd}, nil, nil, stderrBuf, false)
			}

		}
		i++
	}
}

func init() {
	deployer = append(deployer, &MariadbManager{})
}
