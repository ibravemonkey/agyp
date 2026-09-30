package profile

import (
	"strings"
	"testing"
)

func TestDetectTerminalProtocol(t *testing.T) {
	t.Run("ghostty default safe ascii in agy statusline", func(t *testing.T) {
		t.Setenv("AGYP_RENDER", "")
		t.Setenv("TERM_PROGRAM", "ghostty")
		t.Setenv("GHOSTTY_RESOURCES_DIR", "")
		if p := DetectTerminalProtocol(); p != ProtocolASCII {
			t.Errorf("expected ProtocolASCII for ghostty by default in statusline, got %v", p)
		}
	})

	t.Run("ghostty manual override kitty", func(t *testing.T) {
		t.Setenv("AGYP_RENDER", "kitty")
		t.Setenv("TERM_PROGRAM", "ghostty")
		if p := DetectTerminalProtocol(); p != ProtocolKitty {
			t.Errorf("expected ProtocolKitty on explicit AGYP_RENDER=kitty, got %v", p)
		}
	})

	t.Run("zed terminal fallback", func(t *testing.T) {
		t.Setenv("AGYP_RENDER", "")
		t.Setenv("TERM_PROGRAM", "zed")
		t.Setenv("GHOSTTY_RESOURCES_DIR", "")
		if p := DetectTerminalProtocol(); p != ProtocolASCII {
			t.Errorf("expected ProtocolASCII for zed, got %v", p)
		}
	})

	t.Run("manual override kitty", func(t *testing.T) {
		t.Setenv("AGYP_RENDER", "kitty")
		t.Setenv("TERM_PROGRAM", "zed")
		if p := DetectTerminalProtocol(); p != ProtocolKitty {
			t.Errorf("expected ProtocolKitty on AGYP_RENDER=kitty, got %v", p)
		}
	})

	t.Run("manual override ascii", func(t *testing.T) {
		t.Setenv("AGYP_RENDER", "ascii")
		t.Setenv("TERM_PROGRAM", "ghostty")
		if p := DetectTerminalProtocol(); p != ProtocolASCII {
			t.Errorf("expected ProtocolASCII on AGYP_RENDER=ascii, got %v", p)
		}
	})
}

func TestBuildHUDLines(t *testing.T) {
	quota := &ModelQuotaDetails{
		Fraction5H:         0.84,
		CompactReset5H:     "1h14m",
		FractionWeekly:     0.72,
		CompactResetWeekly: "3d8h",
	}
	tel := TokenTelemetry{
		InputTokens:     2300,
		OutputTokens:    918,
		DurationSeconds: 2.8,
		Speed:           179.2,
		CtxPct:          12,
		HasCtx:          true,
	}

	lines := BuildHUDLines("agy2", "igor", "master", "idle", "gemini-3.8-flash", "high", 0.0024, 12, true, quota, true, tel)
	if len(lines) != 5 {
		t.Fatalf("expected 5 HUD lines, got %d", len(lines))
	}

	// Line 1 contains profile and status
	if !strings.Contains(lines[0], "[agy2]") || !strings.Contains(lines[0], "ONLINE") {
		t.Errorf("unexpected Line 1: %s", lines[0])
	}

	// Line 2 contains model & effort
	if !strings.Contains(lines[1], "gemini-3.8-flash") || !strings.Contains(lines[1], "(high)") {
		t.Errorf("unexpected Line 2: %s", lines[1])
	}

	// Line 3 contains branch and folder
	if !strings.Contains(lines[2], "master") {
		t.Errorf("unexpected Line 3: %s", lines[2])
	}

	// Line 4 contains 5h and 7d quotas
	if !strings.Contains(lines[3], "5h") || !strings.Contains(lines[3], "84%") || !strings.Contains(lines[3], "7d") || !strings.Contains(lines[3], "72%") {
		t.Errorf("unexpected Line 4: %s", lines[3])
	}

	// Line 5 contains ctx and speed
	if !strings.Contains(lines[4], "12% ctx") || !strings.Contains(lines[4], "179.2/s") {
		t.Errorf("unexpected Line 5: %s", lines[4])
	}
}

func TestFormatStatusLineHUD_Zed(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "zed")
	t.Setenv("AGYP_RENDER", "ascii")

	quota := &ModelQuotaDetails{
		Fraction5H:         1.0,
		CompactReset5H:     "4h58m",
		FractionWeekly:     1.0,
		CompactResetWeekly: "6d23h",
	}
	tel := TokenTelemetry{Speed: 69.6}

	out := FormatStatusLineHUD("agy2", "igor", "master", "idle", "gemini-3.8-flash", "high", 0, 0, true, quota, true, tel)
	lines := strings.Split(out, "\n")
	if len(lines) < 5 {
		t.Fatalf("expected at least 5 lines, got %d:\n%s", len(lines), out)
	}

	// Verify Braille characters exist on line 0
	if !strings.Contains(lines[0], "⡤⡀") {
		t.Errorf("expected Braille characters in output line 0, got: %s", lines[0])
	}

	// Verify HUD content on right
	if !strings.Contains(lines[0], "[agy2]") {
		t.Errorf("expected profile name in line 0, got: %s", lines[0])
	}
	if !strings.Contains(lines[1], "gemini-3.8-flash") {
		t.Errorf("expected model name in line 1, got: %s", lines[1])
	}
	if !strings.Contains(lines[3], "5h") || !strings.Contains(lines[3], "7d") {
		t.Errorf("expected quotas in line 3, got: %s", lines[3])
	}
	if !strings.Contains(lines[4], "69.6/s") {
		t.Errorf("expected speed in line 4, got: %s", lines[4])
	}
}

func TestFormatStatusLineHUD_Ghostty(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "ghostty")
	t.Setenv("AGYP_RENDER", "kitty")

	quota := &ModelQuotaDetails{
		Fraction5H:         1.0,
		CompactReset5H:     "4h58m",
		FractionWeekly:     1.0,
		CompactResetWeekly: "6d23h",
	}
	tel := TokenTelemetry{Speed: 69.6}

	out := FormatStatusLineHUD("agy2", "igor", "master", "idle", "gemini-3.8-flash", "high", 0, 0, true, quota, true, tel)
	lines := strings.Split(out, "\n")
	if len(lines) < 5 {
		t.Fatalf("expected at least 5 lines, got %d:\n%s", len(lines), out)
	}

	// If PNG was found, line 0 should contain Kitty escape sequence \033_G
	// If not found, it falls back cleanly to Braille
	if strings.Contains(lines[0], "\033_G") {
		// Kitty protocol sequence verified
		if !strings.Contains(lines[0], "a=T,f=100") {
			t.Errorf("expected kitty parameters in sequence, got: %s", lines[0])
		}
	} else {
		// Braille fallback verified
		if !strings.Contains(lines[0], "⡤⡀") {
			t.Errorf("expected Braille fallback if image not found, got: %s", lines[0])
		}
	}
}
