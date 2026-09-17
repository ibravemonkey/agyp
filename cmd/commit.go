package cmd

import (
	"os"

	"github.com/ibravemonkey/agyp/internal/gitops"
	"github.com/spf13/cobra"
)

var (
	commitMsg      string
	commitAll      bool
	commitStageAll bool
	commitYes      bool
	commitPush     bool
	commitNoCheck  bool
	commitDryRun   bool
	commitModel    string
	commitEffort   string
	commitPrompt   string
)

var commitCmd = &cobra.Command{
	Use:               "commit [profile_name] [flags]",
	Short:             "Check staged git files with AI and commit using auto-selected or specified profile",
	Long:              `Inspects staged git changes using an AI profile (auto-selected based on 5h Gemini quota or specified), performs a code review check, generates or validates a commit message, and commits the changes.`,
	ValidArgsFunction: CompleteProfileNames,
	Args:              cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var profileName string
		if len(args) > 0 {
			profileName = args[0]
		}

		opts := gitops.CommitOptions{
			ProfileName: profileName,
			Message:     commitMsg,
			All:         commitAll,
			StageAll:    commitStageAll,
			Yes:         commitYes,
			Push:        commitPush,
			NoCheck:     commitNoCheck,
			DryRun:      commitDryRun,
			Model:       commitModel,
			Effort:      commitEffort,
			Prompt:      commitPrompt,
		}

		svc := gitops.NewService(nil, nil, os.Stdin, cmd.OutOrStdout(), cmd.ErrOrStderr())
		return svc.Execute(cmd.Context(), opts)
	},
}

func init() {
	commitCmd.Flags().StringVarP(&commitMsg, "message", "m", "", "Specify commit message directly")
	commitCmd.Flags().BoolVarP(&commitAll, "all", "a", false, "Automatically stage modified/deleted tracked files before commit (git add -u)")
	commitCmd.Flags().BoolVarP(&commitStageAll, "stage-all", "A", false, "Automatically stage all changes including untracked files before commit (git add -A)")
	commitCmd.Flags().BoolVarP(&commitYes, "yes", "y", false, "Automatically accept commit message and commit without interactive prompt")
	commitCmd.Flags().BoolVarP(&commitPush, "push", "p", false, "Automatically push to current git branch after committing")
	commitCmd.Flags().BoolVar(&commitNoCheck, "no-check", false, "Skip AI code review check")
	commitCmd.Flags().BoolVar(&commitDryRun, "dry-run", false, "Perform AI review and message generation without executing git commit")
	commitCmd.Flags().StringVar(&commitModel, "model", "", "Override model for agy commit check (defaults to latest Gemini Flash)")
	commitCmd.Flags().StringVar(&commitEffort, "effort", "", "Override reasoning effort for agy commit check (defaults to low)")
	commitCmd.Flags().StringVar(&commitPrompt, "prompt", "", "Additional custom prompt instructions for commit check")

	rootCmd.AddCommand(commitCmd)
}
