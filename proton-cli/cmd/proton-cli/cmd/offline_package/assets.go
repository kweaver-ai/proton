package offline_package

import (
	_ "embed"
)

//go:install.sh
var installBytes []byte

//go:proton-package.repo.tmpl
var repoTemplateBytes []byte
