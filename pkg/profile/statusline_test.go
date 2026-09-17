package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionContextSaveAndGet(t *testing.T) {
	t.Setenv("HERDR_PANE_ID", "")
	tempDir := t.TempDir()

	// Initial check on empty
	if _, ok := GetSessionContext(tempDir); ok {
		t.Errorf("expected GetSessionContext to return false on empty dir")
	}

	// Save context
	state := &SessionContextState{
		UsedPercentage:      42.4,
		InputTokens:         1500,
		CacheReadTokens:     3500,
		CacheCreationTokens: 500,
		ModelID:             "claude-3-7-sonnet",
		ModelDisplayName:    "Claude 3.7 Sonnet",
	}
	if err := SaveSessionContext(tempDir, state); err != nil {
		t.Fatalf("SaveSessionContext failed: %v", err)
	}

	pct, ok := GetSessionContext(tempDir)
	if !ok {
		t.Fatalf("expected GetSessionContext to return true")
	}
	if pct != 42 {
		t.Errorf("expected pct=42, got %d", pct)
	}

	// Test expired context (> 2 hours)
	state.UpdatedAt = time.Now().Add(-3 * time.Hour)
	data, _ := json.Marshal(state)
	_ = os.WriteFile(filepath.Join(tempDir, sessionContextFilename), data, 0600)

	if _, ok := GetSessionContext(tempDir); ok {
		t.Errorf("expected expired context to return false")
	}
}

func TestHandleStatusLine(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	pDir, err := Create("test-statusline-profile")
	if err != nil {
		t.Fatalf("Create profile failed: %v", err)
	}
	_ = SetCurrent("test-statusline-profile")
	t.Setenv("HOME", pDir)
	t.Setenv("AGYP_PROFILE", "test-statusline-profile")

	payloadJSON := `{
		"conversation_title": "Investigate memory leak",
		"cost": 0.0042,
		"model": {
			"id": "claude-3-7-sonnet",
			"display_name": "Claude 3.7 Sonnet"
		},
		"context_window": {
			"used_percentage": 33.2,
			"current_usage": {
				"input_tokens": 120,
				"cache_read_input_tokens": 800,
				"cache_creation_input_tokens": 0
			}
		},
		"quota": {
			"gemini-5h": {
				"remaining_fraction": 0.85,
				"reset_time": "2026-08-27T12:00:00Z"
			}
		}
	}`

	var stdout, stderr bytes.Buffer
	err = HandleStatusLine(context.Background(), strings.NewReader(payloadJSON), &stdout, &stderr)
	if err != nil {
		t.Fatalf("HandleStatusLine returned error: %v", err)
	}

	pct, ok := GetSessionContext(pDir)
	if !ok {
		t.Fatalf("expected context to be saved for profile")
	}
	if pct != 33 {
		t.Errorf("expected pct=33, got %d", pct)
	}

	state, okState := GetSessionContextState(pDir)
	if !okState || state == nil {
		t.Fatalf("expected GetSessionContextState to succeed")
	}
	if state.ConversationTitle != "Investigate memory leak" {
		t.Errorf("expected ConversationTitle 'Investigate memory leak', got %q", state.ConversationTitle)
	}
	if state.Cost != 0.0042 {
		t.Errorf("expected Cost 0.0042, got %f", state.Cost)
	}

	// Verify .active_model was updated
	activeData, err := os.ReadFile(filepath.Join(pDir, ".active_model"))
	if err != nil || strings.TrimSpace(string(activeData)) != "claude-3-7-sonnet" {
		t.Errorf("expected .active_model to be claude-3-7-sonnet, got: %s", string(activeData))
	}
	// Verify stdout contains formatted statusline text
	outStr := stdout.String()
	if !strings.Contains(outStr, "test-statusline-profile") {
		t.Errorf("expected stdout to contain profile name, got: %q", outStr)
	}
	if !strings.Contains(outStr, "33%") {
		t.Errorf("expected stdout to contain '33%%', got: %q", outStr)
	}
	if !strings.Contains(outStr, "claude-3-7-sonnet") {
		t.Errorf("expected stdout to contain 'claude-3-7-sonnet', got: %q", outStr)
	}
}

