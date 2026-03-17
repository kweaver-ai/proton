package offline_package

import (
	"github.com/spf13/cobra"
)

func newPlanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "plan",
		Run: func(cmd *cobra.Command, _ []string) { cmd.OutOrStdout().Write(manifestBytes) },
	}
	return cmd
}
