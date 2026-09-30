package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

func TestEnsureDefaultModelAndEffort(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("AGYP_DIR", tempDir)
	_ = profile.SaveCachedDiscoveredModels(&profile.DiscoveredModels{
		FetchedAt:   time.Now(),
		LatestFlash: profile.DefaultGeminiModel,
		LatestPro:   "gemini-3.1-pro",
		AllModels:   []string{profile.DefaultGeminiModel, "gemini-3.1-pro"},
	})

	t.Run("Default behavior when no model or effort specified", func(t *testing.T) {
		var agyArgs []string
		res := EnsureDefaultModelAndEffort(agyArgs)
		expected := []string{"--model", profile.DefaultGeminiModel, "--effort", "high"}
		if len(res) != len(expected) {
			t.Fatalf("expected %v, got %v", expected, res)
		}
		for i, v := range expected {
			if res[i] != v {
				t.Errorf("at index %d: expected %q, got %q", i, v, res[i])
			}
		}
	})

	t.Run("Preserves custom model and skips default model injection", func(t *testing.T) {
		agyArgs := []string{"--model", "claude-3-5-sonnet"}
		res := EnsureDefaultModelAndEffort(agyArgs)
		if len(res) != 2 || res[1] != "claude-3-5-sonnet" {
			t.Errorf("expected custom model to be preserved without extra effort, got %v", res)
		}
	})

	t.Run("Appends effort high for gemini flash models when effort not specified", func(t *testing.T) {
		agyArgs := []string{"--model", "gemini-3.8-flash"}
		res := EnsureDefaultModelAndEffort(agyArgs)
		expected := []string{"--model", "gemini-3.8-flash", "--effort", "high"}
		if len(res) != len(expected) {
			t.Fatalf("expected %v, got %v", expected, res)
		}
	})

	t.Run("Ignores subcommands", func(t *testing.T) {
		subcommands := []string{"models", "agent", "agents", "help", "mcp", "mic-serve", "remote-control"}
		for _, sub := range subcommands {
			args := []string{sub}
			res := EnsureDefaultModelAndEffort(args)
			if len(res) != 1 || res[0] != sub {
				t.Errorf("subcommand %q was modified: %v", sub, res)
			}
		}
	})
}

func TestIsInteractiveSession(t *testing.T) {
	cases := []struct {
		args     []string
		expected bool
	}{
		{[]string{}, true},
		{[]string{"--dangerously-skip-permissions"}, true},
		{[]string{"-p", "hello"}, false},
		{[]string{"--print", "hello"}, false},
		{[]string{"--prompt=hello"}, false},
		{[]string{"-h"}, false},
		{[]string{"--help"}, false},
		{[]string{"models"}, false},
		{[]string{"auth", "status"}, false},
		{[]string{"mcp", "list"}, false},
	}

	for _, c := range cases {
		got := IsInteractiveSession(c.args)
		if got != c.expected {
			t.Errorf("IsInteractiveSession(%v) = %v, expected %v", c.args, got, c.expected)
		}
	}
}

func TestResolveResumeProfile(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	_, err := profile.Create("work")
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	var errBuf bytes.Buffer
	p, args, err := ResolveResumeProfile("work", []string{"--model", "gemini-3.8-flash"}, &errBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != "work" {
		t.Errorf("expected profile 'work', got %q", p)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %v", args)
	}
}

type mockRunner struct {
	calledOpts RunOptions
	err        error
}

func (m *mockRunner) Run(ctx context.Context, opts RunOptions) error {
	m.calledOpts = opts
	return m.err
}

func TestMockRunner_Execution(t *testing.T) {
	m := &mockRunner{}
	opts := RunOptions{
		ProfileName: "test-profile",
		AgyArgs:     []string{"--prompt", "test"},
	}

	err := m.Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.calledOpts.ProfileName != "test-profile" {
		t.Errorf("expected profile 'test-profile', got %q", m.calledOpts.ProfileName)
	}
}

func TestResolveResumeProfile_ExhaustedQuotaFailover(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	dir1, err := profile.Create("agy1")
	if err != nil {
		t.Fatalf("failed to create agy1: %v", err)
	}
	_, err = profile.Create("agy2")
	if err != nil {
		t.Fatalf("failed to create agy2: %v", err)
	}

	tok1 := `{"token":{"access_token":"tok1","refresh_token":"ref1"}}`
	tok2 := `{"token":{"access_token":"tok2","refresh_token":"ref2"}}`
	_ = profile.WriteTokenToProfile(dir1, tok1)
	dir2, _ := profile.GetProfileDir("agy2")
	_ = profile.WriteTokenToProfile(dir2, tok2)

	makeQuota := func(pct float64) *profile.QuotaSummary {
		return &profile.QuotaSummary{
			Groups: []profile.QuotaGroup{
				{
					DisplayName: "gemini",
					Buckets: []profile.QuotaBucket{
						{Window: "5h", RemainingFraction: pct},
					},
				},
			},
		}
	}

	// agy1 is exhausted (0%), agy2 is healthy (100%)
	_ = profile.SaveCachedQuota("agy1", makeQuota(0.0))
	_ = profile.SaveCachedQuota("agy2", makeQuota(1.0))

	convID := "conv-failover-test"
	brainDir := filepath.Join(dir1, ".gemini", "antigravity-cli", "brain", convID)
	_ = os.MkdirAll(brainDir, 0700)
	_ = profile.SaveLastConversation(convID)

	var errBuf bytes.Buffer
	// Even though explicit "agy1" is requested with -c, it should failover to agy2 because agy1 has 0% quota
	target, args, err := ResolveResumeProfile("agy1", []string{"-c"}, &errBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if target != "agy2" {
		t.Errorf("expected failover to agy2, got %q (stderr: %s)", target, errBuf.String())
	}
	if len(args) != 1 || args[0] != "--conversation="+convID {
		t.Errorf("expected args [--conversation=%s], got %v", convID, args)
	}

	// Verify conversation brain is now in agy2
	newOwner, err := profile.FindProfileByConversation(convID)
	if err != nil || newOwner != "agy2" {
		t.Errorf("expected conversation owner to be agy2, got %q (err: %v)", newOwner, err)
	}
}
