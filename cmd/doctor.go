package cmd

import (
	"context"
	"io"

	"github.com/ibravemonkey/agyp/internal/doctor"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Aliases: []string{"health"},
	Short:   "Perform diagnostic health checks on agyp, agy CLI, profiles, and Herdr integration",
	Long:    `Inspect system binaries, OAuth credentials, macOS Keychain integrity, model discovery, and Herdr hooks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDoctorWithWriter(cmd.Context(), cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctorWithWriter(ctx context.Context, w io.Writer) error {
	svc := doctor.NewService(nil, nil)
	return svc.Execute(ctx, w)
}

func runDoctor(ctx context.Context) error {
	return runDoctorWithWriter(ctx, doctorCmd.OutOrStdout())
}
