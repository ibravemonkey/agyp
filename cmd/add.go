package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/quaywin/agys/internal/shell"
	"github.com/quaywin/agys/pkg/profile"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <profile_name>",
	Short: "Create a new profile and perform agy login",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		if err := profile.ValidateName(profileName); err != nil {
			return err
		}

		exists, profileDir, err := profile.Exists(profileName)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("профиль %q уже существует в %s", profileName, profileDir)
		}

		cmd.Printf("\n\033[1;34m●\033[0m Создание изолированного профиля \033[1;37m%s\033[0m...\n", profileName)
		createdDir, err := profile.Create(profileName)
		if err != nil {
			return err
		}

		cmd.Printf("\033[1;34m●\033[0m Открываем браузер для авторизации Google OAuth (`agy`)...\n\n")

		if err := profile.RunCmdWithSignals(cmd.Context(), createdDir); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "\n\033[1;33m!\033[0m Предупреждение: процесс `agy` завершился с ошибкой: %v\n", err)
			_ = profile.Delete(profileName)
			return err
		}

		// Persist newly created Keychain token to profile disk storage
		profile.SyncKeychainTokenToDisk(createdDir, "")
		_ = profile.SyncAllTokenLocations(createdDir)
		_ = profile.EnsureOnboardingCompleted(createdDir)

		// Check authenticated email
		email, _ := profile.FetchProfileEmail(cmd.Context(), profileName)
		if email != "" {
			cmd.Printf("\n\033[1;32m✓\033[0m Авторизация успешна: \033[1;37m%s\033[0m\n", email)
		} else {
			cmd.Printf("\n\033[1;32m✓\033[0m Профиль %q успешно инициализирован\n", profileName)
		}

		// Generate instant executable shims in ~/.local/bin and update shell rc
		mgr := shell.NewSetupManager()
		home, _ := profile.GetRealUserHome()
		binDir := filepath.Join(home, ".local", "bin")

		allProfiles, _ := profile.List()
		createdShims, _ := mgr.SyncProfileShims(binDir, allProfiles)

		rcs := mgr.DetectShellRCs(home)
		for _, rc := range rcs {
			_, _ = mgr.ConfigureShellRC(rc, allProfiles)
		}

		// Display created shims for convenience
		prioIndex := 1
		for i, p := range allProfiles {
			if p == profileName {
				prioIndex = i + 1
				break
			}
		}

		cmd.Println("\n\033[1;36m✨ Мгновенные команды созданы в ~/.local/bin (доступны без перезапуска):\033[0m")
		cmd.Printf("  ● \033[1;32magy%d\033[0m   — запуск Antigravity CLI под этим профилем\n", prioIndex)
		cmd.Printf("  ● \033[1;32muse%d\033[0m   — переключить профиль по умолчанию на %s\n", prioIndex, profileName)
		if profileName != fmt.Sprintf("agy%d", prioIndex) {
			cmd.Printf("  ● \033[1;32m%s\033[0m   — прямой запуск\n", profileName)
		}

		_ = createdShims

		cmd.Println("\nГотово! Вы можете сразу набрать \033[1;32magyq\033[0m для проверки квот или начать работу.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
