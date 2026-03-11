package v1

import (
	corev1 "k8s.io/api/core/v1"
)

type HAProxy struct {
	Enabled         bool                        `json:"enabled,omitempty"`
	Image           string                      `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`
	ImagePullPolicy corev1.PullPolicy           `json:"imagePullPolicy,omitempty" protobuf:"bytes,14,opt,name=imagePullPolicy,casttype=PullPolicy"`
	Service         *HAProxySvc                 `json:"service"`
	Resources       corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
	NodeAffinity    *corev1.NodeAffinity        `json:"nodeAffinity,omitempty" protobuf:"bytes,1,opt,name=nodeAffinity"`
}

type HAProxySvc struct {
	EnableDualStack  bool `json:"enableDualStack"`
	SingleMasterPort int  `json:"wPort"`
	MultiMasterPort  int  `json:"rPort"`
}
