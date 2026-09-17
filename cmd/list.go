package cmd

import (
	"context"
	"fmt"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/ibravemonkey/agyp/pkg/profile"
	"github.com/spf13/cobra"
)

var (
	listQuota bool
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all active profile directories",
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := profile.List()
		if err != nil {
			return err
		}

		if len(profiles) == 0 {
			PrintEmptyProfilesBanner(cmd.OutOrStdout(), "Antigravity Profiles Manager (agyp)")
			return nil
		}

		currentProfile, _ := profile.GetCurrent()
		priorities, _ := profile.GetAllPriorities()

		duplicates, _ := profile.DetectDuplicateTokens()

		if !listQuota {
			cmd.Println("Active Profiles:")
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "PROFILE\tPRIO\tEMAIL\tCONFIG\tPATH")
			for _, p := range profiles {
				dir, _ := profile.GetProfileDir(p)
				pName := p
				if p == currentProfile {
					pName += " (default)"
				}
				prio := priorities[p]
				email, _ := profile.GetCachedEmail(p)
				if email == "" {
					email = "-"
				}
				if dupList, isDup := duplicates[p]; isDup {
					email += fmt.Sprintf(" [!] DUPLICATE TOKEN (shared with %v)", dupList)
				}
				cfg := profile.GetConfigSummary(p)
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n", pName, prio, email, cfg, dir)
			}
			tw.Flush()
			return nil
		}

		// Query quotas in parallel if listQuota is true
		ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		results := make([]profile.ProfileQuotaInfo, len(profiles))

		for i, pName := range profiles {
			wg.Add(1)
			go func(index int, name string) {
				defer wg.Done()
				email, _ := profile.FetchProfileEmail(ctx, name)
				summary, err := profile.FetchQuota(ctx, name)
				cliCfg := profile.IsCLIConfigured(name)
				ideCfg := profile.IsIDEConfigured(name)
				cfgSum := profile.GetConfigSummary(name)
				if err != nil {
					results[index] = profile.ProfileQuotaInfo{
						ProfileName:   name,
						Email:         email,
						Active:        false,
						Error:         err.Error(),
						CLIConfigured: cliCfg,
						IDEConfigured: ideCfg,
						Configured:    cfgSum,
					}
				} else {
					results[index] = profile.ProfileQuotaInfo{
						ProfileName:   name,
						Email:         email,
						Active:        true,
						Quota:         summary,
						CLIConfigured: cliCfg,
						IDEConfigured: ideCfg,
						Configured:    cfgSum,
					}
				}
			}(i, pName)
		}

		wg.Wait()

		cmd.Println("Active Profiles & Quota Status:")
		profile.RenderQuotaTable(cmd.OutOrStdout(), results, currentProfile, priorities)
		return nil
	},
}

func init() {
	listCmd.Flags().BoolVarP(&listQuota, "quota", "q", false, "Show quota summary for each profile")
	rootCmd.AddCommand(listCmd)
}
