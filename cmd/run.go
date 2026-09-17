package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/ibravemonkey/agyp/internal/runner"
	"github.com/ibravemonkey/agyp/pkg/profile"
	"github.com/spf13/cobra"
)

var runAll bool

var runCmd = &cobra.Command{
	Use:               "run [profile_name] -- [agy_commands]",
	Short:             "Execute agy command with specified profile, auto quota selection, or default profile",
	ValidArgsFunction: CompleteRunArgs,
	Args:              cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		var profileName string
		var agyArgs []string

		var firstArg string
		if len(args) > 0 {
			firstArg = args[0]
		}

		if firstArg != "" && profile.IsAuto(firstArg) {
			profileName = profile.AutoProfileKeyword
			agyArgs = args[1:]
		} else if firstArg != "" {
			exists, _, _ := profile.Exists(firstArg)
			if exists {
				profileName = firstArg
				agyArgs = args[1:]
			} else {
				defaultProf, err := resolveDefaultProfile()
				if err != nil {
					return err
				}
				if defaultProf != "" {
					profileName = defaultProf
					agyArgs = args
				} else {
					if profile.ValidateName(firstArg) != nil && strings.HasPrefix(firstArg, "-") {
						return fmt.Errorf("no profile specified and no default profile set. Specify a profile or set one with `agys use <profile_name>`")
					}
					return fmt.Errorf("profile %q does not exist. Use `agys add %s` to create it, or set a default profile with `agys use <profile_name>`", firstArg, firstArg)
				}
			}
		} else {
			defaultProf, err := resolveDefaultProfile()
			if err != nil {
				return err
			}
			if defaultProf != "" {
				profileName = defaultProf
				agyArgs = args
			} else {
				return fmt.Errorf("no profile specified and no default profile set. Specify a profile or set one with `agys use <profile_name>`")
			}
		}

		opts := runner.RunOptions{
			ProfileName: profileName,
			AgyArgs:     agyArgs,
			RunAll:      runAll,
			Stdout:      cmd.OutOrStdout(),
			Stderr:      cmd.ErrOrStderr(),
		}

		execRunner := runner.NewRunner()
		return execRunner.Run(cmd.Context(), opts)
	},
}

func resolveDefaultProfile() (string, error) {
	return runner.ResolveDefaultProfile()
}

func runWithProfile(cmd *cobra.Command, profileName string, agyArgs []string) error {
	return runWithProfileAndDir(cmd, profileName, agyArgs, "")
}

func runWithProfileAndDir(cmd *cobra.Command, profileName string, agyArgs []string, workingDir string) error {
	opts := runner.RunOptions{
		ProfileName: profileName,
		AgyArgs:     agyArgs,
		WorkingDir:  workingDir,
		Stdout:      cmd.OutOrStdout(),
		Stderr:      cmd.ErrOrStderr(),
	}
	return runner.NewRunner().Run(cmd.Context(), opts)
}

func resolveResumeProfile(profileName string, agyArgs []string, workingDir ...string) (string, []string, error) {
	return runner.ResolveResumeProfile(profileName, agyArgs, os.Stderr, workingDir...)
}

func isInteractiveSession(agyArgs []string) bool {
	return runner.IsInteractiveSession(agyArgs)
}

// EnsureDefaultModelAndEffort delegates to runner.EnsureDefaultModelAndEffort.
func EnsureDefaultModelAndEffort(args []string) []string {
	return runner.EnsureDefaultModelAndEffort(args)
}

// EnsureDefaultModelAndEffortWithModel delegates to runner.EnsureDefaultModelAndEffortWithModel.
func EnsureDefaultModelAndEffortWithModel(args []string, defaultModel string) []string {
	return runner.EnsureDefaultModelAndEffortWithModel(args, defaultModel)
}

func init() {
	runCmd.Flags().BoolVarP(&runAll, "all", "a", false, "Execute agy command across all profiles sequentially")
	runCmd.DisableFlagParsing = false
	rootCmd.AddCommand(runCmd)
}
