package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ibravemonkey/agyp/internal/shell"
	"github.com/ibravemonkey/agyp/pkg/profile"
	"github.com/spf13/cobra"
)

var (
	setupUninstall bool
	setupTargetRC  string
	setupBinDir    string
)

var setupShellCmd = &cobra.Command{
	Use:     "setup-shell",
	Aliases: []string{"setup", "init-shell"},
	Short:   "Configure shell integration and install standalone commands (agy, agyq) automatically",
	Long: `Configures your shell (~/.zshrc or ~/.bashrc) and installs executable shims in ~/.local/bin
so that 'agy', 'agyq', 'agyp', and profile aliases work automatically out of the box without manual configuration.

Examples:
  agyp setup-shell            # Auto-detect shell and install integration
  agyp setup-shell -u         # Remove agyp shell integration
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, err := profile.GetRealUserHome()
		if err != nil {
			homeDir, _ = os.UserHomeDir()
		}

		binDir := setupBinDir
		if binDir == "" {
			binDir = filepath.Join(homeDir, ".local", "bin")
		}

		mgr := shell.NewSetupManager()

		var rcFiles []string
		if setupTargetRC != "" {
			rcFiles = []string{setupTargetRC}
		} else {
			rcFiles = mgr.DetectShellRCs(homeDir)
		}

		if setupUninstall {
			for _, rc := range rcFiles {
				removed, unErr := mgr.UninstallShellRC(rc)
				if unErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to clean %s: %v\n", rc, unErr)
				} else if removed {
					cmd.Printf("✓ Удалена интеграция agyp из %s\n", rc)
				}
			}
			cmd.Println("Интеграция с оболочкой успешно удалена.")
			return nil
		}

		// 1. Install executable shims (agy, agyq) in ~/.local/bin
		shims, shimErr := mgr.InstallShims(binDir)
		if shimErr != nil {
			return fmt.Errorf("ошибка установки исполняемых команд в %s: %w", binDir, shimErr)
		}
		for _, s := range shims {
			cmd.Printf("✓ Установлена команда: %s\n", s)
		}

		// 2. Synchronize profile launcher & switcher shims (agy1, use1, ...)
		profiles, _ := profile.List()
		profShims, _ := mgr.SyncProfileShims(binDir, profiles)
		if len(profShims) > 0 {
			cmd.Printf("✓ Созданы команды профилей: %s\n", strings.Join(profShims, ", "))
		}

		// 3. Configure shell rc files
		configuredAny := false
		for _, rc := range rcFiles {
			ok, cfgErr := mgr.ConfigureShellRC(rc, profiles)
			if cfgErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Предупреждение: не удалось настроить %s: %v\n", rc, cfgErr)
			} else if ok {
				cmd.Printf("✓ Настроена оболочка: %s\n", rc)
				configuredAny = true
			}
		}

		if !configuredAny && len(rcFiles) > 0 {
			cmd.Printf("✓ Оболочка уже была настроена (%s)\n", rcFiles[0])
		}

		// 3. User feedback
		cmd.Println("\n✨ Все готово! Команды работают «из-под капота»:")
		cmd.Println("  ● agyq               — просмотр лимитов и квот всех аккаунтов")
		cmd.Println("  ● agyp add <name>    — создание профиля и запуск авторизации")
		cmd.Println("  ● agy                — запуск Antigravity через активный профиль")
		cmd.Println("  ● agyp auto          — умный запуск по наибольшей квоте")
		cmd.Println("  ● agyp list          — список всех профилей")

		primaryRC := "~/.zshrc"
		if len(rcFiles) > 0 {
			primaryRC = rcFiles[0]
		}
		cmd.Printf("\nЧтобы обновить текущую сессию терминала, выполните:\n  source %s\n", primaryRC)

		return nil
	},
}

func init() {
	setupShellCmd.Flags().BoolVarP(&setupUninstall, "uninstall", "u", false, "Remove agyp shell integration from rc file")
	setupShellCmd.Flags().StringVar(&setupTargetRC, "rc", "", "Target shell configuration file (defaults to auto-detected ~/.zshrc or ~/.bashrc)")
	setupShellCmd.Flags().StringVar(&setupBinDir, "bin-dir", "", "Target bin directory for shims (defaults to ~/.local/bin)")

	rootCmd.AddCommand(setupShellCmd)
}
