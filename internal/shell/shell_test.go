package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallShims(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewSetupManager()

	created, err := mgr.InstallShims(tempDir)
	if err != nil {
		t.Fatalf("InstallShims failed: %v", err)
	}

	if len(created) != 3 {
		t.Errorf("expected 3 shims created, got %d", len(created))
	}

	agyShim := filepath.Join(tempDir, "agy")
	info, err := os.Stat(agyShim)
	if err != nil {
		t.Fatalf("agy shim not found: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("expected agy shim to be executable, mode: %v", info.Mode())
	}

	agyContent, _ := os.ReadFile(agyShim)
	if !strings.Contains(string(agyContent), "exec agyp run") {
		t.Errorf("agy shim missing exec agyp run: %s", string(agyContent))
	}

	agyqShim := filepath.Join(tempDir, "agyq")
	infoQ, err := os.Stat(agyqShim)
	if err != nil {
		t.Fatalf("agyq shim not found: %v", err)
	}
	if infoQ.Mode()&0111 == 0 {
		t.Errorf("expected agyq shim to be executable, mode: %v", infoQ.Mode())
	}

	agyaShim := filepath.Join(tempDir, "agya")
	infoA, err := os.Stat(agyaShim)
	if err != nil {
		t.Fatalf("agya shim not found: %v", err)
	}
	if infoA.Mode()&0111 == 0 {
		t.Errorf("expected agya shim to be executable, mode: %v", infoA.Mode())
	}
	agyaContent, _ := os.ReadFile(agyaShim)
	if !strings.Contains(string(agyaContent), "exec agyp run --auto") {
		t.Errorf("agya shim missing exec agyp run --auto: %s", string(agyaContent))
	}
}

func TestInstallShims_PreservesRealBinary(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewSetupManager()

	realAgy := filepath.Join(tempDir, "agy")
	dummyBinary := []byte("\xca\xfe\xba\xbe-mach-o-binary")
	if err := os.WriteFile(realAgy, dummyBinary, 0755); err != nil {
		t.Fatalf("failed to write dummy binary: %v", err)
	}

	created, err := mgr.InstallShims(tempDir)
	if err != nil {
		t.Fatalf("InstallShims failed: %v", err)
	}

	// agy should NOT be recreated/overwritten, but agya and agyq should be created
	if len(created) != 2 {
		t.Errorf("expected 2 shims (agya, agyq) to be created, got %v", created)
	}
	content, _ := os.ReadFile(realAgy)
	if string(content) != string(dummyBinary) {
		t.Errorf("real agy binary was unexpectedly overwritten! content: %s", string(content))
	}
}

func TestConfigureShellRC_Idempotent(t *testing.T) {
	tempDir := t.TempDir()
	rcPath := filepath.Join(tempDir, ".zshrc")
	mgr := NewSetupManager()

	initialContent := "# Existing user config\nexport FOO=BAR\n"
	_ = os.WriteFile(rcPath, []byte(initialContent), 0644)

	// First run
	ok, err := mgr.ConfigureShellRC(rcPath, []string{"work1", "work2"})
	if err != nil || !ok {
		t.Fatalf("first ConfigureShellRC failed: %v", err)
	}

	data1, _ := os.ReadFile(rcPath)
	str1 := string(data1)
	if !strings.Contains(str1, BlockStartMarker) || !strings.Contains(str1, "alias agy1=") {
		t.Fatalf("missing agyp block: %s", str1)
	}
	if !strings.Contains(str1, "export FOO=BAR") {
		t.Errorf("existing user config was deleted: %s", str1)
	}

	// Count occurrences of start marker
	if strings.Count(str1, BlockStartMarker) != 1 {
		t.Errorf("expected exactly 1 start marker, got %d", strings.Count(str1, BlockStartMarker))
	}

	// Second run (updating profiles)
	ok, err = mgr.ConfigureShellRC(rcPath, []string{"work1", "work2", "work3"})
	if err != nil || !ok {
		t.Fatalf("second ConfigureShellRC failed: %v", err)
	}

	data2, _ := os.ReadFile(rcPath)
	str2 := string(data2)
	if strings.Count(str2, BlockStartMarker) != 1 {
		t.Errorf("expected exactly 1 start marker after update, got %d: %s", strings.Count(str2, BlockStartMarker), str2)
	}
	if !strings.Contains(str2, "alias agy3=") {
		t.Errorf("missing updated profile work3: %s", str2)
	}

	// Uninstall
	uninstalled, err := mgr.UninstallShellRC(rcPath)
	if err != nil || !uninstalled {
		t.Fatalf("UninstallShellRC failed: %v", err)
	}

	data3, _ := os.ReadFile(rcPath)
	str3 := string(data3)
	if strings.Contains(str3, BlockStartMarker) {
		t.Errorf("managed block still present after uninstall: %s", str3)
	}
	if !strings.Contains(str3, "export FOO=BAR") {
		t.Errorf("user config was removed during uninstall: %s", str3)
	}
}

func TestSyncProfileShims(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewSetupManager()

	// 1. Initial sync with 2 profiles: "agy1" and "work"
	created, err := mgr.SyncProfileShims(tempDir, []string{"agy1", "work"})
	if err != nil {
		t.Fatalf("SyncProfileShims failed: %v", err)
	}

	// "agy1" creates: agy1, use1
	// "work" creates: agy2, use2, work, use-work
	expectedFiles := []string{"agy1", "use1", "agy2", "use2", "work", "use-work"}
	for _, f := range expectedFiles {
		path := filepath.Join(tempDir, f)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected shim %s not found in %v", f, created)
		} else if info.Mode()&0111 == 0 {
			t.Errorf("expected shim %s to be executable", f)
		}
	}

	// Check content of agy2 launcher
	agy2Content, _ := os.ReadFile(filepath.Join(tempDir, "agy2"))
	if !strings.Contains(string(agy2Content), `agyp use "work"`) || !strings.Contains(string(agy2Content), `exec agyp run "work"`) {
		t.Errorf("agy2 shim content incorrect: %s", string(agy2Content))
	}

	// 2. Remove "work" and sync with only "personal"
	_, err = mgr.SyncProfileShims(tempDir, []string{"personal"})
	if err != nil {
		t.Fatalf("second SyncProfileShims failed: %v", err)
	}

	// Old shims "agy2", "use2", "work", "use-work" must be removed
	oldShims := []string{"agy2", "use2", "work", "use-work"}
	for _, f := range oldShims {
		path := filepath.Join(tempDir, f)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("old shim %s was not cleaned up", f)
		}
	}

	// New shims "agy1", "use1", "personal", "use-personal" should exist
	newShims := []string{"agy1", "use1", "personal", "use-personal"}
	for _, f := range newShims {
		path := filepath.Join(tempDir, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("new shim %s not found", f)
		}
	}

	// 3. Test that core binaries (like agyp itself) are never removed, even if they contain the header
	agysBin := filepath.Join(tempDir, "agyp")
	if err := os.WriteFile(agysBin, []byte("fake binary with "+profileShimHeader), 0755); err != nil {
		t.Fatalf("failed to write fake agyp: %v", err)
	}
	_, err = mgr.SyncProfileShims(tempDir, []string{"personal"})
	if err != nil {
		t.Fatalf("SyncProfileShims failed: %v", err)
	}
	if _, err := os.Stat(agysBin); err != nil {
		t.Errorf("agyp binary was unexpectedly deleted by SyncProfileShims: %v", err)
	}
}
