package offline_package

import (
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Manifest struct {
	meta.TypeMeta
	meta.ObjectMeta `json:"metadata,omitzero"`

	Spec ManifestSpec `json:"spec,omitzero"`
}

type ManifestSpec struct {
	Architecture string `json:"architecture,omitzero"`

	Components Component `json:"components,omitzero"`

	Binaries []Artifact `json:"binaries,omitzero"`

	Charts []Artifact `json:"charts,omitzero"`

	Images []Artifact `json:"images,omitzero"`

	RPMs []Artifact `json:"rpms,omitzero"`
}

type Component struct {
	Name string `json:"name,omitzero"`

	Version string `json:"version,omitzero"`
}

type Artifact struct {
	Name string `json:"name,omitzero"`

	Source
}

type Source struct {
	HTTP *HTTPSource `json:"http,omitzero"`

	OCI *OCISource `json:"oci,omitzero"`
}

type HTTPSource struct {
	URL string `json:"url,omitzero"`

	Format string `json:"format,omitzero"`

	Path string `json:"path,omitzero"`
}

type OCISource struct {
	Reference string `json:"reference,omitzero"`
}
