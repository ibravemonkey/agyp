package cmd

import (
	"fmt"
	"os"

	"github.com/ibravemonkey/agyp/pkg/updater"
	"github.com/ibravemonkey/agyp/pkg/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "agyp",
	Short:         "agyp (Antigravity Profiles) manages isolated account profiles and real-time multi-account quota tracking",
	Long: `agyp isolates multi-account profiles across the Google Antigravity ecosystem (CLI, IDE, GUI, Remote)
and provides native, real-time profile quota tracking (5H & Weekly) and lifecycle hooks for Herdr multi-agent workspaces.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		updater.NotifyIfRecentlyUpdated(cmd.Name())
		updater.MaybeTriggerBackgroundUpdate(cmd.Name())
	},
}
// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	rootCmd.Version = version.GetVersionInfo()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
