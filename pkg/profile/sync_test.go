package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncBaseEnvironmentToProfile(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYS_DIR", filepath.Join(tempHome, ".agys"))

	// 1. Create dummy base environment
	baseSkillsDir := filepath.Join(tempHome, ".gemini", "config", "skills")
	_ = os.MkdirAll(baseSkillsDir, 0755)
	_ = os.WriteFile(filepath.Join(baseSkillsDir, "skill1.md"), []byte("# Skill 1"), 0644)

	baseMcpConfig := filepath.Join(tempHome, ".gemini", "config", "mcp_config.json")
	_ = os.WriteFile(baseMcpConfig, []byte(`{"mcpServers":{"test":{"command":"node"}}}`), 0644)

	baseSettings := filepath.Join(tempHome, ".gemini", "antigravity-cli", "settings.json")
	_ = os.MkdirAll(filepath.Dir(baseSettings), 0755)
	_ = os.WriteFile(baseSettings, []byte(`{"model":"gemini-3.8-flash","mcpServers":{"rtk":{"command":"rtk"}}}`), 0644)

	baseGitConfig := filepath.Join(tempHome, ".gitconfig")
	_ = os.WriteFile(baseGitConfig, []byte("[user]\nname = Test\n"), 0644)

	// 2. Create profile
	pDir, err := Create("testprof")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 3. Sync base environment
	err = SyncBaseEnvironmentToProfile(pDir)
	if err != nil {
		t.Fatalf("SyncBaseEnvironmentToProfile failed: %v", err)
	}

	// 4. Verify skills symlink
	profileSkills := filepath.Join(pDir, ".gemini", "config", "skills")
	skillInfo, err := os.Lstat(profileSkills)
	if err != nil {
		t.Fatalf("profile skills not found: %v", err)
	}
	if skillInfo.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected profile skills to be a symlink")
	}

	// Verify content through symlink
	content, err := os.ReadFile(filepath.Join(profileSkills, "skill1.md"))
	if err != nil || string(content) != "# Skill 1" {
		t.Errorf("skill content mismatch: %s", string(content))
	}

	// 5. Verify mcp_config.json symlink
	profileMcp := filepath.Join(pDir, ".gemini", "config", "mcp_config.json")
	mcpInfo, err := os.Lstat(profileMcp)
	if err != nil || mcpInfo.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected mcp_config.json to be a symlink")
	}

	// 6. Verify merged settings.json
	pSettings := filepath.Join(pDir, ".gemini", "antigravity-cli", "settings.json")
	pData, err := os.ReadFile(pSettings)
	if err != nil {
		t.Fatalf("profile settings.json missing: %v", err)
	}
	var pRaw map[string]any
	_ = json.Unmarshal(pData, &pRaw)
	servers, ok := pRaw["mcpServers"].(map[string]any)
	if !ok || servers["rtk"] == nil {
		t.Errorf("expected merged rtk mcpServer in profile settings.json, got: %v", pRaw)
	}

	// 7. Verify .gitconfig symlink
	pGit := filepath.Join(pDir, ".gitconfig")
	gitInfo, err := os.Lstat(pGit)
	if err != nil || gitInfo.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected .gitconfig to be a symlink")
	}
}

func TestSyncBaseEnvironmentToAllProfiles(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYS_DIR", filepath.Join(tempHome, ".agys"))

	_, _ = Create("prof1")
	_, _ = Create("prof2")

	synced, err := SyncBaseEnvironmentToAllProfiles()
	if err != nil {
		t.Fatalf("SyncBaseEnvironmentToAllProfiles failed: %v", err)
	}
	if len(synced) != 2 {
		t.Errorf("expected 2 profiles synced, got %d (%v)", len(synced), synced)
	}
}
