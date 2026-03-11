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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	rdsv1 "proton-rds-mariadb-operator/api/v1"
)

// RDSMariaDBClusterReconciler reconciles a RDSMariaDBCluster object
type RDSMariaDBClusterReconciler struct {
	Client         client.Client
	Scheme         *runtime.Scheme
	ClientCMD      *ClientCMD
	Lockers        LockStore
	StorageManager *StorageManager
	Deployer       []Deployer
}

var deployer []Deployer

const version = "1.0.0"

type Deployer interface {
	Deploy(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error
	Clear(ctx context.Context, req ctrl.Request, o *rdsv1.RDSMariaDBCluster) error
	Init(client client.Client, clientCMD *ClientCMD, scheme *runtime.Scheme)
}

func NewReconciler(mgr manager.Manager) (*RDSMariaDBClusterReconciler, error) {
	cmd, err := NewClientCMD()
	if err != nil {
		return &RDSMariaDBClusterReconciler{}, err
	}
	c := mgr.GetClient()
	s := mgr.GetScheme()
	for _, d := range deployer {
		d.Init(c, cmd, s)
	}
	return &RDSMariaDBClusterReconciler{
		Client:    c,
		Scheme:    s,
		ClientCMD: cmd,
		Lockers:   NewLockStore(),
		StorageManager: &StorageManager{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		},
		Deployer: deployer,
	}, nil
}

//+kubebuilder:rbac:groups=rds.proton.aishu.cn,resources=rdsmariadbclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rds.proton.aishu.cn,resources=rdsmariadbclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=rds.proton.aishu.cn,resources=rdsmariadbclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups="";apps;rbac.authorization.k8s.io;batch,resources=pods/exec;persistentvolumes;persistentvolumeclaims;services;configmaps;deployments;statefulsets;statefulsets/scale;secrets;pods;endpoints;jobs;cronjobs;serviceaccounts;clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the RDSMariaDBCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *RDSMariaDBClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// TODO(user): your logic here
	fmt.Println("called at: ", time.Now())
	o := &rdsv1.RDSMariaDBCluster{}

	err := r.Client.Get(ctx, req.NamespacedName, o)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !o.ObjectMeta.DeletionTimestamp.IsZero() {
		finalizers := []string{}
		for _, fnlz := range o.GetFinalizers() {
			switch fnlz {
			case "delete-storage":
				err = r.StorageManager.Clear(ctx, req, o)
				if err != nil && !k8serrors.IsNotFound(err) {
					finalizers = append(finalizers, fnlz)
				}
			case "delete-app":
				for _, d := range r.Deployer {
					err = d.Clear(ctx, req, o)
					if err != nil && !k8serrors.IsNotFound(err) {
						finalizers = append(finalizers, fnlz)
						break
					}
				}
			}
		}
		o.SetFinalizers(finalizers)

		err = r.Client.Update(context.TODO(), o)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		// object is being deleted, no need in further actions
		return ctrl.Result{}, nil
	}

	//if err = rdsv1.Validate(o); err != nil {
	//	log.Error(err, "Verify CR failed: ")
	//	return ctrl.Result{}, nil
	//}

	err = ReconcileUserSecret(r.Client, o, r.Scheme)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	err = r.StorageManager.Deploy(ctx, req, o)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	for _, d := range r.Deployer {
		err = d.Deploy(ctx, req, o)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	o.SetFinalizers([]string{"delete-storage", "delete-app"})
	annotations := o.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if val, ok := annotations["ctrlVersion"]; ok {
		if val != version {
			//TODO. upgrade
			log.Info("Update ctrl.")
		}
	}
	annotations["ctrlVersion"] = version

	// rds 1.x版本迁移过来需要创建账户
	if _, ok := annotations["rdsAlreadyExists"]; ok {
		log.Info("upgrade rds...")
		secretObj := corev1.Secret{}
		pod := &corev1.Pod{}
		err := r.Client.Get(context.TODO(),
			types.NamespacedName{
				Namespace: o.Namespace,
				Name:      o.Spec.SecretName,
			},
			&secretObj,
		)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		err = r.Client.Get(context.TODO(), types.NamespacedName{
			Namespace: o.GetNamespace(),
			Name:      o.GetName() + "-mariadb-0",
		}, pod)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		cmds := []string{
			fmt.Sprintf("mariadb -u%s -p%s -e \"GRANT ALL ON *.* TO 'safety'@'127.0.0.1' IDENTIFIED BY 'fakepassword' WITH GRANT OPTION;\"",
				string(secretObj.Data["username"]),
				string(secretObj.Data["password"]),
			),
			fmt.Sprintf("mariadb -u%s -p%s -e \"GRANT ALL ON *.* TO 'safety'@'localhost' IDENTIFIED BY 'fakepassword' WITH GRANT OPTION;\"",
				string(secretObj.Data["username"]),
				string(secretObj.Data["password"]),
			),
			fmt.Sprintf("mariadb -u%s -p%s -e \"GRANT ALL ON *.* TO '%s'@'%%' IDENTIFIED BY '%s' WITH GRANT OPTION;\"",
				string(secretObj.Data["username"]),
				string(secretObj.Data["password"]),
				string(secretObj.Data["username"]),
				string(secretObj.Data["password"]),
			),
			fmt.Sprintf("mariadb -u%s -p%s -e \"CREATE USER IF NOT EXISTS 'monitor'@'%%' IDENTIFIED BY 'fakepassword';\"",
				string(secretObj.Data["username"]),
				string(secretObj.Data["password"]),
			),
		}
		for {
			if isRunning, _ := r.ClientCMD.IsPodRunning(pod.GetNamespace(), pod.GetName()); isRunning {
				for _, cmd := range cmds {
					stderrBuf := &bytes.Buffer{}
					err = r.ClientCMD.Exec(pod, "mariadb", []string{"/bin/sh", "-c", cmd}, nil, nil, stderrBuf, false)
					if err != nil {
						return ctrl.Result{Requeue: true}, err
					}
					if stderrBuf.Len() != 0 {
						return ctrl.Result{Requeue: true}, err
					}
				}
				break
			}
		}
		delete(annotations, "rdsAlreadyExists")
	}

	o.SetAnnotations(annotations)
	err = r.Client.Update(ctx, o)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	o.Status.LastAppliedConfiguration = *o.Spec.DeepCopy()
	err = r.Client.Status().Update(ctx, o)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RDSMariaDBClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rdsv1.RDSMariaDBCluster{}).
		WithEventFilter(ReconcilePredicate{}).
		Complete(r)
}
