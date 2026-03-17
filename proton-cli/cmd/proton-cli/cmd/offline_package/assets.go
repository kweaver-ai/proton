package offline_package

import _ "embed"

var (
	//go:embed manifest.yaml
	manifestBytes []byte

	//go:embed install.sh
	scriptInstallBytes []byte

	//go:embed proton.repo.tmpl
	templateProtonRepoBytes []byte
)
