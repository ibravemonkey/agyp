package cmd

import (
	"strings"

	"github.com/quaywin/agys/pkg/profile"
	"github.com/spf13/cobra"
)

var (
	aliasPrefix string
)

var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Generate shell aliases for configured profiles",
	Long: `Generate shell alias commands for all configured agys profiles.

Add the following to your ~/.zshrc or ~/.bashrc to auto-generate profile aliases:
  eval "$(agys alias)"
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := profile.List()
		if err != nil {
			return err
		}

		if len(profiles) == 0 {
			cmd.Println("# No active profiles found.")
			cmd.Println("# Use `agys add <profile_name>` to create a profile first.")
			return nil
		}

		cmd.Println("# agys shell aliases")
		for _, p := range profiles {
			// Normalize profile name for alias (replace hyphens/special chars if needed)
			aliasName := aliasPrefix + p
			aliasName = strings.ReplaceAll(aliasName, "-", "_")
			cmd.Printf("alias %s=\"agys run %s --\"\n", aliasName, p)
		}
		return nil
	},
}

func init() {
	aliasCmd.Flags().StringVarP(&aliasPrefix, "prefix", "p", "agy-", "Prefix for generated profile aliases")
	rootCmd.AddCommand(aliasCmd)
}
