package controllers

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	mrand "math/rand"
	"time"

	rdsv1 "proton-rds-mariadb-operator/api/v1"

	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReconcileUserSecret(client client.Client, o *rdsv1.RDSMariaDBCluster, scheme *runtime.Scheme) error {
	secretObj := corev1.Secret{}
	err := client.Get(context.TODO(),
		types.NamespacedName{
			Namespace: o.Namespace,
			Name:      o.Spec.SecretName,
		},
		&secretObj,
	)
	if err == nil {
		return nil
	} else if !k8serror.IsNotFound(err) {
		return err
	}

	data := make(map[string][]byte)
	data["username"] = []byte("root")
	data["password"], err = generatePass()
	if err != nil {
		return err
	}
	secretObj = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Spec.SecretName,
			Namespace: o.Namespace,
		},
		Data: data,
		Type: corev1.SecretTypeOpaque,
	}
	err = ctrl.SetControllerReference(o, &secretObj, scheme)
	if err != nil {
		return fmt.Errorf("create Users secret: %v", err)
	}
	err = client.Create(context.TODO(), &secretObj)
	if err != nil {
		return fmt.Errorf("create Users secret: %v", err)
	}
	return nil
}

const (
	passwordMaxLen = 20
	passwordMinLen = 16
	passSymbols    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789"
)

//generatePass generate random password
func generatePass() ([]byte, error) {
	mrand.Seed(time.Now().UnixNano())
	ln := mrand.Intn(passwordMaxLen-passwordMinLen) + passwordMinLen
	b := make([]byte, ln)
	for i := 0; i < ln; i++ {
		randInt, err := rand.Int(rand.Reader, big.NewInt(int64(len(passSymbols))))
		if err != nil {
			return nil, err
		}
		b[i] = passSymbols[randInt.Int64()]
	}

	return b, nil
}
