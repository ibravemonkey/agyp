package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	BlockStartMarker       = "# >>> agyp >>>"
	BlockEndMarker         = "# <<< agyp <<<"
	LegacyBlockStartMarker = "# >>> agys_mod >>>"
	LegacyBlockEndMarker   = "# <<< agys_mod <<<"
)
type SetupManager interface {
	InstallShims(binDir string) ([]string, error)
	SyncProfileShims(binDir string, profiles []string) ([]string, error)
	ConfigureShellRC(rcPath string, profiles []string) (bool, error)
	UninstallShellRC(rcPath string) (bool, error)
	DetectShellRCs(homeDir string) []string
}

type defaultManager struct{}

// NewSetupManager returns a new SetupManager instance.
func NewSetupManager() SetupManager {
	return &defaultManager{}
}

// DetectShellRCs detects user's interactive shell configuration files.
func (m *defaultManager) DetectShellRCs(homeDir string) []string {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}

	var candidates []string
	zshrc := filepath.Join(homeDir, ".zshrc")
	bashrc := filepath.Join(homeDir, ".bashrc")
	bashProfile := filepath.Join(homeDir, ".bash_profile")

	shellEnv := os.Getenv("SHELL")
	if strings.Contains(shellEnv, "zsh") {
		candidates = append(candidates, zshrc)
		if fileExists(bashrc) {
			candidates = append(candidates, bashrc)
		}
	} else if strings.Contains(shellEnv, "bash") {
		candidates = append(candidates, bashrc)
		if fileExists(bashProfile) {
			candidates = append(candidates, bashProfile)
		}
		if fileExists(zshrc) {
			candidates = append(candidates, zshrc)
		}
	} else {
		// Default: zshrc on macOS / general, bashrc if exists
		candidates = append(candidates, zshrc)
		if fileExists(bashrc) {
			candidates = append(candidates, bashrc)
		}
	}

	return candidates
}

// InstallShims creates lightweight executable scripts in binDir for `agy` and `agyq`.
func (m *defaultManager) InstallShims(binDir string) ([]string, error) {
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", binDir, err)
	}

	var created []string

	// 1. Shim for agy -> agyp run "$@"
	// CAUTION: If agy is already the actual Antigravity CLI binary (executable, not our wrapper),
	// do NOT overwrite it! The shell function in .zshrc handles interactive wrapping,
	// and overwriting the real binary would cause infinite recursion in agyp run.
	agyShimPath := filepath.Join(binDir, "agy")
	agyContent := `#!/bin/sh
# agy wrapper by agyp
exec agyp run "$@"
`
	shouldWriteAgy := true
	if data, err := os.ReadFile(agyShimPath); err == nil {
		if !strings.Contains(string(data), "# agy wrapper by agyp") && !strings.Contains(string(data), "# agy wrapper by agys_mod") {
			shouldWriteAgy = false
		}
	}
	if shouldWriteAgy {
		if err := os.WriteFile(agyShimPath, []byte(agyContent), 0755); err != nil {
			return nil, fmt.Errorf("failed to create agy shim: %w", err)
		}
		created = append(created, agyShimPath)
	}

	// 2. Shim for agyq -> agyp quota "$@" (or python script if present)
	agyqShimPath := filepath.Join(binDir, "agyq")
	agyqContent := `#!/bin/sh
# agyq quota viewer by agyp
if [ -f "$HOME/.local/bin/agy-quota" ] && command -v python3 >/dev/null 2>&1; then
  exec python3 "$HOME/.local/bin/agy-quota" "$@"
fi
exec agyp quota "$@"
`
	if err := os.WriteFile(agyqShimPath, []byte(agyqContent), 0755); err != nil {
		return nil, fmt.Errorf("failed to create agyq shim: %w", err)
	}
	created = append(created, agyqShimPath)

	return created, nil
}

const profileShimHeader = "# agyp profile shim"

