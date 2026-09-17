package cmd

import (
	"github.com/ibravemonkey/agyp/pkg/profile"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:               "rename <old_name> <new_name>",
	Aliases:           []string{"mv"},
	Short:             "Rename an existing profile directory",
	ValidArgsFunction: CompleteCloneOrRenameArgs,
	Args:              cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName := args[0]
		newName := args[1]

		if err := profile.Rename(oldName, newName); err != nil {
			return err
		}

		cmd.Printf("Profile %q successfully renamed to %q.\n", oldName, newName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(renameCmd)
}
