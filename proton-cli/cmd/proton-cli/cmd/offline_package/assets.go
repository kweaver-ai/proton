package offline_package

import _ "embed"

var (
	//go:embed install.sh
	scriptInstallBytes []byte

	//go:embed proton.repo.tmpl
	templateProtonRepoBytes []byte
)
