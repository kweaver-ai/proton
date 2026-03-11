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
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	rdsv1 "proton-rds-mariadb-operator/api/v1"

	corev1 "k8s.io/api/core/v1"
)

type Watcher struct {
	Client    client.Client
	ClientCMD *ClientCMD
	Scheme    *runtime.Scheme
	Running   bool
	CancelMap map[string]context.CancelFunc
}

func (w *Watcher) Clear(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	k := o.GetNamespace() + o.GetName()
	if _, ok := w.CancelMap[k]; ok {
		w.CancelMap[k]()
		delete(w.CancelMap, k)
	}
	return nil
}

func (w *Watcher) Deploy(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	k := o.GetNamespace() + o.GetName()
	if _, ok := w.CancelMap[k]; !ok {
		ctx2, cancel := context.WithCancel(context.TODO())
		w.CancelMap[k] = cancel
		go w.EnsureService(ctx2, *o)
	}

	return nil
}

func (w *Watcher) constructLBService(o *rdsv1.RDSMariaDBCluster) *corev1.Service {
	if o.Spec.Mariadb.Service.EnableDualStack {
		var ipPolicy corev1.IPFamilyPolicyType = corev1.IPFamilyPolicyRequireDualStack
		return &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Servivce",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.GetName() + "-mariadb-master",
				Namespace: o.GetNamespace(),
			},
			Spec: corev1.ServiceSpec{
				IPFamilies: []corev1.IPFamily{
					corev1.IPv6Protocol,
					corev1.IPv4Protocol,
				},
				IPFamilyPolicy: &ipPolicy,
				Selector: map[string]string{
					"app":     o.GetName() + "-mariadb",
					"db-role": "master",
				},
				Ports: []corev1.ServicePort{
					{
						Name: "single-master-port",
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
			Name:      o.GetName() + "-mariadb-master",
			Namespace: o.GetNamespace(),
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app":     o.GetName() + "-mariadb",
				"db-role": "master",
			},
			Ports: []corev1.ServicePort{
				{
					Name: "single-master-port",
					Port: int32(o.Spec.Mariadb.Service.Port),
					TargetPort: intstr.IntOrString{
						IntVal: 3306,
					},
				},
			},
		},
	}

}

func (w *Watcher) createOrUpdate(ctx context.Context, owns, obj client.Object) error {
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

	err := w.Client.Get(context.Background(), types.NamespacedName{
		Name:      objectMeta.GetName(),
		Namespace: objectMeta.GetNamespace(),
	}, oldObject)

	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	if k8serrors.IsNotFound(err) {
		err = ctrl.SetControllerReference(owns, obj, w.Scheme)
		if err != nil {
			return err
		}
		return w.Client.Create(ctx, obj)
	}

	oldObjectMeta := oldObject.(metav1.ObjectMetaAccessor).GetObjectMeta()
	objectMeta.SetResourceVersion(oldObjectMeta.GetResourceVersion())
	switch object := obj.(type) {
	case *corev1.Service:
		object.Spec.ClusterIP = oldObject.(*corev1.Service).Spec.ClusterIP
	}
	err = ctrl.SetControllerReference(owns, obj, w.Scheme)
	if err != nil {
		return err
	}

	return w.Client.Update(context.TODO(), obj)
}

func (w *Watcher) Init(client client.Client, clientCMD *ClientCMD, scheme *runtime.Scheme) {
	w.Client = client
	w.Scheme = scheme
	w.ClientCMD = clientCMD
	w.CancelMap = make(map[string]context.CancelFunc)
}

func init() {
	deployer = append(deployer, &Watcher{})
}

func (w *Watcher) EnsureService(ctx context.Context, o rdsv1.RDSMariaDBCluster) {
	log := log.FromContext(ctx)
	svc := w.constructLBService(&o)
	w.createOrUpdate(ctx, &o, svc)
	tiker := time.NewTicker(2 * time.Second)
	master := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-tiker.C:
			endpoint, err := w.ClientCMD.client.Endpoints(o.GetNamespace()).Get(ctx, o.GetName()+"-mariadb-master", metav1.GetOptions{})
			if err != nil {
				log.Error(err, "")
				continue
			}
			if len(endpoint.Subsets) != 1 || len(endpoint.Subsets[0].Addresses) != 1 {
				opts := []client.ListOption{
					client.InNamespace(o.GetNamespace()),
					client.MatchingLabels{
						"app": o.GetName() + "-mariadb",
					},
				}
				pods := &corev1.PodList{}
				err := w.Client.List(ctx, pods, opts...)
				if err != nil {
					log.Error(err, "")
					continue
				}
				for _, pod := range pods.Items {
					err := w.ClientCMD.LabelPod(o.GetNamespace(), pod.GetName(), "db-role", "backup")
					if err != nil {
						log.Error(err, pod.GetName()+" failed to switch to backup")
					}
				}
				for _, pod := range pods.Items {
					isReady := false
					for _, v := range pod.Status.Conditions {
						if v.Type == corev1.ContainersReady && v.Status == corev1.ConditionTrue {
							isReady = true
							break
						}
					}
					if isReady || len(pods.Items) == 1 {
						err = w.ClientCMD.LabelPod(o.GetNamespace(), pod.GetName(), "db-role", "master")
						if err != nil {
							log.Error(err, pod.GetName()+" failed to switch to master")
						} else {
							log.Info(pod.GetName() + " switch to master")
							time.Sleep(15 * time.Second)
							if master != "" && master != pod.GetName() {
								p, err := w.ClientCMD.client.Pods(pod.GetNamespace()).Get(ctx, master, metav1.GetOptions{})
								if err != nil {
									log.Error(err, "failed to get old master")
								} else {
									w.ClientCMD.Exec(p, "mariadb", []string{"/bin/sh", "-c", "mariadb-admin -u $MYSQL_ROOT_USER -p$MYSQL_ROOT_PASSWORD shutdown"}, nil, nil, nil, false)
									w.ClientCMD.client.Pods(o.GetNamespace()).Delete(context.TODO(), master, metav1.DeleteOptions{})
								}
							}
							master = pod.GetName()
							break
						}
					}
				}
			} else {
				master = endpoint.Subsets[0].Addresses[0].TargetRef.Name
			}

		}
	}
}
