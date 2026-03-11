package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type Mgmt struct {
	Image           string                      `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`
	ImagePullPolicy corev1.PullPolicy           `json:"imagePullPolicy,omitempty" protobuf:"bytes,14,opt,name=imagePullPolicy,casttype=PullPolicy"`
	Conf            MgmtConf                    `json:"conf"`
	Service         *Svc                        `json:"service"`
	Resources       corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
}

type MgmtConf map[string]intstr.IntOrString
