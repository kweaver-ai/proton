package v1

import "devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/client/rest"

const (
	// GroupName is the group name use in this package
	GroupName = "slb"
	// GroupVersion is the group version use in this package
	GroupVersion = "v1"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = rest.GroupVersion{Group: GroupName, Version: GroupVersion}

// DefaultPort is the default port
const DefaultPort = 9202
