package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SyncBaseEnvironmentToProfile synchronizes shared developer environment components
// (skills, plugins, MCP servers, rtk, sqz, compass, shell toolchains) from the user's
// base environment ($HOME) into an isolated profile.
func SyncBaseEnvironmentToProfile(profileDir string) error {
	if profileDir == "" {
		return nil
	}

	baseHome, err := GetRealUserHome()
	if err != nil || baseHome == "" {
		baseHome, _ = os.UserHomeDir()
	}
	if baseHome == "" {
		return nil
	}

	// 1. Create directory structure inside profile
	geminiConfigDir := filepath.Join(profileDir, ".gemini", "config")
	geminiCliDir := filepath.Join(profileDir, ".gemini", "antigravity-cli")
	_ = os.MkdirAll(geminiConfigDir, 0700)
	_ = os.MkdirAll(geminiCliDir, 0700)

	// 2. Shell & Dev Toolchains (shared tooling: .cargo, .ssh, .gitconfig, and .local/bin)
	// IMPORTANT: Never symlink the entire .local directory! On Linux and macOS, .local contains
	// .local/share (XDG_DATA_HOME) and .local/state (XDG_STATE_HOME) which store application data,
	// credentials, and keyrings. Symlinking all of .local breaks profile isolation!
	// Only symlink .local/bin so shared CLI tools (rtk, sqz, compass) are available in PATH.
	profileLocalDir := filepath.Join(profileDir, ".local")
	if info, err := os.Lstat(profileLocalDir); err == nil && (info.Mode()&os.ModeSymlink != 0) {
		_ = os.Remove(profileLocalDir)
	}
	_ = os.MkdirAll(filepath.Join(profileLocalDir, "share"), 0700)
	_ = os.MkdirAll(filepath.Join(profileLocalDir, "state"), 0700)
	_ = os.MkdirAll(filepath.Join(profileLocalDir, "bin"), 0700)

	baseLocalBin := filepath.Join(baseHome, ".local", "bin")
	if info, err := os.Stat(baseLocalBin); err == nil && info.IsDir() {
		_ = safeSymlink(baseLocalBin, filepath.Join(profileLocalDir, "bin"))
	}

	_ = safeSymlink(filepath.Join(baseHome, ".cargo"), filepath.Join(profileDir, ".cargo"))
	_ = safeSymlink(filepath.Join(baseHome, ".ssh"), filepath.Join(profileDir, ".ssh"))
	_ = safeSymlink(filepath.Join(baseHome, ".gitconfig"), filepath.Join(profileDir, ".gitconfig"))
	_ = safeSymlink(filepath.Join(baseHome, ".oh-my-zsh"), filepath.Join(profileDir, ".oh-my-zsh"))
	_ = safeSymlink(filepath.Join(baseHome, ".p10k.zsh"), filepath.Join(profileDir, ".p10k.zsh"))
	_ = safeSymlink(filepath.Join(baseHome, ".zsh"), filepath.Join(profileDir, ".zsh"))
	_ = safeSymlink(filepath.Join(baseHome, ".nvm"), filepath.Join(profileDir, ".nvm"))
	// 3. Directives & Rules (GEMINI.md, rules/)
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "GEMINI.md"), filepath.Join(profileDir, ".gemini", "GEMINI.md"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "rules"), filepath.Join(geminiConfigDir, "rules"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "rules"), filepath.Join(geminiCliDir, "rules"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "hooks.json"), filepath.Join(geminiConfigDir, "hooks.json"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "hooks.json"), filepath.Join(geminiCliDir, "hooks.json"))

	// 4. Skills (skills/ directory, skills.json)
	baseSkillsDir := filepath.Join(baseHome, ".gemini", "config", "skills")
	if _, err := os.Stat(baseSkillsDir); os.IsNotExist(err) {
		altSkills := filepath.Join(baseHome, ".gemini", "antigravity-cli", "skills")
		if _, altErr := os.Stat(altSkills); altErr == nil {
			baseSkillsDir = altSkills
		}
	}
	_ = safeSymlink(baseSkillsDir, filepath.Join(geminiConfigDir, "skills"))
	_ = safeSymlink(baseSkillsDir, filepath.Join(geminiCliDir, "skills"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "skills.json"), filepath.Join(geminiConfigDir, "skills.json"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "skills.json"), filepath.Join(geminiCliDir, "skills.json"))

	// 5. Tools (skill-compass, bin with rtk, sqz, notify-sound)
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "skill-compass"), filepath.Join(geminiConfigDir, "skill-compass"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "skill-compass"), filepath.Join(geminiCliDir, "skill-compass"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "bin"), filepath.Join(geminiConfigDir, "bin"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "bin"), filepath.Join(geminiCliDir, "bin"))

	// 6. Plugins & Extensions (plugins/ directory, plugins.json)
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "plugins"), filepath.Join(geminiConfigDir, "plugins"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "plugins"), filepath.Join(geminiCliDir, "plugins"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "plugins.json"), filepath.Join(geminiConfigDir, "plugins.json"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "plugins.json"), filepath.Join(geminiCliDir, "plugins.json"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "import_manifest.json"), filepath.Join(geminiConfigDir, "import_manifest.json"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "import_manifest.json"), filepath.Join(geminiCliDir, "import_manifest.json"))

	// 7. MCP Configurations (mcp_config.json)
	baseMcpConfig := filepath.Join(baseHome, ".gemini", "config", "mcp_config.json")
	if _, err := os.Stat(baseMcpConfig); os.IsNotExist(err) {
		altMcp := filepath.Join(baseHome, ".gemini", "antigravity-cli", "mcp_config.json")
		if _, altErr := os.Stat(altMcp); altErr == nil {
			baseMcpConfig = altMcp
		}
	}
	_ = safeSymlink(baseMcpConfig, filepath.Join(geminiConfigDir, "mcp_config.json"))
	_ = safeSymlink(baseMcpConfig, filepath.Join(geminiCliDir, "mcp_config.json"))
	geminiAppDir := filepath.Join(profileDir, ".gemini", "antigravity")
	_ = os.MkdirAll(geminiAppDir, 0700)
	_ = safeSymlink(baseMcpConfig, filepath.Join(geminiAppDir, "mcp_config.json"))

	// 8. OMP Environment (.omp/plugins, .omp/agent/config.yml, mcp.json, skill-compass)
	baseOmpDir := filepath.Join(baseHome, ".omp")
	if info, err := os.Stat(baseOmpDir); err == nil && info.IsDir() {
		profileOmpDir := filepath.Join(profileDir, ".omp")
		_ = os.MkdirAll(filepath.Join(profileOmpDir, "agent"), 0700)
		_ = safeSymlink(filepath.Join(baseOmpDir, "plugins"), filepath.Join(profileOmpDir, "plugins"))
		_ = safeSymlink(filepath.Join(baseOmpDir, "agent", "config.yml"), filepath.Join(profileOmpDir, "agent", "config.yml"))
		_ = safeSymlink(filepath.Join(baseOmpDir, "agent", "mcp.json"), filepath.Join(profileOmpDir, "agent", "mcp.json"))
		_ = safeSymlink(filepath.Join(baseOmpDir, "agent", "skill-compass"), filepath.Join(profileOmpDir, "agent", "skill-compass"))
	}
	// 8. Merge base configuration (theme, permissions, MCP, agentMode) into profile settings.json
	baseSettingsCandidates := []string{
		filepath.Join(baseHome, ".gemini", "antigravity-cli", "settings.json"),
		filepath.Join(baseHome, ".gemini", "settings.json"),
		filepath.Join(baseHome, ".gemini", "config", "settings.json"),
	}
	for _, baseSettings := range baseSettingsCandidates {
		if _, err := os.Stat(baseSettings); err == nil {
			_ = mergeBaseSettings(baseSettings, filepath.Join(geminiCliDir, "settings.json"))
			break
		}
	}

	// 9. Configure real-time statusLine hook for Antigravity footer bar
	_ = SyncStatusLineSettings(profileDir)

	// 10. Unified Conversation & History Store (shared across profiles)
	_ = SyncUnifiedStore(baseHome, profileDir)

	return nil
}