// SyncProfileShims creates instant executable commands in binDir for all active profiles.
// Examples: agy1, use1, work1, use-work1.
func (m *defaultManager) SyncProfileShims(binDir string, profiles []string) ([]string, error) {
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", binDir, err)
	}

	// 1. Remove old managed profile shims in binDir
	// CAUTION: Never delete core binaries (agyp, agyp, agy, agyp-sync, agyp-sync, agyq).
	// Only delete small shell scripts that start with #!/bin/ and contain profileShimHeader.
	entries, err := os.ReadDir(binDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if name == "agyp" || name == "agys" || name == "agy" || name == "agyp-sync" || name == "agys-sync" || name == "agyq" || name == "notify-sound.sh" {
				continue
			}
			filePath := filepath.Join(binDir, name)
			info, statErr := entry.Info()
			if statErr != nil || info.Size() > 8192 {
				continue
			}
			data, readErr := os.ReadFile(filePath)
			if readErr == nil && strings.HasPrefix(string(data), "#!/bin/") &&
				(strings.Contains(string(data), profileShimHeader) || strings.Contains(string(data), "# agys profile shim")) {
				_ = os.Remove(filePath)
			}
		}
	}

	// 2. Generate shims for current profiles
	var created []string
	for i, p := range profiles {
		idxStr := resolveProfileNumber(p, i)
		cleanName := strings.ReplaceAll(p, "-", "_")

		// Profile launcher content
		launcherContent := fmt.Sprintf(`#!/bin/sh
%s
agyp use %q >/dev/null 2>&1
exec agyp run %q "$@"
`, profileShimHeader, p, p)

		// Profile switcher content
		switcherContent := fmt.Sprintf(`#!/bin/sh
%s
exec agyp use %q "$@"
`, profileShimHeader, p)
		// Create numbered shims (agy<N>, use<N>)
		agyIdxShim := filepath.Join(binDir, "agy"+idxStr)
		if err := os.WriteFile(agyIdxShim, []byte(launcherContent), 0755); err == nil {
			created = append(created, "agy"+idxStr)
		}

		useIdxShim := filepath.Join(binDir, "use"+idxStr)
		if err := os.WriteFile(useIdxShim, []byte(switcherContent), 0755); err == nil {
			created = append(created, "use"+idxStr)
		}

		// Create named shim if profile name is different from agy<N>
		if p != "agy"+idxStr && p != "" {
			namedShim := filepath.Join(binDir, cleanName)
			if err := os.WriteFile(namedShim, []byte(launcherContent), 0755); err == nil {
				created = append(created, cleanName)
			}

			useNamedShim := filepath.Join(binDir, "use-"+cleanName)
			if err := os.WriteFile(useNamedShim, []byte(switcherContent), 0755); err == nil {
				created = append(created, "use-"+cleanName)
			}
		}
	}

	return created, nil
}

// ConfigureShellRC inserts or updates the managed block in a shell rc file.
func (m *defaultManager) ConfigureShellRC(rcPath string, profiles []string) (bool, error) {
	existingContent := ""
	if fileExists(rcPath) {
		bytes, err := os.ReadFile(rcPath)
		if err != nil {
			return false, fmt.Errorf("failed to read %s: %w", rcPath, err)
		}
		existingContent = string(bytes)
	}

	block := GenerateManagedBlock(profiles)

	var newContent string
	startIdx := strings.Index(existingContent, BlockStartMarker)
	endIdx := strings.Index(existingContent, BlockEndMarker)
	var markerLen int
	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		markerLen = len(BlockEndMarker)
	} else {
		startIdx = strings.Index(existingContent, LegacyBlockStartMarker)
		endIdx = strings.Index(existingContent, LegacyBlockEndMarker)
		markerLen = len(LegacyBlockEndMarker)
	}

	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		// Replace existing block
		before := existingContent[:startIdx]
		after := existingContent[endIdx+markerLen:]
		// Clean up leading newlines in after
		after = strings.TrimPrefix(after, "\n")
		newContent = strings.TrimRight(before, "\n") + "\n\n" + block + "\n"
		if after != "" {
			newContent += "\n" + strings.TrimLeft(after, "\n")
		}
	} else {
		// Append to file
		if strings.TrimSpace(existingContent) == "" {
			newContent = block + "\n"
		} else {
			newContent = strings.TrimRight(existingContent, "\n") + "\n\n" + block + "\n"
		}
	}

	if err := os.WriteFile(rcPath, []byte(newContent), 0644); err != nil {
		return false, fmt.Errorf("failed to write %s: %w", rcPath, err)
	}

	return true, nil
}

