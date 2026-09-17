package cmd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

func TestSyncCmdFlags(t *testing.T) {
	if syncCmd.Name() != "sync" {
		t.Errorf("expected command name 'sync', got %q", syncCmd.Name())
	}

	qFlag := syncCmd.Flags().Lookup("quiet")
	if qFlag == nil || qFlag.Shorthand != "q" {
		t.Errorf("expected -q / --quiet flag on syncCmd")
	}
}

func TestSyncCmdExecution(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	_, _ = profile.Create("prof1")

	var buf bytes.Buffer
	syncCmd.SetOut(&buf)
	syncCmd.SetArgs([]string{})

	err := syncCmd.Execute()
	if err != nil {
		t.Fatalf("syncCmd.Execute failed: %v", err)
	}
}
