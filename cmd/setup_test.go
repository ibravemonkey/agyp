package cmd

import (
	"testing"
)

func TestSetupShellCommandFlags(t *testing.T) {
	if setupShellCmd.Name() != "setup-shell" {
		t.Errorf("expected command name 'setup-shell', got %q", setupShellCmd.Name())
	}

	hasSetupAlias := false
	for _, a := range setupShellCmd.Aliases {
		if a == "setup" {
			hasSetupAlias = true
			break
		}
	}
	if !hasSetupAlias {
		t.Errorf("expected alias 'setup' on setupShellCmd")
	}

	uFlag := setupShellCmd.Flags().Lookup("uninstall")
	if uFlag == nil || uFlag.Shorthand != "u" {
		t.Errorf("expected -u / --uninstall flag on setupShellCmd")
	}

	rcFlag := setupShellCmd.Flags().Lookup("rc")
	if rcFlag == nil {
		t.Errorf("expected --rc flag on setupShellCmd")
	}

	binFlag := setupShellCmd.Flags().Lookup("bin-dir")
	if binFlag == nil {
		t.Errorf("expected --bin-dir flag on setupShellCmd")
	}
}
