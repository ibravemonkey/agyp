package cmd

import (
	"context"
	"time"

	"github.com/ibravemonkey/agyp/pkg/profile"
	"github.com/spf13/cobra"
)

var bgQuotaCmd = &cobra.Command{
	Use:    "__bg-quota <profile>",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		_, err := profile.FetchQuota(ctx, profileName)
		return err
	},
}

func init() {
	profile.SetHookProcess(true)
	rootCmd.AddCommand(bgQuotaCmd)
}
