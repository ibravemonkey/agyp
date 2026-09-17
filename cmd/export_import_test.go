package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

func TestExportImport_PlaintextRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("AGYP_REAL_HOME", tempDir)
	t.Setenv("AGYP_DIR", filepath.Join(tempDir, ".agyp"))

	// Create a profile to export
	pName := "exportplain"
	pDir := filepath.Join(tempDir, ".agyp", "profiles", pName)
	if err := os.MkdirAll(pDir, 0755); err != nil {
		t.Fatalf("failed to create profile dir: %v", err)
	}
	testFile := filepath.Join(pDir, "test_config.json")
	if err := os.WriteFile(testFile, []byte(`{"key":"plain_value"}`), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	exportArchive := filepath.Join(tempDir, "plain_backup.tar.gz")

	// Export
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"export", pName, "-o", exportArchive})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	if _, err := os.Stat(exportArchive); err != nil {
		t.Fatalf("export archive not found: %v", err)
	}

	// Import into new profile
	importedName := "importplain"
	buf.Reset()
	rootCmd.SetArgs([]string{"import", exportArchive, importedName})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("import failed: %v", err)
	}

	importedFile := filepath.Join(tempDir, ".agyp", "profiles", importedName, "test_config.json")
	data, err := os.ReadFile(importedFile)
	if err != nil {
		t.Fatalf("imported file missing: %v", err)
	}
	if !strings.Contains(string(data), "plain_value") {
		t.Fatalf("imported file content mismatch: %s", string(data))
	}
}

func TestExportImport_EncryptedRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("AGYP_REAL_HOME", tempDir)
	t.Setenv("AGYP_DIR", filepath.Join(tempDir, ".agyp"))

	pName := "exportenc"
	pDir := filepath.Join(tempDir, ".agyp", "profiles", pName)
	if err := os.MkdirAll(pDir, 0755); err != nil {
		t.Fatalf("failed to create profile dir: %v", err)
	}
	secretFile := filepath.Join(pDir, "secret_token.txt")
	if err := os.WriteFile(secretFile, []byte("super-secret-oauth-token-12345"), 0600); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}

	exportArchive := filepath.Join(tempDir, "secure_backup.agyp.enc")
	password := "SecretPassphrase2026!"

	// 1. Export with --encrypt and --password
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"export", pName, "--encrypt", "--password=" + password, "-o", exportArchive})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("encrypted export failed: %v", err)
	}

	// Verify the archive file is indeed encrypted (has AGYP_ENC magic header)
	file, err := os.Open(exportArchive)
	if err != nil {
		t.Fatalf("failed to open export archive: %v", err)
	}
	isEnc, _, err := profile.IsEncryptedArchive(file)
	file.Close()
	if err != nil || !isEnc {
		t.Fatalf("expected encrypted archive format, isEnc=%v, err=%v", isEnc, err)
	}

	// 2. Import with wrong password should fail
	buf.Reset()
	rootCmd.SetArgs([]string{"import", exportArchive, "wrongpassprofile", "--password=WrongPassword!"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatalf("expected error importing with wrong password, got success")
	}

	// 3. Import with correct password should succeed
	buf.Reset()
	importedName := "importenc"
	rootCmd.SetArgs([]string{"import", exportArchive, importedName, "--password=" + password})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("encrypted import failed with correct password: %v", err)
	}

	importedFile := filepath.Join(tempDir, ".agyp", "profiles", importedName, "secret_token.txt")
	data, err := os.ReadFile(importedFile)
	if err != nil {
		t.Fatalf("imported file missing: %v", err)
	}
	if string(data) != "super-secret-oauth-token-12345" {
		t.Fatalf("imported file content mismatch: got %q", string(data))
	}
}

func TestExportImport_EncryptedWithEnvVar(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("AGYP_REAL_HOME", tempDir)
	t.Setenv("AGYP_DIR", filepath.Join(tempDir, ".agyp"))
	t.Setenv("AGYP_ENCRYPTION_KEY", "EnvPasswordVal777!")

	pName := "exportenv"
	pDir := filepath.Join(tempDir, ".agyp", "profiles", pName)
	_ = os.MkdirAll(pDir, 0755)
	_ = os.WriteFile(filepath.Join(pDir, "env_data.txt"), []byte("env-protected-data"), 0600)

	exportArchive := filepath.Join(tempDir, "env_backup.agyp.enc")

	// Export using env var for password
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"export", pName, "-e", "-o", exportArchive})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("export with env var failed: %v", err)
	}

	// Import using env var for password and inferring target name from filename
	buf.Reset()
	rootCmd.SetArgs([]string{"import", exportArchive})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("import with env var failed: %v", err)
	}

	// Inferred name should be "env_backup"
	importedFile := filepath.Join(tempDir, ".agyp", "profiles", "env_backup", "env_data.txt")
	data, err := os.ReadFile(importedFile)
	if err != nil {
		t.Fatalf("imported file missing under inferred name: %v", err)
	}
	if string(data) != "env-protected-data" {
		t.Fatalf("data mismatch: %s", string(data))
	}
}
