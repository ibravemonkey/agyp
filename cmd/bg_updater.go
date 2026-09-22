package cmd

import (
	"github.com/ibravemonkey/agyp/pkg/updater"
	"github.com/spf13/cobra"
)

var bgUpdaterCmd = &cobra.Command{
	Use:    "__bg-updater",
	Short:  "Internal background updater worker",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return updater.RunBackgroundWorker()
	},
}

func init() {
	rootCmd.AddCommand(bgUpdaterCmd)
}
