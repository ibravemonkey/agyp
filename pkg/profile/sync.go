package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
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

	// 2. Shell & Dev Toolchains (shared tooling: .local, .cargo, .ssh, .gitconfig)
	_ = safeSymlink(filepath.Join(baseHome, ".local"), filepath.Join(profileDir, ".local"))
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

	// 5. Tools (skill-compass, bin with rtk, sqz, notify-sound)
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "skill-compass"), filepath.Join(geminiConfigDir, "skill-compass"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "bin"), filepath.Join(geminiConfigDir, "bin"))

	// 6. Plugins & Extensions (plugins/ directory, plugins.json)
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "plugins"), filepath.Join(geminiConfigDir, "plugins"))
	_ = safeSymlink(filepath.Join(baseHome, ".gemini", "config", "plugins.json"), filepath.Join(geminiConfigDir, "plugins.json"))

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