func TestFormatStatusLineText(t *testing.T) {
	// 1. Full data without colors, with effort and cost
	quota := &ModelQuotaDetails{
		Fraction5H:         0.95,
		CompactReset5H:     "1h26m",
		FractionWeekly:     0.79,
		CompactResetWeekly: "6h35m",
	}
	s := FormatStatusLineText("davidnguyen", "gemini-3.7-flash", "high", 0.0042, 5, true, quota, false)
	expected := "[davidnguyen] · 5% ctx\ngemini-3.7-flash (high) · 5H: 95% (1h26m) · Week: 79% (6h35m) · $0.0042"
	if s != expected {
		t.Errorf("FormatStatusLineText() = %q, expected %q", s, expected)
	}

	// 2. Full data with colors
	sColored := FormatStatusLineText("davidnguyen", "gemini-3.7-flash", "high", 0.0042, 5, true, quota, true)
	if !strings.Contains(sColored, "[davidnguyen]") || !strings.Contains(sColored, "\033[") {
		t.Errorf("expected colored string to contain ANSI escapes, got: %q", sColored)
	}

	// 3. No quota data (offline or error), zero cost (omitted)
	sNoQuota := FormatStatusLineText("davidnguyen", "gemini-3.7-flash", "", 0.0, 10, true, nil, false)
	expectedNoQuota := "[davidnguyen] · 10% ctx\ngemini-3.7-flash"
	if sNoQuota != expectedNoQuota {
		t.Errorf("FormatStatusLineText() = %q, expected %q", sNoQuota, expectedNoQuota)
	}
	// 4. Critical quota (< 5%) with colors
	quotaCrit := &ModelQuotaDetails{
		Fraction5H:     0.03,
		CompactReset5H: "45m",
	}
	sCrit := FormatStatusLineText("davidnguyen", "gemini-3.7-flash", "high", 0.05, 85, true, quotaCrit, true)
	// Should contain red ANSI for 85% ctx and red ANSI for 3% quota
	if !strings.Contains(sCrit, "\033[1;31m") {
		t.Errorf("expected critical alert color ANSI code in output, got: %q", sCrit)
	}
}

func TestTokenTelemetryFormatting(t *testing.T) {
	// 1. FormatTokenCount tests
	if got := FormatTokenCount(0); got != "0" {
		t.Errorf("FormatTokenCount(0) = %q, expected '0'", got)
	}
	if got := FormatTokenCount(918); got != "918" {
		t.Errorf("FormatTokenCount(918) = %q, expected '918'", got)
	}
	if got := FormatTokenCount(2300); got != "2.3K" {
		t.Errorf("FormatTokenCount(2300) = %q, expected '2.3K'", got)
	}
	if got := FormatTokenCount(130000); got != "130K" {
		t.Errorf("FormatTokenCount(130000) = %q, expected '130K'", got)
	}
	if got := FormatTokenCount(1500000); got != "1.5M" {
		t.Errorf("FormatTokenCount(1500000) = %q, expected '1.5M'", got)
	}
	if got := FormatTokenCount(2000000); got != "2M" {
		t.Errorf("FormatTokenCount(2000000) = %q, expected '2M'", got)
	}

	// 2. FormatDurationSec tests
	if got := FormatDurationSec(2.8); got != "2.8s" {
		t.Errorf("FormatDurationSec(2.8) = %q, expected '2.8s'", got)
	}
	if got := FormatDurationSec(74.0); got != "1m14s" {
		t.Errorf("FormatDurationSec(74.0) = %q, expected '1m14s'", got)
	}

	// 3. FormatSpeed tests
	if got := FormatSpeed(179.2); got != "179.2/s" {
		t.Errorf("FormatSpeed(179.2) = %q, expected '179.2/s'", got)
	}

	// 4. FormatTokenTelemetry plain text
	tel := TokenTelemetry{
		InputTokens:     2300,
		OutputTokens:    918,
		CacheTokens:     130000,
		CtxPct:          12,
		HasCtx:          true,
		DurationSeconds: 2.8,
		Speed:           179.2,
	}
	plain := FormatTokenTelemetry(tel, false)
	expectedPlain := "\uf090 2.3K   \uf08b 918   \uf1c0 130K (12%)   \uf017 2.8s   \uf0e4 179.2/s"
	if plain != expectedPlain {
		t.Errorf("FormatTokenTelemetry() plain = %q, expected %q", plain, expectedPlain)
	}

	// 5. FormatTokenTelemetry colored
	colored := FormatTokenTelemetry(tel, true)
	if !strings.Contains(colored, "\033[") || !strings.Contains(colored, "2.3K") || !strings.Contains(colored, "179.2/s") {
		t.Errorf("FormatTokenTelemetry() colored missing elements: %q", colored)
	}

	// 6. FormatStatusLineTextExtended with Line 3 telemetry
	quota := &ModelQuotaDetails{
		Fraction5H:         0.84,
		CompactReset5H:     "1h14m",
		FractionWeekly:     0.72,
		CompactResetWeekly: "3d8h",
	}
	fullOut := FormatStatusLineTextExtended("agy1", "my-project", "main", "Idle", "gemini-3.8-flash", "high", 0.0024, 12, true, quota, false, tel)
	lines := strings.Split(fullOut, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines in full statusline output, got %d:\n%s", len(lines), fullOut)
	}
	if lines[0] != "[agy1] · 📦 my-project ·  main ·  Idle" {
		t.Errorf("unexpected line 1: %q", lines[0])
	}
	if lines[1] != "gemini-3.8-flash (high) · 5H: 84% (1h14m) · Week: 72% (3d8h) · $0.0024" {
		t.Errorf("unexpected line 2: %q", lines[1])
	}
	if lines[2] != expectedPlain {
		t.Errorf("unexpected line 3: got %q, expected %q", lines[2], expectedPlain)
	}
}

