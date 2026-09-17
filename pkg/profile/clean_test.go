package profile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)
func TestParseTTL(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"", DefaultTTL, false},
		{"   ", DefaultTTL, false},
		{"14d", 14 * 24 * time.Hour, false},
		{"7day", 7 * 24 * time.Hour, false},
		{"1days", 1 * 24 * time.Hour, false},
		{"30 days", 30 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"1week", 7 * 24 * time.Hour, false},
		{"1month", 30 * 24 * time.Hour, false},
		{"2months", 60 * 24 * time.Hour, false},
		{"48h", 48 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"invalid", 0, true},
		{"-5d", 0, true},
		{"-10h", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseTTL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParseTTL(%q) expected error, got nil", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("ParseTTL(%q) unexpected error: %v", tc.input, err)
				}
				if got != tc.expected {
					t.Errorf("ParseTTL(%q) = %v, expected %v", tc.input, got, tc.expected)
				}
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	if FormatBytes(500) != "500 B" {
		t.Errorf("expected '500 B', got %q", FormatBytes(500))
	}
	if FormatBytes(1024) != "1.0 KB" {
		t.Errorf("expected '1.0 KB', got %q", FormatBytes(1024))
	}
	if FormatBytes(10*1024*1024) != "10.0 MB" {
		t.Errorf("expected '10.0 MB', got %q", FormatBytes(10*1024*1024))
	}
}

