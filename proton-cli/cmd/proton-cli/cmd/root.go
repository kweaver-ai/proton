/*
Copyright © 2022 Jimmy.li@aishu.cn
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/cmd/proton-cli/cmd/backup"
	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/cmd/proton-cli/cmd/images"
	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/cmd/proton-cli/cmd/offline_package"
	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/cmd/proton-cli/cmd/recover"
	"devops.aishu.cn/AISHUDevOps/ICT/_git/proton-opensource.git/proton-cli/v3/pkg/core/global"
)

const (
	flagServicePackage      = "service-package"
	flagServicePackageECeph = "service-package-eceph"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:                "proton-cli",
	Short:              "proton cluster deploy tool",
	CompletionOptions:  cobra.CompletionOptions{HiddenDefaultCmd: true},
	DisableSuggestions: false,
	SilenceUsage:       true,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd.AddCommand(backup.BackupCmd)
	rootCmd.AddCommand(recover.RecoverCmd)
	rootCmd.AddCommand(images.SetImageCmd())
	rootCmd.AddCommand(K8SCmd())
	rootCmd.AddCommand(newAlphaCmd())
	rootCmd.AddCommand(offline_package.NewCommand())

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&global.LoggerLevel,
		"log-level",
		"l",
		global.LoggerLevel,
		"set log level(info,debug,error)")
	rootCmd.PersistentFlags().StringVarP(&global.ServicePackage, flagServicePackage, "s", global.ServicePackage, `path to a "service-package" directory`)
	rootCmd.PersistentFlags().StringVar(&global.ServicePackageECeph, flagServicePackageECeph, global.ServicePackageECeph, `path to a "service-package-eceph" directory`)
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.proton-cli.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().BoolVarP(&global.ComponentManageDirectConnect,
		"cm-direct",
		"",
		false,
		"component manage direct connect")
}
