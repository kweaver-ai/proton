package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type Mariadb struct {
	Image           string                      `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`
	ImagePullPolicy corev1.PullPolicy           `json:"imagePullPolicy,omitempty" protobuf:"bytes,14,opt,name=imagePullPolicy,casttype=PullPolicy"`
	Conf            MariadbConf                 `json:"conf"`
	Service         *Svc                        `json:"service"`
	Storage         *Storage                    `json:"storage"`
	NodeAffinity    *corev1.NodeAffinity        `json:"nodeAffinity,omitempty" protobuf:"bytes,1,opt,name=nodeAffinity"`
	Resources       corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
	Logrotate       Logrotate                   `json:"logrotate,omitempty"`
}

type MariadbConf map[string]intstr.IntOrString

type Logrotate struct {
	Schedule string `json:"schedule,omitempty"`
}