// UninstallShellRC removes the managed block from a shell rc file.
func (m *defaultManager) UninstallShellRC(rcPath string) (bool, error) {
	if !fileExists(rcPath) {
		return false, nil
	}

	bytes, err := os.ReadFile(rcPath)
	if err != nil {
		return false, err
	}
	content := string(bytes)

	removeBlock := func(s, startM, endM string) (string, bool) {
		sIdx := strings.Index(s, startM)
		eIdx := strings.Index(s, endM)
		if sIdx == -1 || eIdx == -1 || eIdx < sIdx {
			return s, false
		}
		before := s[:sIdx]
		after := s[eIdx+len(endM):]
		res := strings.TrimRight(before, "\n")
		if trimmedAfter := strings.TrimLeft(after, "\n"); trimmedAfter != "" {
			res += "\n\n" + trimmedAfter
		}
		return res + "\n", true
	}

	removedAny := false
	if updated, ok := removeBlock(content, BlockStartMarker, BlockEndMarker); ok {
		content = updated
		removedAny = true
	}
	if updated, ok := removeBlock(content, LegacyBlockStartMarker, LegacyBlockEndMarker); ok {
		content = updated
		removedAny = true
	}

	if !removedAny {
		return false, nil
	}

	if err := os.WriteFile(rcPath, []byte(content), 0644); err != nil {
		return false, err
	}

	return true, nil
}

// GenerateManagedBlock builds the shell script block to inject.
func GenerateManagedBlock(profiles []string) string {
	var sb strings.Builder

	sb.WriteString(BlockStartMarker + "\n")
	sb.WriteString("# Автоматическая настройка окружения agyp\n")
	sb.WriteString(`case ":$PATH:" in` + "\n")
	sb.WriteString(`  *":$HOME/.local/bin:"*) ;;` + "\n")
	sb.WriteString(`  *) export PATH="$HOME/.local/bin:$PATH" ;;` + "\n")
	sb.WriteString(`esac` + "\n\n")

	sb.WriteString(`agyp() {` + "\n")
	sb.WriteString(`  if [ -x "${HOME}/.local/bin/agyp-sync" ]; then` + "\n")
	sb.WriteString(`    "${HOME}/.local/bin/agyp-sync" --quiet 2>/dev/null` + "\n")
	sb.WriteString(`  elif [ -x "${HOME}/.local/bin/agys-sync" ]; then` + "\n")
	sb.WriteString(`    "${HOME}/.local/bin/agys-sync" --quiet 2>/dev/null` + "\n")
	sb.WriteString(`  fi` + "\n")
	sb.WriteString(`  command agyp "$@"` + "\n")
	sb.WriteString(`}` + "\n")
	sb.WriteString(`agys() { agyp "$@"; }` + "\n\n")
	sb.WriteString(`agy()  { agyp run "$@"; }` + "\n")
	sb.WriteString(`agyq() {` + "\n")
	sb.WriteString(`  if [ -x "${HOME}/.local/bin/agy-quota" ] && command -v python3 >/dev/null 2>&1; then` + "\n")
	sb.WriteString(`    "${HOME}/.local/bin/agy-quota" "$@"` + "\n")
	sb.WriteString(`  else` + "\n")
	sb.WriteString(`    agyp quota "$@"` + "\n")
	sb.WriteString(`  fi` + "\n")
	sb.WriteString(`}` + "\n")

	if len(profiles) > 0 {
		sb.WriteString("\n# Быстрые псевдонимы для профилей\n")
		for i, p := range profiles {
			aliasNum := resolveProfileNumber(p, i)
			cleanName := strings.ReplaceAll(p, "-", "_")
			sb.WriteString(fmt.Sprintf(`alias agy%s="agyp use %s && agyp run %s"`+"\n", aliasNum, p, p))
			sb.WriteString(fmt.Sprintf(`alias use%s="agyp use %s"`+"\n", aliasNum, p))
			if cleanName != aliasNum && cleanName != "agy"+aliasNum && cleanName != "" {
				sb.WriteString(fmt.Sprintf(`alias agy-%s="agyp run %s --"`+"\n", cleanName, p))
			}
		}
	}

	sb.WriteString(BlockEndMarker)
	return sb.String()
}

// resolveProfileNumber extracts explicit number if name matches "agy<N>", otherwise falls back to index+1.
func resolveProfileNumber(profileName string, index int) string {
	if strings.HasPrefix(profileName, "agy") && len(profileName) > 3 {
		num := profileName[3:]
		isDigits := true
		for _, ch := range num {
			if ch < '0' || ch > '9' {
				isDigits = false
				break
			}
		}
		if isDigits && len(num) > 0 {
			return num
		}
	}
	return fmt.Sprintf("%d", index+1)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
