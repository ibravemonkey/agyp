package profile

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordTokenUsage_IncrementalDeltas(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	// 1. First turn: 1000 input, 200 output, 500 cache
	err := RecordTokenUsage("work", "gemini-3.8-flash", "conv-1", 1000, 200, 500)
	if err != nil {
		t.Fatalf("RecordTokenUsage failed: %v", err)
	}

	summary, err := GetTokenStatsSummary(7, "")
	if err != nil {
		t.Fatalf("GetTokenStatsSummary failed: %v", err)
	}
	if summary.TotalInput != 1000 {
		t.Errorf("expected TotalInput 1000, got %d", summary.TotalInput)
	}
	if summary.TotalOutput != 200 {
		t.Errorf("expected TotalOutput 200, got %d", summary.TotalOutput)
	}
	if summary.TotalCache != 500 {
		t.Errorf("expected TotalCache 500, got %d", summary.TotalCache)
	}
	if summary.TotalTokens != 1700 {
		t.Errorf("expected TotalTokens 1700, got %d", summary.TotalTokens)
	}

	// 2. Second turn in same conversation: cumulative now 1500 input, 350 output, 1000 cache
	// Deltas should be: +500 input, +150 output, +500 cache -> total should become 1500+350+1000 = 2850
	err = RecordTokenUsage("work", "gemini-3.8-flash", "conv-1", 1500, 350, 1000)
	if err != nil {
		t.Fatalf("RecordTokenUsage second turn failed: %v", err)
	}

	summary2, err := GetTokenStatsSummary(7, "")
	if err != nil {
		t.Fatalf("GetTokenStatsSummary 2 failed: %v", err)
	}
	if summary2.TotalInput != 1500 {
		t.Errorf("expected TotalInput 1500 after second turn, got %d", summary2.TotalInput)
	}
	if summary2.TotalOutput != 350 {
		t.Errorf("expected TotalOutput 350 after second turn, got %d", summary2.TotalOutput)
	}
	if summary2.TotalCache != 1000 {
		t.Errorf("expected TotalCache 1000 after second turn, got %d", summary2.TotalCache)
	}
	if summary2.TotalTokens != 2850 {
		t.Errorf("expected TotalTokens 2850 after second turn, got %d", summary2.TotalTokens)
	}

	// 3. Redundant turn with exact same numbers -> deltas 0, totals unchanged
	err = RecordTokenUsage("work", "gemini-3.8-flash", "conv-1", 1500, 350, 1000)
	if err != nil {
		t.Fatalf("RecordTokenUsage redundant turn failed: %v", err)
	}
	summary3, _ := GetTokenStatsSummary(7, "")
	if summary3.TotalTokens != 2850 {
		t.Errorf("expected TotalTokens 2850 unchanged after redundant call, got %d", summary3.TotalTokens)
	}

	// 4. Another conversation on different profile
	err = RecordTokenUsage("personal", "claude-sonnet-4-6", "conv-2", 2000, 500, 0)
	if err != nil {
		t.Fatalf("RecordTokenUsage second profile failed: %v", err)
	}

	summary4, _ := GetTokenStatsSummary(7, "")
	if summary4.TotalTokens != 2850+2500 {
		t.Errorf("expected TotalTokens 5350, got %d", summary4.TotalTokens)
	}

	// 5. Filter by profile "personal"
	summaryPersonal, err := GetTokenStatsSummary(7, "personal")
	if err != nil {
		t.Fatalf("GetTokenStatsSummary filtered failed: %v", err)
	}
	if summaryPersonal.TotalTokens != 2500 {
		t.Errorf("expected personal profile total 2500, got %d", summaryPersonal.TotalTokens)
	}
}

func TestFormatTokenStats_Render(t *testing.T) {
	summary := &TokenStatsSummary{
		DaysRequested: 7,
		TotalInput:    142500,
		TotalOutput:   18200,
		TotalCache:    450100,
		TotalTokens:   610800,
		AveragePerDay: 87257,
		DailyRows: []DailyStatRow{
			{
				Date:         time.Now().Format("2006-01-02"),
				InputTokens:  142500,
				OutputTokens: 18200,
				CacheTokens:  450100,
				TotalTokens:  610800,
				Percentage:   100.0,
			},
		},
		ProfileRows: []ProfileStatRow{
			{
				ProfileName:  "work",
				InputTokens:  142500,
				OutputTokens: 18200,
				CacheTokens:  450100,
				TotalTokens:  610800,
				Percentage:   100.0,
			},
		},
		ModelRows: []ModelStatRow{
			{
				ModelName:    "gemini-3.8-flash",
				InputTokens:  142500,
				OutputTokens: 18200,
				CacheTokens:  450100,
				TotalTokens:  610800,
				Percentage:   100.0,
			},
		},
	}

	out := FormatTokenStats(summary, false)
	if !strings.Contains(out, "Antigravity Token Usage Statistics") {
		t.Errorf("missing header in output: %s", out)
	}
	if !strings.Contains(out, "142,500") {
		t.Errorf("expected formatted number 142,500 in output: %s", out)
	}
	if !strings.Contains(out, "610,800") {
		t.Errorf("expected formatted number 610,800 in output: %s", out)
	}
	if !strings.Contains(out, "work") {
		t.Errorf("expected profile name 'work' in output: %s", out)
	}
	if !strings.Contains(out, "gemini-3.8-flash") {
		t.Errorf("expected model name in output: %s", out)
	}
}

func TestRenderProgressBar(t *testing.T) {
	// Full
	barFull := RenderProgressBar(1.0, 10, false)
	if barFull != "██████████" {
		t.Errorf("expected full bar '██████████', got %q", barFull)
	}

	// Empty
	barEmpty := RenderProgressBar(0.0, 10, false)
	if barEmpty != "░░░░░░░░░░" {
		t.Errorf("expected empty bar '░░░░░░░░░░', got %q", barEmpty)
	}

	// Half
	barHalf := RenderProgressBar(0.5, 10, false)
	if barHalf != "█████░░░░░" {
		t.Errorf("expected half bar '█████░░░░░', got %q", barHalf)
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{9, "9"},
		{999, "999"},
		{1000, "1,000"},
		{142500, "142,500"},
		{1234567, "1,234,567"},
	}

	for _, tc := range tests {
		got := FormatNumber(tc.input)
		if got != tc.expected {
			t.Errorf("FormatNumber(%d) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
