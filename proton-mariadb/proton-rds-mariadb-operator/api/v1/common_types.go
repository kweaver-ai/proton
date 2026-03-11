package v1

type Volume struct {
	Host string `json:"host"`
	Path string `json:"path"`
}

type Svc struct {
	EnableDualStack bool `json:"enableDualStack"`
	Port            int  `json:"port"`
}

type Storage struct {
	Capacity         string   `json:"capacity"`
	StorageClassName string   `json:"storageClassName"`
	VolumeSpec       []Volume `json:"volumeSpec" `
}
