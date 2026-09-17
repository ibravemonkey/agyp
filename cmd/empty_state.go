package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

// PrintEmptyProfilesBanner renders a clean, informative onboarding card when no profiles exist.
func PrintEmptyProfilesBanner(out io.Writer, title string) {
	baseDir, _ := profile.GetBaseDir()
	useColor := os.Getenv("NO_COLOR") == ""

	if !useColor {
		fmt.Fprintf(out, "\n⚡ %s\n\n", title)
		fmt.Fprintf(out, "No profiles found in %s\n\n", baseDir)
		fmt.Fprintln(out, "Подключите аккаунты для начала работы:")
		fmt.Fprintln(out, "  ● agyp add agy1    — подключить первый аккаунт (Google OAuth)")
		fmt.Fprintln(out, "  ● agyp add agy2    — подключить второй аккаунт")
		fmt.Fprintln(out, "  ● agyp add <name>  — подключить с произвольным именем (например, work)")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "После добавления станут доступны команды:")
		fmt.Fprintln(out, "  ● agyq             — просмотр лимитов 5H и недельных")
		fmt.Fprintln(out, "  ● agy1, agy2       — запуск Antigravity CLI под нужным аккаунтом")
		fmt.Fprintln(out, "  ● agyp auto        — авто-выбор аккаунта с максимальной квотой")
		fmt.Fprintln(out, "  ● agyp list        — список всех профилей")
		fmt.Fprintln(out, "")
		return
	}

	fmt.Fprintf(out, "\n\033[1;36m⚡ %s\033[0m\n\n", title)
	fmt.Fprintln(out, "  \033[90m╭──────────────────────────────────────────────────────────────╮\033[0m")
	fmt.Fprintf(out, "  \033[90m│\033[0m  \033[1;33mNo profiles found\033[0m in \033[36m%s\033[0m\033[90m%s│\033[0m\n", baseDir, padSpaces(baseDir, 39))
	fmt.Fprintln(out, "  \033[90m│                                                              │\033[0m")
	fmt.Fprintln(out, "  \033[90m│\033[0m  \033[1mПодключите аккаунты для начала работы:\033[0m                      \033[90m│\033[0m")
	fmt.Fprintln(out, "  \033[90m│\033[0m    \033[1;32m● agyp add agy1\033[0m    — первый аккаунт (Google OAuth)        \033[90m│\033[0m")
	fmt.Fprintln(out, "  \033[90m│\033[0m    \033[1;32m● agyp add agy2\033[0m    — второй аккаунт                      \033[90m│\033[0m")
	fmt.Fprintln(out, "  \033[90m│\033[0m    \033[1;32m● agyp add <name>\033[0m  — профиль с любым именем (напр. work) \033[90m│\033[0m")
	fmt.Fprintln(out, "  \033[90m│                                                              │\033[0m")
	fmt.Fprintln(out, "  \033[90m│\033[0m  \033[1mПосле добавления станут доступны команды:\033[0m                  \033[90m│\033[0m")
	fmt.Fprintln(out, "  \033[90m│\033[0m    \033[36m● agyq\033[0m             — просмотр лимитов 5H и недельных      \033[90m│\033[0m")
	fmt.Fprintln(out, "  \033[90m│\033[0m    \033[36m● agy1, agy2\033[0m       — запуск Antigravity через профиль     \033[90m│\033[0m")
	fmt.Fprintln(out, "  \033[90m│\033[0m    \033[36m● agyp auto\033[0m        — авто-выбор по максимальной квоте     \033[90m│\033[0m")
	fmt.Fprintln(out, "  \033[90m│\033[0m    \033[36m● agyp list\033[0m        — список всех профилей                 \033[90m│\033[0m")
	fmt.Fprintln(out, "  \033[90m╰──────────────────────────────────────────────────────────────╯\033[0m")
}

func padSpaces(text string, maxLen int) string {
	diff := maxLen - len(text)
	if diff <= 0 {
		return " "
	}
	spaces := make([]byte, diff)
	for i := range spaces {
		spaces[i] = ' '
	}
	return string(spaces)
}
