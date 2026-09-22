package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

func TestStatsCmd_Execution(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))
	t.Setenv("NO_COLOR", "1")

	// Seed some token usage
	_ = profile.RecordTokenUsage("agy1", "gemini-3.8-flash", "test-c1", 5000, 1000, 20000)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"stats"})

	// Reset flags
	statsJSON = false
	statsDays = 7
	statsProfile = ""

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("statsCmd execute error: %v", err)
	}

	result := out.String()
	if !strings.Contains(result, "Antigravity Token Usage Statistics") {
		t.Errorf("expected header in output, got: %s", result)
	}
	if !strings.Contains(result, "5,000") {
		t.Errorf("expected input tokens '5,000' in output, got: %s", result)
	}
	if !strings.Contains(result, "agy1") {
		t.Errorf("expected profile 'agy1' in output, got: %s", result)
	}
}

func TestStatsCmd_JSONOutput(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	_ = profile.RecordTokenUsage("work", "gemini-3.8-flash", "test-c2", 8000, 2000, 50000)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"stats", "--json"})

	statsJSON = false
	statsDays = 7
	statsProfile = ""

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("statsCmd execute error: %v", err)
	}

	var parsed profile.TokenStatsSummary
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse json output: %v, raw: %s", err, out.String())
	}

	if parsed.TotalInput != 8000 {
		t.Errorf("expected TotalInput 8000, got %d", parsed.TotalInput)
	}
	if parsed.TotalTokens != 60000 {
		t.Errorf("expected TotalTokens 60000, got %d", parsed.TotalTokens)
	}
}

func TestStatsResetCmd(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	_ = profile.RecordTokenUsage("work", "gemini-3.8-flash", "test-c3", 1000, 100, 0)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"stats", "reset"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("statsResetCmd error: %v", err)
	}

	summary, err := profile.GetTokenStatsSummary(7, "")
	if err != nil {
		t.Fatalf("GetTokenStatsSummary after reset failed: %v", err)
	}
	if summary.TotalTokens != 0 {
		t.Errorf("expected 0 total tokens after reset, got %d", summary.TotalTokens)
	}
}
