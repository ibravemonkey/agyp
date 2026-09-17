package cmd

import (
	"github.com/ibravemonkey/agyp/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information for agys CLI",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("agys version %s\n", version.GetVersionInfo())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
