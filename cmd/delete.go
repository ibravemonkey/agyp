package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/quaywin/agys/internal/shell"
	"github.com/quaywin/agys/pkg/profile"
	"github.com/spf13/cobra"
)

var forceDelete bool

var deleteCmd = &cobra.Command{
	Use:               "delete <profile_name>",
	Aliases:           []string{"rm"},
	Short:             "Delete a profile directory",
	ValidArgsFunction: CompleteProfileNames,
	Args:              cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		exists, profileDir, err := profile.Exists(profileName)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("профиль %q не существует", profileName)
		}

		if !forceDelete {
			cmd.Printf("Вы действительно хотите удалить профиль %q (%s)? [y/N]: ", profileName, profileDir)
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("не удалось прочитать подтверждение: %w", err)
			}
			input = strings.ToLower(strings.TrimSpace(input))
			if input != "y" && input != "yes" {
				cmd.Println("Удаление отменено.")
				return nil
			}
		}

		if err := profile.Delete(profileName); err != nil {
			return err
		}

		// Clean up shims and refresh shell rc
		mgr := shell.NewSetupManager()
		home, _ := profile.GetRealUserHome()
		binDir := filepath.Join(home, ".local", "bin")
		remainingProfiles, _ := profile.List()

		_, _ = mgr.SyncProfileShims(binDir, remainingProfiles)

		rcs := mgr.DetectShellRCs(home)
		for _, rc := range rcs {
			_, _ = mgr.ConfigureShellRC(rc, remainingProfiles)
		}

		cmd.Printf("\033[1;32m✓\033[0m Профиль %q успешно удален.\n", profileName)
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVarP(&forceDelete, "force", "f", false, "Force deletion without prompt")
	rootCmd.AddCommand(deleteCmd)
}