func TestCleanProfile_DryRunAndActual(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("AGYP_REAL_HOME", tempDir)
	t.Setenv("AGYP_DIR", filepath.Join(tempDir, ".agyp"))

	profileName := "testclean"
	profileDir := filepath.Join(tempDir, ".agyp", "profiles", profileName)
	cliBrain := filepath.Join(profileDir, ".gemini", "antigravity-cli", "brain")
	if err := os.MkdirAll(cliBrain, 0755); err != nil {
		t.Fatalf("failed to create cliBrain: %v", err)
	}
	// s1: 30 days old (should be cleaned if KeepLast < 4)
	// s2: 20 days old (should be cleaned if KeepLast < 3)
	// s3: 5 days old (newer than 10d TTL)
	// s4: 1 day old (newest)
	now := time.Now()
	sessions := []struct {
		id      string
		age     time.Duration
		content string
	}{
		{"conv-old-1", 30 * 24 * time.Hour, "old transcript 1"},
		{"conv-old-2", 20 * 24 * time.Hour, "old transcript 2"},
		{"conv-mid-3", 5 * 24 * time.Hour, "mid transcript 3"},
		{"conv-new-4", 1 * 24 * time.Hour, "new transcript 4"},
	}

	var histLines []byte
	for _, s := range sessions {
		convDir := filepath.Join(cliBrain, s.id)
		logDir := filepath.Join(convDir, ".system_generated", "logs")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			t.Fatalf("failed to create log dir: %v", err)
		}
		trPath := filepath.Join(logDir, "transcript.jsonl")
		if err := os.WriteFile(trPath, []byte(s.content), 0644); err != nil {
			t.Fatalf("failed to write transcript: %v", err)
		}
		// Set ModTime
		mTime := now.Add(-s.age)
		_ = os.Chtimes(trPath, mTime, mTime)
		_ = os.Chtimes(convDir, mTime, mTime)

		// Add history record
		histEntry := map[string]interface{}{
			"conversationId": s.id,
			"display":        s.id + " display",
			"timestamp":      mTime.Unix(),
		}
		line, _ := json.Marshal(histEntry)
		histLines = append(histLines, line...)
		histLines = append(histLines, '\n')
	}

	histPath := filepath.Join(profileDir, ".gemini", "antigravity-cli", "history.jsonl")
	if err := os.WriteFile(histPath, histLines, 0644); err != nil {
		t.Fatalf("failed to write history.jsonl: %v", err)
	}

	// Create a dummy cache directory
	cacheDir := filepath.Join(profileDir, "Caches")
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "cache.dat"), []byte("cached data dummy"), 0644)

	// Step 1: Dry-run test with TTL 10 days, KeepLast 2
	// Expected: conv-old-1 and conv-old-2 are older than 10d and beyond the 2 newest (s4, s3)
	// So 2 sessions to clean, 2 kept, plus Caches
	opts := CleanOptions{
		TTL:      10 * 24 * time.Hour,
		KeepLast: 2,
		DryRun:   true,
	}

	report, err := CleanProfile(profileName, opts)
	if err != nil {
		t.Fatalf("CleanProfile dry-run failed: %v", err)
	}

	if report.SessionsScanned != 4 {
		t.Errorf("expected 4 sessions scanned, got %d", report.SessionsScanned)
	}
	if report.SessionsCleaned != 2 {
		t.Errorf("expected 2 sessions cleaned in dry-run, got %d", report.SessionsCleaned)
	}
	if report.SessionsKept != 2 {
		t.Errorf("expected 2 sessions kept in dry-run, got %d", report.SessionsKept)
	}
	if report.BytesFreed <= 0 {
		t.Errorf("expected BytesFreed > 0, got %d", report.BytesFreed)
	}

	// In DryRun, directories must still exist
	if _, err := os.Stat(filepath.Join(cliBrain, "conv-old-1")); err != nil {
		t.Errorf("conv-old-1 should still exist in DryRun, got err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "cache.dat")); err != nil {
		t.Errorf("cache.dat should still exist in DryRun, got err: %v", err)
	}

	// Step 2: Actual run
	opts.DryRun = false
	reportActual, err := CleanProfile(profileName, opts)
	if err != nil {
		t.Fatalf("CleanProfile actual failed: %v", err)
	}

	if reportActual.SessionsCleaned != 2 {
		t.Errorf("expected 2 sessions cleaned, got %d", reportActual.SessionsCleaned)
	}

	// Old sessions should be removed
	if _, err := os.Stat(filepath.Join(cliBrain, "conv-old-1")); !os.IsNotExist(err) {
		t.Errorf("conv-old-1 should have been deleted")
	}
	if _, err := os.Stat(filepath.Join(cliBrain, "conv-old-2")); !os.IsNotExist(err) {
		t.Errorf("conv-old-2 should have been deleted")
	}

	// New sessions must remain intact
	if _, err := os.Stat(filepath.Join(cliBrain, "conv-mid-3")); err != nil {
		t.Errorf("conv-mid-3 should have been kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cliBrain, "conv-new-4")); err != nil {
		t.Errorf("conv-new-4 should have been kept: %v", err)
	}

	// Caches directory should have been removed/emptied
	if _, err := os.Stat(filepath.Join(cacheDir, "cache.dat")); !os.IsNotExist(err) {
		t.Errorf("cache.dat should have been deleted")
	}

	// Verify history.jsonl pruned
	histData, err := os.ReadFile(histPath)
	if err != nil {
		t.Fatalf("failed reading history.jsonl after clean: %v", err)
	}
	histStr := string(histData)
	if bytes.Contains([]byte(histStr), []byte("conv-old-1")) {
		t.Errorf("history.jsonl still contains deleted conv-old-1")
	}
	if bytes.Contains([]byte(histStr), []byte("conv-old-2")) {
		t.Errorf("history.jsonl still contains deleted conv-old-2")
	}
	if !bytes.Contains([]byte(histStr), []byte("conv-mid-3")) {
		t.Errorf("history.jsonl should still contain conv-mid-3")
	}
	if !bytes.Contains([]byte(histStr), []byte("conv-new-4")) {
		t.Errorf("history.jsonl should still contain conv-new-4")
	}
}

func TestCleanProfile_KeepLastProtection(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("AGYP_REAL_HOME", tempDir)
	t.Setenv("AGYP_DIR", filepath.Join(tempDir, ".agyp"))
	profileName := "testkeeplast"
	profileDir := filepath.Join(tempDir, ".agyp", "profiles", profileName)
	cliBrain := filepath.Join(profileDir, ".gemini", "antigravity-cli", "brain")
	_ = os.MkdirAll(cliBrain, 0755)

	// Create 3 sessions all older than 100 days
	now := time.Now()
	for _, id := range []string{"old-1", "old-2", "old-3"} {
		convDir := filepath.Join(cliBrain, id)
		_ = os.MkdirAll(convDir, 0755)
		trPath := filepath.Join(convDir, "transcript.jsonl")
		_ = os.WriteFile(trPath, []byte("data"), 0644)
		mTime := now.Add(-100 * 24 * time.Hour)
		_ = os.Chtimes(trPath, mTime, mTime)
		_ = os.Chtimes(convDir, mTime, mTime)
	}

	// KeepLast is 3: Even though all 3 are older than TTL, 0 sessions should be deleted!
	opts := CleanOptions{
		TTL:      1 * 24 * time.Hour,
		KeepLast: 3,
		DryRun:   false,
	}

	report, err := CleanProfile(profileName, opts)
	if err != nil {
		t.Fatalf("CleanProfile failed: %v", err)
	}

	if report.SessionsCleaned != 0 {
		t.Errorf("expected 0 sessions cleaned due to KeepLast, got %d", report.SessionsCleaned)
	}
	if report.SessionsKept != 3 {
		t.Errorf("expected 3 sessions kept, got %d", report.SessionsKept)
	}
}
