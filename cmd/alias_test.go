package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

func TestAliasCommand(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYS_DIR", filepath.Join(tempHome, ".agys"))

	// Test with no profiles
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"alias"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("alias command failed: %v", err)
	}

	if !strings.Contains(buf.String(), "No active profiles found") {
		t.Errorf("expected 'No active profiles found', got: %s", buf.String())
	}

	// Create test profiles and test aliases generated
	_, err := profile.Create("work-prof")
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}
	_, err = profile.Create("personal-prof")
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	buf.Reset()
	rootCmd.SetArgs([]string{"alias"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("alias command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "alias agy_work_prof=\"agys run work-prof --\"") {
		t.Errorf("expected alias for work-prof, got:\n%s", out)
	}
	if !strings.Contains(out, "alias agy_personal_prof=\"agys run personal-prof --\"") {
		t.Errorf("expected alias for personal-prof, got:\n%s", out)
	}
}
