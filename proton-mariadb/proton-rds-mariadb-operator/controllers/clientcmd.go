package controllers

import (
	"bufio"
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

type ClientCMD struct {
	client     corev1client.CoreV1Interface
	restconfig *restclient.Config
}

func NewClientCMD() (*ClientCMD, error) {
	// Instantiate loader for kubeconfig file.
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{
			Timeout: "10s",
		},
	)

	// Get a rest.Config from the kubeconfig file.  This will be passed into all
	// the client objects we create.
	restconfig, err := kubeconfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	// Create a Kubernetes core/v1 client.
	cl, err := corev1client.NewForConfig(restconfig)
	if err != nil {
		return nil, err
	}

	return &ClientCMD{
		client:     cl,
		restconfig: restconfig,
	}, nil
}

func (c *ClientCMD) PodLogs(namespace, podName string, opts *corev1.PodLogOptions) ([]string, error) {
	logs, err := c.client.Pods(namespace).GetLogs(podName, opts).Stream(context.TODO())
	if err != nil {
		//return nil, errors.Wrap(err, "get pod logs stream")
		return nil, err
	}

	defer logs.Close()

	logArr := make([]string, 0)
	sc := bufio.NewScanner(logs)
	for sc.Scan() {
		logArr = append(logArr, sc.Text())
	}
	//return logArr, errors.Wrap(sc.Err(), "reading logs stream")
	return logArr, sc.Err()
}

func (c *ClientCMD) IsPodRunning(namespace, podName string) (bool, error) {
	pod, err := c.client.Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
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

func (c *ClientCMD) IsPodReady(namespace, podName string) (bool, error) {
	pod, err := c.client.Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
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

func (c *ClientCMD) Exec(pod *corev1.Pod, containerName string, command []string, stdin io.Reader, stdout, stderr io.Writer, tty bool) error {
	// Prepare the API URL used to execute another process within the Pod.  In
	// this case, we'll run a remote shell.
	req := c.client.RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     stdin != nil,
			Stdout:    stdout != nil,
			Stderr:    stderr != nil,
			TTY:       tty,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.restconfig, "POST", req.URL())
	if err != nil {
		return err
	}

	// Connect this process' std{in,out,err} to the remote shell process.
	return exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	})
}

func (c *ClientCMD) REST() restclient.Interface {
	return c.client.RESTClient()
}

func (c *ClientCMD) LabelPod(namespace, podName, labelKey, labelValue string) error {
	pod, err := c.client.Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	labels := pod.GetLabels()
	labels[labelKey] = labelValue
	pod.SetLabels(labels)
	_, err = c.client.Pods(namespace).Update(context.TODO(), pod, metav1.UpdateOptions{})
	return err
}
