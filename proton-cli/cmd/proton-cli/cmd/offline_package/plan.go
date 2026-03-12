package offline_package

import (
	"log"

	"github.com/spf13/cobra"
)

func newPlanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "plan [flags]",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Println("Create a plan to building proton-offline-package")
			return nil
		},
	}
	return cmd
}
