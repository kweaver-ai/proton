package offline_package

import "github.com/spf13/cobra"

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "offline-package",
	}

	cmd.AddCommand(newPlanCommand())
	cmd.AddCommand(newBuildCommand())
	cmd.AddCommand(newInstallCommand())

	return cmd
}
