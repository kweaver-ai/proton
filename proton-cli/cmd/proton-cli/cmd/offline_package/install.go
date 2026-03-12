package offline_package

import (
	"log"

	"github.com/spf13/cobra"
)

func newInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "install PACKAGE",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Println("Install proton-offline-package")
			return nil
		},
	}
	return cmd
}
