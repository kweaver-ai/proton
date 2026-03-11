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

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rdsv1 "proton-rds-mariadb-operator/api/v1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

type logrotateManager struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func (l *logrotateManager) Clear(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	return nil
}

func (l *logrotateManager) Deploy(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error {
	apps := l.constructCronjobs(o)
	for _, app := range apps {
		err := l.createOrUpdate(ctx, o, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *logrotateManager) constructCronjobs(o *rdsv1.RDSMariaDBCluster) []*batchv1.CronJob {
	cronjobs := []*batchv1.CronJob{}
	if o.Spec.Mariadb.Storage.StorageClassName != "" {
		return cronjobs
	}
	var i int32 = 0
	var backOffLimit int32 = 1
	schedule := o.Spec.Mariadb.Logrotate.Schedule
	if schedule == "" {
		schedule = "0 18 * * *"
	}
	for i < *o.Spec.Replicas {
		cronjob := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.GetName() + "-logrotate-" + strconv.Itoa(int(i)),
				Namespace: o.GetNamespace(),
			},
			Spec: batchv1.CronJobSpec{
				Schedule: schedule,
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						BackoffLimit: &backOffLimit,
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Volumes: []corev1.Volume{
									{
										Name: "mariadb",
										VolumeSource: corev1.VolumeSource{
											HostPath: &corev1.HostPathVolumeSource{
												Path: o.Spec.Mariadb.Storage.VolumeSpec[i].Path,
											},
										},
									},
								},
								RestartPolicy: "OnFailure",
								Containers: []corev1.Container{
									{
										Name:  "backup",
										Image: o.Spec.Mariadb.Image,
										Env: []corev1.EnvVar{
											{
												Name:  "MYSQL_HOST",
												Value: o.GetName() + "-mariadb-" + strconv.Itoa(int(i)) + "." + o.GetName() + "-mariadb",
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
										Command: []string{
											"/bin/bash",
											"-ecx",
											fmt.Sprintf(
												"logrotate(){\n" +
													"    logName=$1\n" +
													"    maxSize=$2\n" +
													"    rotate=$3\n" +
													"    flush=\"$4\"\n" +
													"    cd /var/log/mysql\n" +
													"    size=`stat -c %%s $logName`\n" +
													"    if [[ $size -ge $maxSize ]]\n" +
													"    then\n" +
													"        n=`ls $logName* | wc -l` \n" +
													"        for file in `ls $logName* | sort -r`\n" +
													"        do\n" +
													"            if [[ $n -ge $rotate ]]\n" +
													"            then\n" +
													"                rm -rf $file\n" +
													"            else\n" +
													"                mv $file $logName.$n\n" +
													"            fi \n" +
													"            n=$[n-1]\n" +
													"        done\n" +
													"        mariadb -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_ROOT_USER -p$MYSQL_ROOT_PASSWORD -e \"$flush\" \n" +
													"    fi\n" +
													"}\n" +
													"logrotate mysql-error.log $[1024*1024*100] 5 'flush error logs' \n" +
													"logrotate mysql-slow.log $[1024*1024*100] 5 'flush slow logs' \n",
											),
										},
										VolumeMounts: []corev1.VolumeMount{
											{
												Name:      "mariadb",
												MountPath: "/var/log/mysql",
											},
										},
									},
								},
								NodeSelector: map[string]string{
									"kubernetes.io/hostname": o.Spec.Mariadb.Storage.VolumeSpec[i].Host,
								},
							},
						},
					},
				},
			},
		}
		cronjobs = append(cronjobs, cronjob)
		i++
	}
	return cronjobs
}

func (l *logrotateManager) createOrUpdate(ctx context.Context, owns, obj client.Object) error {
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

	err := l.Client.Get(context.Background(), types.NamespacedName{
		Name:      objectMeta.GetName(),
		Namespace: objectMeta.GetNamespace(),
	}, oldObject)

	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	if k8serrors.IsNotFound(err) {
		err = ctrl.SetControllerReference(owns, obj, l.Scheme)
		if err != nil {
			return err
		}
		return l.Client.Create(ctx, obj)
	}

	oldObjectMeta := oldObject.(metav1.ObjectMetaAccessor).GetObjectMeta()
	objectMeta.SetResourceVersion(oldObjectMeta.GetResourceVersion())
	switch object := obj.(type) {
	case *corev1.Service:
		object.Spec.ClusterIP = oldObject.(*corev1.Service).Spec.ClusterIP
	}
	err = ctrl.SetControllerReference(owns, obj, l.Scheme)
	if err != nil {
		return err
	}

	return l.Client.Update(context.TODO(), obj)
}

func (l *logrotateManager) Init(client client.Client, clientCMD *ClientCMD, scheme *runtime.Scheme) {
	l.Client = client
	l.Scheme = scheme
}

func init() {
	deployer = append(deployer, &logrotateManager{})
}
