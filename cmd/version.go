package cmd

import (
	"github.com/ibravemonkey/agyp/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information for agyp CLI",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("agyp version %s\n", version.GetVersionInfo())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
