package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ibravemonkey/agyp/pkg/profile"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:               "clean [profile_name]",
	Aliases:           []string{"prune"},
	Short:             "Clean old conversation sessions, caches, and logs to reclaim disk space",
	Long: `Clean old conversation sessions based on TTL, transient caches (ide-data/logs, Crashpad, Caches),
and logs across a specific profile or all profiles while preserving recent sessions and credentials.`,
	ValidArgsFunction: CompleteProfileNames,
	Args:              cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ttlStr, _ := cmd.Flags().GetString("ttl")
		keepLast, _ := cmd.Flags().GetInt("keep-last")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		all, _ := cmd.Flags().GetBool("all")
		cacheOnly, _ := cmd.Flags().GetBool("cache-only")
		sessionsOnly, _ := cmd.Flags().GetBool("sessions-only")
		force, _ := cmd.Flags().GetBool("force")

		ttlDuration, err := profile.ParseTTL(ttlStr)
		if err != nil {
			return err
		}

		opts := profile.CleanOptions{
			TTL:          ttlDuration,
			KeepLast:     keepLast,
			DryRun:       dryRun,
			CacheOnly:    cacheOnly,
			SessionsOnly: sessionsOnly,
			RemoveLocks:  true,
		}

		var targetProfiles []string

		if all {
			if len(args) > 0 {
				return fmt.Errorf("cannot specify profile name when using --all flag")
			}
			profiles, err := profile.List()
			if err != nil {
				return fmt.Errorf("failed to list profiles: %w", err)
			}
			if len(profiles) == 0 {
				cmd.Println("No profiles configured.")
				return nil
			}
			targetProfiles = profiles
		} else {
			var pName string
			if len(args) > 0 {
				pName = args[0]
			} else {
				current, err := profile.GetCurrent()
				if err != nil {
					return err
				}
				if current == "" {
					return fmt.Errorf("no profile specified. Provide profile name or use --all (-a) to clean all profiles")
				}
				pName = current
			}

			exists, _, err := profile.Exists(pName)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("profile %q does not exist", pName)
			}
			targetProfiles = []string{pName}
		}

		if !dryRun && !force {
			promptMsg := fmt.Sprintf("Clean old sessions (older than %s, keeping last %d) and caches for %s? [y/N]: ",
				ttlStr, keepLast, strings.Join(targetProfiles, ", "))
			cmd.Print(promptMsg)

			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}
			input = strings.ToLower(strings.TrimSpace(input))
			if input != "y" && input != "yes" {
				cmd.Println("Cleanup cancelled.")
				return nil
			}
		}

		prefix := "\033[1;32m✓\033[0m"
		if dryRun {
			prefix = "\033[1;33m[DRY-RUN]\033[0m"
			cmd.Printf("%s Simulating profile cleanup (TTL: %s, keep-last: %d)...\n\n", prefix, ttlStr, keepLast)
		}

		var totalFreed int64
		var totalSessionsCleaned int

		for _, p := range targetProfiles {
			rep, err := profile.CleanProfile(p, opts)
			if err != nil {
				cmd.Printf("  • \033[1;31m✗\033[0m Profile %s: error: %v\n", p, err)
				continue
			}

			totalFreed += rep.BytesFreed
			totalSessionsCleaned += rep.SessionsCleaned

			cmd.Printf("  • Profile \033[1m%s\033[0m:\n", p)
			if !cacheOnly {
				cmd.Printf("    - Sessions: %d cleaned, %d preserved (scanned: %d)\n",
					rep.SessionsCleaned, rep.SessionsKept, rep.SessionsScanned)
			}
			if len(rep.CachesCleaned) > 0 {
				cmd.Printf("    - Caches cleared: %s\n", strings.Join(rep.CachesCleaned, ", "))
			}
			cmd.Printf("    - Space: \033[1;36m%s\033[0m\n", profile.FormatBytes(rep.BytesFreed))

			for _, errMsg := range rep.Errors {
				cmd.Printf("    - Warning: %s\n", errMsg)
			}
		}

		cmd.Println()
		if dryRun {
			cmd.Printf("%s Potential disk space to reclaim: \033[1;32m%s\033[0m (%d sessions across %d profile(s))\n",
				prefix, profile.FormatBytes(totalFreed), totalSessionsCleaned, len(targetProfiles))
		} else {
			cmd.Printf("%s Cleanup finished: reclaimed \033[1;32m%s\033[0m (%d sessions pruned across %d profile(s))\n",
				prefix, profile.FormatBytes(totalFreed), totalSessionsCleaned, len(targetProfiles))
		}

		return nil
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		// Reset flag values to defaults after execution for safe repeated invocations
		_ = cmd.Flags().Set("dry-run", "false")
		_ = cmd.Flags().Set("force", "false")
		_ = cmd.Flags().Set("all", "false")
		_ = cmd.Flags().Set("cache-only", "false")
		_ = cmd.Flags().Set("sessions-only", "false")
		_ = cmd.Flags().Set("ttl", "14d")
		_ = cmd.Flags().Set("older-than", "14d")
		_ = cmd.Flags().Set("keep-last", fmt.Sprintf("%d", profile.DefaultKeepLast))
	},
}

func init() {
	cleanCmd.Flags().String("ttl", "14d", "Delete sessions older than duration (e.g. 14d, 7days, 48h, 1w)")
	cleanCmd.Flags().String("older-than", "14d", "Alias for --ttl")
	cleanCmd.Flags().Int("keep-last", profile.DefaultKeepLast, "Minimum number of most recent sessions to keep")
	cleanCmd.Flags().BoolP("dry-run", "n", false, "Simulate cleanup without deleting any files")
	cleanCmd.Flags().BoolP("all", "a", false, "Clean all configured profiles")
	cleanCmd.Flags().Bool("cache-only", false, "Clean only caches and temporary logs, preserving all sessions")
	cleanCmd.Flags().Bool("sessions-only", false, "Clean only old sessions, preserving caches")
	cleanCmd.Flags().BoolP("force", "f", false, "Do not prompt for confirmation")

	rootCmd.AddCommand(cleanCmd)
}
