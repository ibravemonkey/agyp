package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

func TestAddCmd_InvalidName(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	buf := new(bytes.Buffer)
	addCmd.SetOut(buf)
	addCmd.SetErr(buf)

	err := addCmd.RunE(addCmd, []string{"auto"})
	if err == nil {
		t.Fatalf("expected error for reserved profile name 'auto'")
	}
}

func TestAddCmd_ExistingAuthenticatedProfile(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	profileName := "test-auth-profile"
	pDir, err := profile.Create(profileName)
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// Write mock valid token
	tokenPath := filepath.Join(pDir, ".gemini", "oauth_creds.json")
	_ = os.MkdirAll(filepath.Dir(tokenPath), 0700)
	_ = os.WriteFile(tokenPath, []byte(`{"access_token":"mock-token"}`), 0600)

	buf := new(bytes.Buffer)
	addCmd.SetOut(buf)
	addCmd.SetErr(buf)

	err = addCmd.RunE(addCmd, []string{profileName})
	if err == nil {
		t.Fatalf("expected error when adding already authenticated profile")
	}
	if !strings.Contains(err.Error(), "уже существует") {
		t.Errorf("expected error to contain 'уже существует', got: %v", err)
	}
}

func TestAddCmd_ExistingUnauthenticatedProfile(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	profileName := "test-unauth-profile"
	t.Setenv("PATH", tempHome)
	_, err := profile.Create(profileName)
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// Ensure profile exists but has NO token
	if profile.HasProfileToken(profileName) {
		t.Fatalf("expected profile to have no token")
	}

	buf := new(bytes.Buffer)
	addCmd.SetContext(t.Context())
	addCmd.SetOut(buf)
	addCmd.SetErr(buf)

	// Running addCmd will attempt to run `agy` which will fail in test env,
	// but the critical check is that it does NOT fail with "уже существует".
	err = addCmd.RunE(addCmd, []string{profileName})
	if err != nil && strings.Contains(err.Error(), "уже существует") {
		t.Fatalf("expected unauthenticated profile NOT to fail with 'уже существует', got: %v", err)
	}

	// Output buffer should indicate re-authentication was launched
	outStr := buf.String()
	if !strings.Contains(outStr, "существует, но не авторизован") {
		t.Errorf("expected output to mention 'существует, но не авторизован', got: %s", outStr)
	}

	// Profile directory must NOT have been deleted upon failure because it previously existed
	exists, _, _ := profile.Exists(profileName)
	if !exists {
		t.Errorf("expected existing unauthenticated profile directory to be preserved on failure")
	}
}
