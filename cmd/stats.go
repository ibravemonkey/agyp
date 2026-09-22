package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ibravemonkey/agyp/pkg/profile"
	"github.com/spf13/cobra"
)

var (
	statsDays    int
	statsProfile string
	statsJSON    bool
)

var statsCmd = &cobra.Command{
	Use:               "stats",
	Aliases:           []string{"stat", "tokens", "usage"},
	Short:             "Display token consumption statistics and activity charts across profiles",
	ValidArgsFunction: CompleteProfileNames,
	Long: `Analyze and visualize daily token consumption (Input, Output, Cached) across all Antigravity profiles
and models with ASCII activity charts.

Examples:
  agyp stats                    # Show token usage for the last 7 days
  agyp stats --days 14          # Show token usage for the last 14 days
  agyp stats --days 30          # Show token usage for the last month
  agyp stats -p agy1            # Filter stats by specific profile
  agyp stats --json             # Output raw structured JSON metrics
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		summary, err := profile.GetTokenStatsSummary(statsDays, statsProfile)
		if err != nil {
			return fmt.Errorf("failed to retrieve token statistics: %w", err)
		}

		if statsJSON {
			data, err := json.MarshalIndent(summary, "", "  ")
			if err != nil {
				return err
			}
			cmd.Println(string(data))
			return nil
		}

		useColor := os.Getenv("NO_COLOR") == ""
		rendered := profile.FormatTokenStats(summary, useColor)
		cmd.Print(rendered)
		return nil
	},
}

var statsResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset all accumulated token usage statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := profile.ResetTokenStats(); err != nil {
			return fmt.Errorf("failed to reset token statistics: %w", err)
		}
		cmd.Println("✓ Накопленная статистика токенов успешно сброшена.")
		return nil
	},
}

func init() {
	statsCmd.Flags().IntVarP(&statsDays, "days", "d", 7, "Number of days to analyze (default 7)")
	statsCmd.Flags().StringVarP(&statsProfile, "profile", "p", "", "Filter statistics to a specific profile")
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Output statistics in JSON format")

	statsCmd.AddCommand(statsResetCmd)
	rootCmd.AddCommand(statsCmd)
}