// SyncBaseEnvironmentToAllProfiles synchronizes all configured profiles with the base environment.
func SyncBaseEnvironmentToAllProfiles() ([]string, error) {
	profiles, err := List()
	if err != nil {
		return nil, err
	}

	var synced []string
	for _, p := range profiles {
		if IsAuto(p) {
			continue
		}
		pDir, pErr := GetProfileDir(p)
		if pErr != nil {
			continue
		}
		if err := SyncBaseEnvironmentToProfile(pDir); err == nil {
			synced = append(synced, p)
		}
	}
	return synced, nil
}

func safeSymlink(src, dest string) error {
	if src == "" || dest == "" {
		return nil
	}
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}

	// If dest exists: check if it's already a symlink pointing to src
	if target, err := os.Readlink(dest); err == nil {
		if target == src {
			return nil
		}
		_ = os.Remove(dest)
	} else if _, err := os.Lstat(dest); err == nil {
		_ = os.RemoveAll(dest)
	}

	return os.Symlink(src, dest)
}

func mergeBaseSettings(baseSettingsPath, profileSettingsPath string) error {
	bData, err := os.ReadFile(baseSettingsPath)
	if err != nil {
		return nil
	}
	var bRaw map[string]any
	if err := json.Unmarshal(bData, &bRaw); err != nil {
		return nil
	}
	if len(bRaw) == 0 {
		return nil
	}

	var pRaw map[string]any
	pData, err := os.ReadFile(profileSettingsPath)
	if err == nil {
		_ = json.Unmarshal(pData, &pRaw)
	}
	if pRaw == nil {
		pRaw = make(map[string]any)
	}

	// 1. Copy base user preferences (colorScheme, agentMode, notifications, model) if not set in profile
	keysToInherit := []string{
		"colorScheme",
		"agentMode",
		"artifactReviewPolicy",
		"enableTerminalSandbox",
		"toolPermission",
		"showFeedbackSurvey",
		"notifications",
		"model",
	}
	for _, key := range keysToInherit {
		if val, exists := bRaw[key]; exists {
			if _, pExists := pRaw[key]; !pExists {
				pRaw[key] = val
			}
		}
	}

	// 2. Merge permissions (allow list)
	if bPerms, ok := bRaw["permissions"].(map[string]any); ok && len(bPerms) > 0 {
		pPerms, _ := pRaw["permissions"].(map[string]any)
		if pPerms == nil {
			pPerms = make(map[string]any)
		}
		if bAllow, ok := bPerms["allow"].([]any); ok {
			pAllow, _ := pPerms["allow"].([]any)
			seen := make(map[string]bool)
			for _, item := range pAllow {
				if s, ok := item.(string); ok {
					seen[s] = true
				}
			}
			for _, item := range bAllow {
				if s, ok := item.(string); ok && !seen[s] {
					pAllow = append(pAllow, item)
					seen[s] = true
				}
			}
			pPerms["allow"] = pAllow
		}
		pRaw["permissions"] = pPerms
	}

	// 3. Merge MCP servers
	if bServers, ok := bRaw["mcpServers"].(map[string]any); ok && len(bServers) > 0 {
		pServers, ok := pRaw["mcpServers"].(map[string]any)
		if !ok || pServers == nil {
			pServers = make(map[string]any)
		}
		for k, v := range bServers {
			if _, exists := pServers[k]; !exists {
				pServers[k] = v
			}
		}
		pRaw["mcpServers"] = pServers
	}

	// 4. Merge trustedWorkspaces
	if bWorkspaces, ok := bRaw["trustedWorkspaces"].([]any); ok && len(bWorkspaces) > 0 {
		pWorkspaces, _ := pRaw["trustedWorkspaces"].([]any)
		seen := make(map[string]bool)
		for _, item := range pWorkspaces {
			if s, ok := item.(string); ok {
				seen[s] = true
			}
		}
		for _, item := range bWorkspaces {
			if s, ok := item.(string); ok && !seen[s] {
				pWorkspaces = append(pWorkspaces, item)
				seen[s] = true
			}
		}
		pRaw["trustedWorkspaces"] = pWorkspaces
	}

	updated, err := json.MarshalIndent(pRaw, "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(profileSettingsPath), 0700)
	return WriteFileAtomic(profileSettingsPath, updated, 0600)
}

// SyncUnifiedStore links conversation data, brain, annotations, summaries, and history
// from the base environment ($HOME/.gemini/antigravity-cli) into the profile.
// Any existing isolated data in the profile is merged into base before creating symlinks,
// ensuring zero data loss and seamless conversation sharing across all profiles.
func SyncUnifiedStore(baseHome, profileDir string) error {
	if baseHome == "" || profileDir == "" {
		return nil
	}

	for _, sub := range []string{
		filepath.Join(".gemini", "antigravity-cli"),
		filepath.Join(".gemini", "antigravity"),
	} {
		baseSubDir := filepath.Join(baseHome, sub)
		profileSubDir := filepath.Join(profileDir, sub)

		// Only synchronize if the base user environment actually has this directory
		if info, err := os.Stat(baseSubDir); os.IsNotExist(err) || !info.IsDir() {
			continue
		}
		_ = os.MkdirAll(profileSubDir, 0700)

		// 1. Shared directories
		for _, dirName := range []string{"conversations", "brain", "annotations"} {
			baseDir := filepath.Join(baseSubDir, dirName)
			profileTarget := filepath.Join(profileSubDir, dirName)

			_ = os.MkdirAll(baseDir, 0700)
			_ = linkOrMergeDir(baseDir, profileTarget, profileDir)
		}

		// 2. Shared files
		for _, fileName := range []string{"conversation_summaries.db", "history.jsonl", "jetbox_summaries_proto.pb"} {
			baseFile := filepath.Join(baseSubDir, fileName)
			profileFile := filepath.Join(profileSubDir, fileName)

			_ = linkOrMergeFile(baseFile, profileFile, profileDir)
		}
	}

	return nil
}

func isCurrentProcessHome(profileDir string) bool {
	currHome := os.Getenv("HOME")
	if currHome == "" || profileDir == "" {
		return false
	}
	return NormalizePath(currHome) == NormalizePath(profileDir)
}

func linkOrMergeDir(baseDir, profileTarget, profileDir string) error {
	info, err := os.Lstat(profileTarget)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(profileTarget)
			if err == nil && (target == baseDir || NormalizePath(target) == NormalizePath(baseDir)) {
				return nil
			}
			_ = os.Remove(profileTarget)
		} else if info.IsDir() {
			// Merge contents into baseDir before replacing with symlink
			entries, _ := os.ReadDir(profileTarget)
			for _, entry := range entries {
				src := filepath.Join(profileTarget, entry.Name())
				dst := filepath.Join(baseDir, entry.Name())
				if _, dstErr := os.Stat(dst); os.IsNotExist(dstErr) {
					if renErr := os.Rename(src, dst); renErr != nil {
						_ = copyRecursive(src, dst)
					}
				}
			}

			if isCurrentProcessHome(profileDir) {
				return nil
			}

			_ = os.RemoveAll(profileTarget)
		} else {
			_ = os.Remove(profileTarget)
		}
	}

	if isCurrentProcessHome(profileDir) {
		return nil
	}

	return os.Symlink(baseDir, profileTarget)
}

