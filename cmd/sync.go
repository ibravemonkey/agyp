package cmd

import (
	"fmt"

	"github.com/ibravemonkey/agyp/pkg/profile"
	"github.com/spf13/cobra"
)

var syncQuiet bool

var syncCmd = &cobra.Command{
	Use:               "sync [profile_name]",
	Short:             "Synchronize skills, MCP servers, plugins, directives, and toolchains from base environment across profiles",
	Long: `Synchronizes developer environment components from your real home (~/) to agyp profiles:
- Skills (~/.gemini/config/skills and skills.json)
- MCP Servers (~/.gemini/config/mcp_config.json and settings.json mcpServers)
- Directives and rules (GEMINI.md, rules/, hooks.json)
- Tools (skill-compass, bin/ with rtk, sqz, notify-sound)
- Developer toolchains (.local, .cargo, .ssh, .gitconfig)

Examples:
  agyp sync           # Synchronize all profiles
  agyp sync agy1      # Synchronize specific profile
  agyp sync -q        # Quiet mode (suppresses output)
`,
	ValidArgsFunction: CompleteProfileNames,
	Args:              cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !syncQuiet {
			cmd.Println("\033[1;36m⚡ Синхронизация профилей agyp с базовым окружением...\033[0m")
		}

		if len(args) > 0 {
			target := args[0]
			exists, pDir, err := profile.Exists(target)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("профиль %q не существует", target)
			}
			if err := profile.SyncBaseEnvironmentToProfile(pDir); err != nil {
				return fmt.Errorf("ошибка синхронизации профиля %q: %w", target, err)
			}
			if !syncQuiet {
				cmd.Printf("  \033[1;32m✓\033[0m Профиль \033[1;37m%s\033[0m синхронизирован (skills, MCP, rtk, sqz, compass, plugins)\n", target)
				cmd.Println("✨ Синхронизация завершена успешно!")
			}
			return nil
		}

		synced, err := profile.SyncBaseEnvironmentToAllProfiles()
		if err != nil {
			return err
		}

		if !syncQuiet {
			if len(synced) == 0 {
				cmd.Println("  (нет активных профилей для синхронизации)")
			} else {
				for _, p := range synced {
					cmd.Printf("  \033[1;32m✓\033[0m Профиль \033[1;37m%s\033[0m синхронизирован (skills, MCP, rtk, sqz, compass, plugins)\n", p)
				}
				cmd.Printf("✨ Синхронизировано профилей: %d\n", len(synced))
			}
		}

		return nil
	},
}

func init() {
	syncCmd.Flags().BoolVarP(&syncQuiet, "quiet", "q", false, "Suppress output")
	rootCmd.AddCommand(syncCmd)
}
