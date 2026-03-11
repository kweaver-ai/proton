package common

import (
	"bytes"
	"context"

	"errors"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type K8SClient struct {
	clientSet     *kubernetes.Clientset
	dynamicClient dynamic.Interface
}

func NewK8SClient() (*K8SClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &K8SClient{clientSet: clientset, dynamicClient: dynamicClient}, nil
}

func (k *K8SClient) IsPodReady(namespace, podName string) (bool, error) {
	pod, err := k.clientSet.CoreV1().
		Pods(namespace).
		Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if pod.Status.Phase != corev1.PodRunning {
		return false, nil
	}

	for _, v := range pod.Status.Conditions {
		if v.Type == corev1.ContainersReady && v.Status == corev1.ConditionTrue {
			return true, nil
		}
	}
	return false, nil
}

func (k *K8SClient) CreateJob(job *batchv1.Job) (*batchv1.Job, error) {
	return k.clientSet.BatchV1().Jobs(job.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
}

func (k *K8SClient) DeleteJob(namespace, name string) error {
	pp := metav1.DeletePropagationBackground
	return k.clientSet.BatchV1().Jobs(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{PropagationPolicy: &pp})
}

func (k *K8SClient) ScaleSts(namespace, stsname string, replicas int32) error {
	sc, err := k.clientSet.
		AppsV1().
		StatefulSets(namespace).
		GetScale(context.TODO(), stsname, metav1.GetOptions{})
	if err != nil {
		return err
	}
	sc.Spec.Replicas = replicas
	_, err = k.clientSet.
		AppsV1().
		StatefulSets(namespace).
		UpdateScale(context.TODO(), stsname, sc, metav1.UpdateOptions{})
	return err
}

func (k *K8SClient) ClientSet() *kubernetes.Clientset {
	return k.clientSet
}

func (k *K8SClient) DynamicClient() dynamic.Interface {
	return k.dynamicClient
}

func (k *K8SClient) ExecPod(namespace, podName, containerName string, cmd []string) (stdout bytes.Buffer, stderr bytes.Buffer, err error) {
	execOpt := &corev1.PodExecOptions{
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
		Container: containerName,
		Command:   cmd,
	}

	req := k.clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(execOpt, scheme.ParameterCodec)

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return stdout, stderr, errors.New("NewSPDYExecutor: " + err.Error())
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return stdout, stderr, errors.New("exec.Stream:" + err.Error())
	}

	return
}
