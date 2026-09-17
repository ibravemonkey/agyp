package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)
func TestCleanCmd_HelpAndFlags(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"clean", "--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("clean --help failed: %v", err)
	}

	out := buf.String()
	for _, expected := range []string{"--ttl", "--older-than", "--keep-last", "--dry-run", "--all", "--cache-only"} {
		if !bytes.Contains([]byte(out), []byte(expected)) {
			t.Errorf("expected clean help to contain %q, got: %s", expected, out)
		}
	}
	_ = cleanCmd.Flags().Set("help", "false")
	_ = rootCmd.Flags().Set("help", "false")
}

func TestCleanCmd_DryRunExecution(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("AGYP_REAL_HOME", tempDir)
	t.Setenv("AGYP_DIR", filepath.Join(tempDir, ".agyp"))

	profileName := "cleancmdtest"
	profileDir := filepath.Join(tempDir, ".agyp", "profiles", profileName)
	cliBrain := filepath.Join(profileDir, ".gemini", "antigravity-cli", "brain")
	if err := os.MkdirAll(cliBrain, 0755); err != nil {
		t.Fatalf("failed to create cliBrain: %v", err)
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"clean", profileName, "--dry-run", "--ttl=7d", "--keep-last=1"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("clean execution failed: %v", err)
	}

	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("[DRY-RUN]")) {
		t.Errorf("expected dry-run output, got: %s", out)
	}
}

func TestCleanCmd_ActualExecution(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("AGYP_REAL_HOME", tempDir)
	t.Setenv("AGYP_DIR", filepath.Join(tempDir, ".agyp"))

	profileName := "cleancmdactual"
	profileDir := filepath.Join(tempDir, ".agyp", "profiles", profileName)
	cliBrain := filepath.Join(profileDir, ".gemini", "antigravity-cli", "brain")
	_ = os.MkdirAll(cliBrain, 0755)

	// Old session
	oldDir := filepath.Join(cliBrain, "old-session")
	_ = os.MkdirAll(oldDir, 0755)
	trPath := filepath.Join(oldDir, "transcript.jsonl")
	_ = os.WriteFile(trPath, []byte("old transcript data"), 0644)
	mTime := time.Now().Add(-40 * 24 * time.Hour)
	_ = os.Chtimes(trPath, mTime, mTime)
	_ = os.Chtimes(oldDir, mTime, mTime)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"clean", profileName, "--force", "--ttl=1d", "--keep-last=0"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("clean execution failed: %v", err)
	}

	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("Cleanup finished")) {
		t.Errorf("expected 'Cleanup finished' in output, got: %s", out)
	}

	// Verify directory deleted
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("expected old-session to be deleted, got err: %v", err)
	}
}