func linkOrMergeFile(baseFile, profileFile, profileDir string) error {
	info, err := os.Lstat(profileFile)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(profileFile)
			if err == nil && (target == baseFile || NormalizePath(target) == NormalizePath(baseFile)) {
				return nil
			}
			_ = os.Remove(profileFile)
		} else if info.Mode().IsRegular() {
			if _, baseErr := os.Stat(baseFile); os.IsNotExist(baseErr) {
				_ = os.Rename(profileFile, baseFile)
			} else {
				if filepath.Base(profileFile) == "history.jsonl" {
					mergeHistoryFiles(profileFile, baseFile)
				} else if filepath.Base(profileFile) == "conversation_summaries.db" {
					mergeSummariesDB(profileFile, baseFile)
				}
				if isCurrentProcessHome(profileDir) {
					return nil
				}
				_ = os.Remove(profileFile)
				_ = os.Remove(profileFile + "-wal")
				_ = os.Remove(profileFile + "-shm")
			}
		} else {
			_ = os.RemoveAll(profileFile)
		}
	}

	if isCurrentProcessHome(profileDir) {
		return nil
	}

	if _, err := os.Stat(baseFile); err == nil {
		return os.Symlink(baseFile, profileFile)
	}
	return nil
}

func mergeHistoryFiles(srcFile, dstFile string) {
	srcBytes, err := os.ReadFile(srcFile)
	if err != nil || len(srcBytes) == 0 {
		return
	}
	dstBytes, err := os.ReadFile(dstFile)
	if err != nil {
		_ = os.WriteFile(dstFile, srcBytes, 0600)
		return
	}

	dstLines := strings.Split(string(dstBytes), "\n")
	seen := make(map[string]bool, len(dstLines))
	for _, l := range dstLines {
		t := strings.TrimSpace(l)
		if t != "" {
			seen[t] = true
		}
	}

	var toAppend []string
	for _, l := range strings.Split(string(srcBytes), "\n") {
		t := strings.TrimSpace(l)
		if t != "" && !seen[t] {
			seen[t] = true
			toAppend = append(toAppend, t)
		}
	}

	if len(toAppend) > 0 {
		f, err := os.OpenFile(dstFile, os.O_APPEND|os.O_WRONLY, 0600)
		if err == nil {
			defer f.Close()
			for _, line := range toAppend {
				_, _ = f.WriteString(line + "\n")
			}
		}
	}
}

func mergeSummariesDB(srcDB, dstDB string) {
	if _, err := exec.LookPath("sqlite3"); err == nil {
		query := fmt.Sprintf("ATTACH DATABASE '%s' AS p; INSERT OR IGNORE INTO conversation_summaries SELECT * FROM p.conversation_summaries; DETACH DATABASE p;", srcDB)
		cmd := exec.Command("sqlite3", dstDB, query)
		_ = cmd.Run()
	}
}

func copyRecursive(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyRecursive(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}

