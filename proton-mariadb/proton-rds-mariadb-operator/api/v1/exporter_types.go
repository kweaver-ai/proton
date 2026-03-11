package v1

import corev1 "k8s.io/api/core/v1"

type Exporter struct {
	Image           string            `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty" protobuf:"bytes,14,opt,name=imagePullPolicy,casttype=PullPolicy"`
}
