package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

func TestPriorityCommand(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYS_DIR", filepath.Join(tempHome, ".agys"))

	pName := "testprio"
	_, err := profile.Create(pName)
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// Set priority
	rootCmd.SetArgs([]string{"priority", "set", pName, "42"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("priority set failed: %v", err)
	}
	if !strings.Contains(buf.String(), "set to 42") {
		t.Errorf("expected 'set to 42' in output, got: %s", buf.String())
	}

	// Get priority
	buf.Reset()
	rootCmd.SetArgs([]string{"priority", "get", pName})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("priority get failed: %v", err)
	}
	if !strings.Contains(buf.String(), "42") {
		t.Errorf("expected '42' in output, got: %s", buf.String())
	}

	// List priorities
	buf.Reset()
	rootCmd.SetArgs([]string{"priority", "list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("priority list failed: %v", err)
	}
	if !strings.Contains(buf.String(), "testprio: 42") {
		t.Errorf("expected 'testprio: 42' in list, got: %s", buf.String())
	}
}
