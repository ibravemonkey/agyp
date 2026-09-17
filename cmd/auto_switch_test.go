package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

// TestE2E_AutomaticAccountSwitching tests the complete lifecycle of multi-account automatic rotation:
// 1. Creation of multiple accounts (agy1, agy2, agy3) with credentials.
// 2. Active conversation owned by agy1.
// 3. In-flight 429 quota exhaustion detected by statusline hook.
// 4. Token hot-swap and pending switch marker verification.
// 5. Automatic conversation migration to agy2 upon resume with auto mode.
// 6. Cascading auto-switch to agy3 when agy2 also runs out of quota.
func TestE2E_AutomaticAccountSwitching(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYP_DIR", filepath.Join(tempHome, ".agyp"))

	// 1. Setup 3 accounts
	acc1 := "agy1"
	acc2 := "agy2"
	acc3 := "agy3"

	dir1, err := profile.Create(acc1)
	if err != nil {
		t.Fatalf("failed to create acc1: %v", err)
	}
	dir2, err := profile.Create(acc2)
	if err != nil {
		t.Fatalf("failed to create acc2: %v", err)
	}
	dir3, err := profile.Create(acc3)
	if err != nil {
		t.Fatalf("failed to create acc3: %v", err)
	}

	// Setup distinct credentials for each account
	tok1 := `{"token":{"access_token":"token_acc1_initial","refresh_token":"ref_acc1"}}`
	tok2 := `{"token":{"access_token":"token_acc2","refresh_token":"ref_acc2"}}`
	tok3 := `{"token":{"access_token":"token_acc3","refresh_token":"ref_acc3"}}`

	_ = profile.WriteTokenToProfile(dir1, tok1)
	_ = profile.WriteTokenToProfile(dir2, tok2)
	_ = profile.WriteTokenToProfile(dir3, tok3)

	// Set initial quota: agy1 = 100%, agy2 = 90%, agy3 = 70%
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

	_ = profile.SaveCachedQuota(acc1, makeQuota(1.0))
	_ = profile.SaveCachedQuota(acc2, makeQuota(0.90))
	_ = profile.SaveCachedQuota(acc3, makeQuota(0.70))

	// 2. Create active conversation in agy1
	convID := "conv-e2e-workflow"
	brainDir := filepath.Join(dir1, ".gemini", "antigravity-cli", "brain", convID)
	logsDir := filepath.Join(brainDir, ".system_generated", "logs")
	if err := os.MkdirAll(logsDir, 0700); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(logsDir, "transcript.jsonl"), []byte(`{"step_index":0,"content":"initial user task"}`+"\n"), 0600)
	_ = profile.SaveLastConversation(convID)

	// Verify owner before exhaustion
	initialOwner, err := profile.FindProfileByConversation(convID)
	if err != nil || initialOwner != acc1 {
		t.Fatalf("expected initial owner to be %q, got %q (err: %v)", acc1, initialOwner, err)
	}

	// 3. In-flight: agy1 hits 429 quota exhaustion (Fraction5H = 0.0)
	exhaustedDetails := &profile.ModelQuotaDetails{
		Fraction5H:     0.0,
		FractionWeekly: 0.10,
	}

	// Update agy1 cached quota to 0.0
	_ = profile.SaveCachedQuota(acc1, makeQuota(0.0))

	alert := profile.CheckAndHandleInFlightQuota(context.Background(), acc1, dir1, convID, exhaustedDetails, false)
	if !strings.Contains(alert, "429") || !strings.Contains(alert, acc2) {
		t.Errorf("expected statusline alert mentioning 429 and %q, got: %q", acc2, alert)
	}

	// 4. Verify token was pre-staged from agy2 into agy1 directory
	stagedToken, err := profile.ReadRawTokenData(dir1)
	if err != nil || !strings.Contains(string(stagedToken), "token_acc2") {
		t.Errorf("expected agy1 token to be pre-staged with agy2 credentials, got: %s", string(stagedToken))
	}

	// 5. User resumes session via auto mode: agya -c / agyp run --auto -c
	resProf, resArgs, err := resolveResumeProfile("auto", []string{"-c"})
	if err != nil {
		t.Fatalf("resolveResumeProfile auto failed: %v", err)
	}

	if resProf != acc2 {
		t.Fatalf("expected auto-switch from exhausted %q to %q, got %q", acc1, acc2, resProf)
	}
	if len(resArgs) != 1 || resArgs[0] != "--conversation="+convID {
		t.Fatalf("expected -c to be replaced with --conversation=%s, got %v", convID, resArgs)
	}

	// Conversation brain must now reside in agy2
	owner2, err := profile.FindProfileByConversation(convID)
	if err != nil || owner2 != acc2 {
		t.Fatalf("expected conversation to be migrated to %q, got %q (err: %v)", acc2, owner2, err)
	}

	// 6. Cascading exhaustion: agy2 now also drops to 0% quota
	_ = profile.SaveCachedQuota(acc2, makeQuota(0.0))

	resProf2, resArgs2, err := resolveResumeProfile("auto", []string{"-c"})
	if err != nil {
		t.Fatalf("cascading resolveResumeProfile failed: %v", err)
	}

	if resProf2 != acc3 {
		t.Fatalf("expected cascading auto-switch to %q, got %q", acc3, resProf2)
	}
	if len(resArgs2) != 1 || resArgs2[0] != "--conversation="+convID {
		t.Fatalf("expected -c to be replaced with --conversation=%s, got %v", convID, resArgs2)
	}

	// Conversation brain must now reside in agy3
	owner3, err := profile.FindProfileByConversation(convID)
	if err != nil || owner3 != acc3 {
		t.Fatalf("expected conversation to be migrated to %q, got %q (err: %v)", acc3, owner3, err)
	}
}
