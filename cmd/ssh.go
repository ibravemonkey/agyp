package cmd

import (
	"fmt"
	"strings"

	"github.com/ibravemonkey/agyp/internal/sshproxy"
	"github.com/ibravemonkey/agyp/pkg/profile"
	"github.com/spf13/cobra"
)

func shellQuote(s string) string {
	return sshproxy.ShellQuote(s)
}

func startLocalHTTPProxy() (int, func(), error) {
	return sshproxy.StartLocalHTTPProxy()
}

func syncProfileToRemote(server, profileName string) error {
	syncer := sshproxy.NewSSHProfileSyncer()
	return syncer.SyncProfile(rootCmd.Context(), server, profileName)
}

var sshCmd = &cobra.Command{
	Use:               "ssh <server> [remote_path] [profile_name] -- [agy_commands]",
	Short:             "Execute agys/agy natively on a remote server over SSH at a specific path",
	SilenceUsage:      true,
	ValidArgsFunction: CompleteSSHArgs,
	Long: `Connects to a remote host over SSH with pseudo-terminal (PTY) allocation (-t),
automatically syncing local profile credentials, tunneling API requests through local proxy, and executing agys/agy natively on the remote Linux host.

Examples:
  agys ssh user@remote-server
  agys ssh user@remote-server work
  agys ssh user@remote-server /var/www/myproject work
  agys ssh user@remote-server /var/www/myproject work -- --dangerously-skip-permissions
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		server := args[0]
		var remotePath string
		var profileName string
		var agyArgs []string

		remaining := args[1:]
		if len(remaining) > 0 {
			first := remaining[0]
			if strings.HasPrefix(first, "/") || strings.HasPrefix(first, "~") || strings.HasPrefix(first, "./") || strings.Contains(first, "/") {
				remotePath = first
				remaining = remaining[1:]
			}
		}

		if len(remaining) > 0 {
			first := remaining[0]
			if !strings.HasPrefix(first, "-") {
				exists, _, _ := profile.Exists(first)
				if profile.IsAuto(first) || exists {
					profileName = first
					agyArgs = remaining[1:]
				} else if remotePath == "" {
					remotePath = first
					remaining = remaining[1:]
					if len(remaining) > 0 && !strings.HasPrefix(remaining[0], "-") {
						profileName = remaining[0]
						agyArgs = remaining[1:]
					} else {
						agyArgs = remaining
					}
				} else {
					profileName = first
					agyArgs = remaining[1:]
				}
			} else {
				agyArgs = remaining
			}
		}

		if profileName == "" {
			current, _ := profile.GetCurrent()
			if current != "" {
				profileName = current
			} else {
				profileName = profile.AutoProfileKeyword
			}
		}

		// Ensure default model (gemini-3.8-flash) and reasoning effort (high) if not specified
		agyArgs = EnsureDefaultModelAndEffort(agyArgs)

		// Resolve auto-profile if needed
		var targetProfile string
		if profile.IsAuto(profileName) {
			selected, score, err := profile.SelectBestProfile(cmd.Context())
			if err != nil {
				return fmt.Errorf("auto profile selection failed on local machine: %w", err)
			}
			targetProfile = selected
			scoreStr := fmt.Sprintf("%.1f%%", score*100)
			if score < 0 {
				scoreStr = "N/A"
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "[agys] Auto-selected local profile %q (5h Gemini quota: %s)\n", targetProfile, scoreStr)
		} else {
			targetProfile = profileName
		}

		exists, _, err := profile.Exists(targetProfile)
		if err != nil || !exists {
			return fmt.Errorf("local profile %q does not exist. Use `agys add %s` to create it first", targetProfile, targetProfile)
		}

		svc := sshproxy.NewService(nil, nil, nil, cmd.ErrOrStderr())
		return svc.Execute(cmd.Context(), server, remotePath, targetProfile, agyArgs)
	},
}

func init() {
	sshCmd.DisableFlagParsing = false
	rootCmd.AddCommand(sshCmd)
}
