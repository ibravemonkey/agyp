package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quaywin/agys/pkg/profile"
)

func TestListCommand(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYS_DIR", filepath.Join(tempHome, ".agys"))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// List when empty
	rootCmd.SetArgs([]string{"list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("list command failed on empty: %v", err)
	}
	if !strings.Contains(buf.String(), "No profiles found") {
		t.Errorf("expected 'No profiles found', got: %s", buf.String())
	}

	// Create test profiles
	_, err := profile.Create("prof-a")
	if err != nil {
		t.Fatalf("failed to create prof-a: %v", err)
	}
	_, err = profile.Create("prof-b")
	if err != nil {
		t.Fatalf("failed to create prof-b: %v", err)
	}
	_ = profile.SetCurrent("prof-a")

	buf.Reset()
	rootCmd.SetArgs([]string{"list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "prof-a (default)") {
		t.Errorf("expected 'prof-a (default)' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "prof-b") {
		t.Errorf("expected 'prof-b' in output, got:\n%s", out)
	}
}