func TestHandleStatusLine_TokenTelemetryIntegration(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_REAL_HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	pDir, err := Create("test-token-telemetry")
	if err != nil {
		t.Fatalf("Create profile failed: %v", err)
	}
	_ = SetCurrent("test-token-telemetry")
	t.Setenv("AGYP_PROFILE", "test-token-telemetry")

	payloadJSON := `{
		"conversation_title": "Build cool statusline",
		"cost": 0.0024,
		"duration_ms": 2800,
		"speed": 179.2,
		"model": {
			"id": "gemini-3.8-flash",
			"display_name": "Gemini 3.8 Flash"
		},
		"context_window": {
			"used_percentage": 12.0,
			"current_usage": {
				"input_tokens": 2300,
				"output_tokens": 918,
				"cache_read_input_tokens": 130000,
				"cache_creation_input_tokens": 0
			}
		},
		"quota": {
			"gemini-5h": {
				"remaining_fraction": 0.84,
				"reset_time": "2026-08-27T12:00:00Z"
			}
		}
	}`

	var stdout, stderr bytes.Buffer
	err = HandleStatusLine(context.Background(), strings.NewReader(payloadJSON), &stdout, &stderr)
	if err != nil {
		t.Fatalf("HandleStatusLine failed: %v", err)
	}

	outStr := stdout.String()
	if !strings.Contains(outStr, "2.3K") {
		t.Errorf("expected stdout to contain '2.3K', got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "918") {
		t.Errorf("expected stdout to contain '918', got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "130K") {
		t.Errorf("expected stdout to contain '130K', got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "2.8s") {
		t.Errorf("expected stdout to contain '2.8s', got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "179.2/s") {
		t.Errorf("expected stdout to contain '179.2/s', got:\n%s", outStr)
	}

	// Verify SessionContextState preserved the token metrics
	state, ok := GetSessionContextState(pDir)
	if !ok || state == nil {
		t.Fatalf("expected GetSessionContextState to succeed")
	}
	if state.InputTokens != 2300 {
		t.Errorf("expected InputTokens 2300, got %d", state.InputTokens)
	}
	if state.OutputTokens != 918 {
		t.Errorf("expected OutputTokens 918, got %d", state.OutputTokens)
	}
	if state.CacheReadTokens != 130000 {
		t.Errorf("expected CacheReadTokens 130000, got %d", state.CacheReadTokens)
	}
	if state.DurationSeconds != 2.8 {
		t.Errorf("expected DurationSeconds 2.8, got %f", state.DurationSeconds)
	}
	if state.Speed != 179.2 {
		t.Errorf("expected Speed 179.2, got %f", state.Speed)
	}
}

func TestSyncStatusLineSettings(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	pDir, err := Create("test-sync-sl-profile")
	if err != nil {
		t.Fatalf("Create profile failed: %v", err)
	}

	// Seed existing custom statusLine
	cliSettings := filepath.Join(pDir, ".gemini", "antigravity-cli", "settings.json")
	_ = os.MkdirAll(filepath.Dir(cliSettings), 0700)
	_ = os.WriteFile(cliSettings, []byte(`{"statusLine":{"type":"command","command":"echo custom"}}`), 0600)

	if err := SyncStatusLineSettings(pDir); err != nil {
		t.Fatalf("SyncStatusLineSettings failed: %v", err)
	}

	// Check updated settings
	data, _ := os.ReadFile(cliSettings)
	if !strings.Contains(string(data), "agyp statusline-hook") {
		t.Errorf("expected settings.json to contain 'agyp statusline-hook', got: %s", string(data))
	}

	// Check backup was created
	backupPath := filepath.Join(pDir, ".gemini", "config", statuslineBackupFile)
	bData, err := os.ReadFile(backupPath)
	if err != nil || !strings.Contains(string(bData), "echo custom") {
		t.Errorf("expected backup file to contain 'echo custom', got: %s", string(bData))
	}
}

func TestStatusLineBackwardsCompatibilityAndTitlePersistence(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	pDir, err := Create("test-compat-profile")
	if err != nil {
		t.Fatalf("Create profile failed: %v", err)
	}
	_ = SetCurrent("test-compat-profile")
	t.Setenv("HOME", pDir)
	t.Setenv("AGYP_PROFILE", "test-compat-profile")

	// Step 1: Initial payload with conversation_id, title and cost
	firstPayload := `{
		"conversation_id": "conv-persistent-123",
		"conversation_title": "Original Title",
		"cost": 0.005,
		"model": {"id": "gemini-3.8-flash", "display_name": "Gemini 3.8 Flash"},
		"context_window": {"used_percentage": 15.0}
	}`
	var out1 bytes.Buffer
	if err := HandleStatusLine(context.Background(), strings.NewReader(firstPayload), &out1, nil); err != nil {
		t.Fatalf("First HandleStatusLine failed: %v", err)
	}

	state, ok := GetSessionContextState(pDir)
	if !ok || state == nil {
		t.Fatalf("expected state to exist")
	}
	if state.ConversationID != "conv-persistent-123" {
		t.Errorf("expected 'conv-persistent-123', got %q", state.ConversationID)
	}
	if state.ConversationTitle != "Original Title" {
		t.Errorf("expected 'Original Title', got %q", state.ConversationTitle)
	}
	if state.Cost != 0.005 {
		t.Errorf("expected cost 0.005, got %f", state.Cost)
	}

	// Step 2: Subsequent turn without conversation_id or title/cost in payload (e.g. streaming update)
	secondPayload := `{
		"model": {"id": "gemini-3.8-flash", "display_name": "Gemini 3.8 Flash"},
		"context_window": {"used_percentage": 25.0}
	}`
	var out2 bytes.Buffer
	if err := HandleStatusLine(context.Background(), strings.NewReader(secondPayload), &out2, nil); err != nil {
		t.Fatalf("Second HandleStatusLine failed: %v", err)
	}

	state2, ok2 := GetSessionContextState(pDir)
	if !ok2 || state2 == nil {
		t.Fatalf("expected state2 to exist")
	}
	// Title, ID and Cost must be preserved across turns when conversation_id is omitted!
	if state2.ConversationID != "conv-persistent-123" {
		t.Errorf("expected conversation_id to persist as 'conv-persistent-123', got %q", state2.ConversationID)
	}
	if state2.ConversationTitle != "Original Title" {
		t.Errorf("expected title to persist as 'Original Title', got %q", state2.ConversationTitle)
	}
	if state2.Cost != 0.005 {
		t.Errorf("expected cost to persist as 0.005, got %f", state2.Cost)
	}
	if state2.UsedPercentage != 25.0 {
		t.Errorf("expected updated percentage 25.0, got %f", state2.UsedPercentage)
	}
	pct, okPct := GetSessionContext(pDir)
	if !okPct || pct != 25 {
		t.Errorf("expected GetSessionContext pct=25, got %d (ok=%v)", pct, okPct)
	}

	// Step 3: Switched conversation ID should cleanly reset old title and cost
	thirdPayload := `{
		"conversation_id": "conv-switched-456",
		"model": {"id": "gemini-3.8-flash", "display_name": "Gemini 3.8 Flash"},
		"context_window": {"used_percentage": 10.0}
	}`
	var out3 bytes.Buffer
	if err := HandleStatusLine(context.Background(), strings.NewReader(thirdPayload), &out3, nil); err != nil {
		t.Fatalf("Third HandleStatusLine failed: %v", err)
	}

	state3, ok3 := GetSessionContextState(pDir)
	if !ok3 || state3 == nil {
		t.Fatalf("expected state3 to exist")
	}
	if state3.ConversationID != "conv-switched-456" {
		t.Errorf("expected 'conv-switched-456', got %q", state3.ConversationID)
	}
	if state3.ConversationTitle != "" {
		t.Errorf("expected title to reset on switched conversation, got %q", state3.ConversationTitle)
	}
	if state3.Cost != 0.0 {
		t.Errorf("expected cost to reset on switched conversation, got %f", state3.Cost)
	}
}

func TestParsePayloadQuotaDisambiguation(t *testing.T) {
	payload := &StatusLinePayload{
		Quota: map[string]struct {
			RemainingFraction    float64 `json:"remaining_fraction"`
			RemainingFractionAlt float64 `json:"remainingFraction"`
			ResetTime            string  `json:"reset_time"`
			ResetTimeAlt         string  `json:"resetTime"`
			ResetInSeconds       uint64  `json:"reset_in_seconds"`
			ResetInSecondsAlt    uint64  `json:"resetInSeconds"`
		}{
			"gemini-5h": {
				RemainingFraction: 0.85,
				ResetTime:         "2026-09-06T15:00:00Z",
			},
			"gemini-weekly": {
				RemainingFraction: 0.65,
				ResetInSeconds:    7200,
			},
		},
	}
	details := parsePayloadQuota(payload.Quota)
	if details == nil {
		t.Fatalf("expected details to be parsed")
	}
	if details.Fraction5H != 0.85 {
		t.Errorf("expected Fraction5H=0.85, got %f", details.Fraction5H)
	}
	if details.FractionWeekly != 0.65 {
		t.Errorf("expected FractionWeekly=0.65, got %f", details.FractionWeekly)
	}
	if details.CompactResetWeekly == "" || details.CompactResetWeekly == "-" {
		t.Errorf("expected CompactResetWeekly to be populated via ResetInSeconds, got %q", details.CompactResetWeekly)
	}
}

func TestParsePayloadQuotaModelFamilyIsolation(t *testing.T) {
	quotaMap := map[string]struct {
		RemainingFraction    float64 `json:"remaining_fraction"`
		RemainingFractionAlt float64 `json:"remainingFraction"`
		ResetTime            string  `json:"reset_time"`
		ResetTimeAlt         string  `json:"resetTime"`
		ResetInSeconds       uint64  `json:"reset_in_seconds"`
		ResetInSecondsAlt    uint64  `json:"resetInSeconds"`
	}{
		// Alphabetically "claude" comes before "gemini"
		"claude_and_gpt_models_5h": {
			RemainingFraction: 1.0,
		},
		"claude_and_gpt_models_weekly": {
			RemainingFraction: 1.0,
		},
		"gemini_models_5h": {
			RemainingFraction: 0.0344,
			ResetInSeconds:    10740, // ~2h59m
		},
		"gemini_models_weekly": {
			RemainingFraction: 0.7373,
			ResetInSeconds:    512460,
		},
	}

	// 1. When Gemini model is active: must return Gemini quota (0.0344), NOT Claude 100%!
	geminiDetails := parsePayloadQuota(quotaMap, "gemini-3.8-flash (high)")
	if geminiDetails == nil {
		t.Fatalf("expected geminiDetails to be non-nil")
	}
	if int(geminiDetails.Fraction5H*100+0.5) != 3 {
		t.Errorf("expected Gemini 5H to be 3%%, got %f (%d%%)", geminiDetails.Fraction5H, int(geminiDetails.Fraction5H*100+0.5))
	}
	if int(geminiDetails.FractionWeekly*100+0.5) != 74 {
		t.Errorf("expected Gemini Weekly to be 74%%, got %f (%d%%)", geminiDetails.FractionWeekly, int(geminiDetails.FractionWeekly*100+0.5))
	}

	// 2. When Claude model is active: must return Claude quota (100%)
	claudeDetails := parsePayloadQuota(quotaMap, "claude-3-7-sonnet")
	if claudeDetails == nil {
		t.Fatalf("expected claudeDetails to be non-nil")
	}
	if int(claudeDetails.Fraction5H*100+0.5) != 100 {
		t.Errorf("expected Claude 5H to be 100%%, got %f", claudeDetails.Fraction5H)
	}
	if int(claudeDetails.FractionWeekly*100+0.5) != 100 {
		t.Errorf("expected Claude Weekly to be 100%%, got %f", claudeDetails.FractionWeekly)
	}
}

func TestFormatCostEdgeCases(t *testing.T) {
	testCases := []struct {
		cost     float64
		expected string
	}{
		{0.0, "$0.00"},
		{-1.0, "$0.00"},
		{0.00001, "$0.00"},
		{0.000049, "$0.00"},
		{0.0001, "$0.0001"},
		{0.0042, "$0.0042"},
		{0.05, "$0.05"},
		{0.5, "$0.50"},
		{1.0, "$1.00"},
		{1.25, "$1.25"},
		{1.2543, "$1.2543"},
		{10.5, "$10.50"},
	}
	for _, tc := range testCases {
		res := FormatCost(tc.cost)
		if res != tc.expected {
			t.Errorf("FormatCost(%v) = %q, expected %q", tc.cost, res, tc.expected)
		}
	}
}

func TestResolveConversationTitle(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	pDir, err := Create("title-test-profile")
	if err != nil {
		t.Fatalf("Create profile error: %v", err)
	}

	convID := "test-conv-abc"
	logsDir := filepath.Join(pDir, ".gemini", "antigravity-cli", "brain", convID, ".system_generated", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}
	transcriptPath := filepath.Join(logsDir, "transcript.jsonl")
	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-09-07T09:00:00Z","content":"<USER_REQUEST>\nRefactor Herdr sidebar row to show conversation title\n</USER_REQUEST>"}`
	if err := os.WriteFile(transcriptPath, []byte(transcriptContent), 0644); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	// 1. Resolve directly from transcript
	title := ResolveConversationTitle(pDir, convID)
	if title != "Refactor Herdr sidebar row to show conversation title" {
		t.Errorf("expected resolved title from transcript, got: %q", title)
	}

	// 2. Resolve from history.jsonl when transcript not found
	historyPath := filepath.Join(pDir, ".gemini", "antigravity-cli", "history.jsonl")
	historyContent := `{"display":"Fix auth login bug","timestamp":1788747000000,"workspace":"/app","conversationId":"other-conv-xyz"}` + "\n"
	if err := os.WriteFile(historyPath, []byte(historyContent), 0644); err != nil {
		t.Fatalf("failed to write history: %v", err)
	}

	title2 := ResolveConversationTitle(pDir, "other-conv-xyz")
	if title2 != "Fix auth login bug" {
		t.Errorf("expected resolved title from history, got: %q", title2)
	}

	// 3. Unknown convID returns empty string without bleeding other sessions
	title3 := ResolveConversationTitle(pDir, "non-existent-conv")
	if title3 != "" {
		t.Errorf("expected empty string for non-existent convID, got: %q", title3)
	}

	// 4. Empty convID returns empty string without bleeding other sessions
	title4 := ResolveConversationTitle(pDir, "")
	if title4 != "" {
		t.Errorf("expected empty string for empty convID, got: %q", title4)
	}
}

func TestSessionContext_PaneIsolation(t *testing.T) {
	tempDir := t.TempDir()

	// Seed Pane 1
	t.Setenv("HERDR_PANE_ID", "w1:p1")
	state1 := &SessionContextState{
		UsedPercentage:    50.0,
		ConversationTitle: "Task in Pane 1",
		ConversationID:    "conv-p1",
	}
	if err := SaveSessionContext(tempDir, state1); err != nil {
		t.Fatalf("SaveSessionContext for p1 failed: %v", err)
	}

	// Verify Pane 1 reads its own context
	loaded1, ok1 := GetSessionContextState(tempDir)
	if !ok1 || loaded1.ConversationTitle != "Task in Pane 1" {
		t.Fatalf("Pane 1 failed to load its own context: ok=%v, title=%v", ok1, loaded1)
	}

	// Switch to Pane 2 (brand new pane)
	t.Setenv("HERDR_PANE_ID", "w1:p2")
	loaded2, ok2 := GetSessionContextState(tempDir)
	if ok2 || loaded2 != nil {
		t.Fatalf("Pane 2 must NOT inherit Pane 1's context or fall back to any other file, got: %+v", loaded2)
	}

	// Seed Pane 2 with its own context
	state2 := &SessionContextState{
		UsedPercentage:    10.0,
		ConversationTitle: "Task in Pane 2",
		ConversationID:    "conv-p2",
	}
	if err := SaveSessionContext(tempDir, state2); err != nil {
		t.Fatalf("SaveSessionContext for p2 failed: %v", err)
	}

	// Verify Pane 2 loads its own context
	loaded2, ok2 = GetSessionContextState(tempDir)
	if !ok2 || loaded2.ConversationTitle != "Task in Pane 2" {
		t.Fatalf("Pane 2 failed to load its own context: ok=%v, title=%v", ok2, loaded2)
	}

	// Reset Pane 2 and verify Pane 1 is unaffected
	if err := ResetSessionContext(tempDir); err != nil {
		t.Fatalf("ResetSessionContext failed: %v", err)
	}
	if _, ok2AfterReset := GetSessionContextState(tempDir); ok2AfterReset {
		t.Fatalf("Pane 2 should be empty after reset")
	}

	// Switch back to Pane 1 and ensure it is still intact
	t.Setenv("HERDR_PANE_ID", "w1:p1")
	loaded1After, ok1After := GetSessionContextState(tempDir)
	if !ok1After || loaded1After.ConversationTitle != "Task in Pane 1" {
		t.Fatalf("Pane 1 context was corrupted by Pane 2 reset: ok=%v, state=%+v", ok1After, loaded1After)
	}
}

func TestCheckAndHandleInFlightQuota(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	p1 := "active-prof"
	p2 := "backup-prof"
	p1Dir, err := Create(p1)
	if err != nil {
		t.Fatalf("failed to create p1: %v", err)
	}
	p2Dir, err := Create(p2)
	if err != nil {
		t.Fatalf("failed to create p2: %v", err)
	}

	// Setup tokens
	tokP1 := `{"token":{"access_token":"p1_acc","refresh_token":"p1_ref"}}`
	tokP2 := `{"token":{"access_token":"p2_acc","refresh_token":"p2_ref"}}`
	_ = WriteTokenToProfile(p1Dir, tokP1)
	_ = WriteTokenToProfile(p2Dir, tokP2)

	// Healthy p2 quota cache
	p2Summary := &QuotaSummary{
		Groups: []QuotaGroup{
			{
				DisplayName: "gemini",
				Buckets: []QuotaBucket{
					{Window: "5h", RemainingFraction: 0.95},
				},
			},
		},
	}
	_ = SaveCachedQuota(p2, p2Summary)

	// 1. Quota is healthy (0.50) -> should return empty alert
	healthyQuota := &ModelQuotaDetails{
		Fraction5H:     0.50,
		FractionWeekly: 0.90,
	}
	alert := CheckAndHandleInFlightQuota(context.Background(), p1, p1Dir, "conv-1", healthyQuota, false)
	if alert != "" {
		t.Errorf("expected empty alert for healthy quota, got: %q", alert)
	}

	// 2. Quota is exhausted (0.00) -> should return switch alert and stage token
	exhaustedQuota := &ModelQuotaDetails{
		Fraction5H:     0.00,
		FractionWeekly: 0.20,
	}
	alert2 := CheckAndHandleInFlightQuota(context.Background(), p1, p1Dir, "conv-1", exhaustedQuota, false)
	if !strings.Contains(alert2, "429") || !strings.Contains(alert2, p2) {
		t.Errorf("expected alert mentioning 429 and %q, got: %q", p2, alert2)
	}

	// Verify token was hot-swapped to p2's token
	swappedData, err := ReadRawTokenData(p1Dir)
	if err != nil || !strings.Contains(string(swappedData), "p2_acc") {
		t.Errorf("expected p1 token to be hot-swapped to p2, got: %s", string(swappedData))
	}

	// Verify pending switch marker was written
	markerPath := filepath.Join(p1Dir, ".auto_switch_pending")
	if _, statErr := os.Stat(markerPath); statErr != nil {
		t.Errorf("expected .auto_switch_pending file to exist: %v", statErr)
	}
}

