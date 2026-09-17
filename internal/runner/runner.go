package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ibravemonkey/agyp/internal/sshproxy"
	"github.com/ibravemonkey/agyp/pkg/profile"
)

// RunOptions configures the execution of an agy session.
type RunOptions struct {
	ProfileName string
	AgyArgs     []string
	RunAll      bool
	WorkingDir  string
	Stdout      io.Writer
	Stderr      io.Writer
}

// Runner defines the interface for executing agy with profile sandboxing.
type Runner interface {
	Run(ctx context.Context, opts RunOptions) error
}

type defaultRunner struct{}

// NewRunner creates a default execution Runner.
func NewRunner() Runner {
	return &defaultRunner{}
}

// Run executes the command according to the specified options.
func (r *defaultRunner) Run(ctx context.Context, opts RunOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	if opts.RunAll {
		profiles, err := profile.List()
		if err != nil {
			return err
		}
		if len(profiles) == 0 {
			return fmt.Errorf("no active profiles found")
		}
		var lastErr error
		for i, p := range profiles {
			fmt.Fprintf(opts.Stderr, "\n[agys] Executing on profile %q (%d/%d)...\n", p, i+1, len(profiles))
			subOpts := opts
			subOpts.RunAll = false
			subOpts.ProfileName = p
			if err := r.runSingleProfile(ctx, subOpts); err != nil {
				fmt.Fprintf(opts.Stderr, "[agys] Profile %q failed: %v\n", p, err)
				lastErr = err
			}
		}
		return lastErr
	}

	return r.runSingleProfile(ctx, opts)
}

func (r *defaultRunner) runSingleProfile(ctx context.Context, opts RunOptions) error {
	profileName := opts.ProfileName
	agyArgs := opts.AgyArgs

	// Extract explicit model if specified in agyArgs before defaults
	var explicitModel string
	hasExplicitModel := false
	for i := 0; i < len(agyArgs); i++ {
		if agyArgs[i] == "--model" || agyArgs[i] == "-m" {
			hasExplicitModel = true
			if i+1 < len(agyArgs) {
				explicitModel = agyArgs[i+1]
			}
			break
		}
		if strings.HasPrefix(agyArgs[i], "--model=") {
			hasExplicitModel = true
			explicitModel = strings.TrimPrefix(agyArgs[i], "--model=")
			break
		}
		if strings.HasPrefix(agyArgs[i], "-m=") {
			hasExplicitModel = true
			explicitModel = strings.TrimPrefix(agyArgs[i], "-m=")
			break
		}
	}

	agyArgs, _, _ = profile.EnsureAvailableHubPort(agyArgs)

	// Resume detection and profile migration
	var resumeErr error
	profileName, agyArgs, resumeErr = ResolveResumeProfile(profileName, agyArgs, opts.Stderr, opts.WorkingDir)
	if resumeErr != nil {
		return resumeErr
	}

	var targetProfile string
	if profile.IsAuto(profileName) {
		selected, score, err := profile.SelectBestProfile(ctx)
		if err != nil {
			return fmt.Errorf("auto profile selection failed: %w", err)
		}
		targetProfile = selected
		scoreStr := fmt.Sprintf("%.1f%%", score*100)
		if score < 0 {
			scoreStr = "N/A"
		}
		fmt.Fprintf(opts.Stderr, "[agys] Auto-selected profile %q (5h Gemini quota: %s)\n", targetProfile, scoreStr)
	} else {
		targetProfile = profileName
	}

	_ = os.Setenv("AGYS_PROFILE", targetProfile)

	profileDir, err := profile.GetProfileDir(targetProfile)
	if err != nil {
		return err
	}

	_ = profile.SyncAllTokenLocations(profileDir)
	_ = profile.EnsureOnboardingCompleted(profileDir)
	_ = profile.SyncBaseEnvironmentToProfile(profileDir)
	var expectedRefreshToken string
	if initTok, readErr := profile.ReadToken(targetProfile); readErr == nil && initTok != nil {
		expectedRefreshToken = initTok.Token.RefreshToken
	}

	profile.ArmTokenKeepAlive(targetProfile)
	_ = profile.SyncTrustedWorkspaces()

	activeModel := profile.ResolveActiveModel(profileDir, explicitModel)
	if activeModel == "" || activeModel == "gemini" {
		activeModel = profile.GetLatestGeminiModel()
	}

	if hasExplicitModel {
		_ = profile.WriteFileAtomic(filepath.Join(profileDir, ".active_model"), []byte(activeModel), 0600)
		profile.SyncModelToSettings(profileDir, activeModel)
	}

	originalUserArgs := append([]string(nil), agyArgs...)
	agyArgs = EnsureDefaultModelAndEffortWithModel(agyArgs, activeModel)

	// Persist active effort
	activeEffort := ""
	for i := 0; i < len(agyArgs); i++ {
		if agyArgs[i] == "--effort" && i+1 < len(agyArgs) {
			activeEffort = agyArgs[i+1]
			break
		} else if strings.HasPrefix(agyArgs[i], "--effort=") {
			activeEffort = strings.TrimPrefix(agyArgs[i], "--effort=")
			break
		}
	}
	if activeEffort != "" {
		_ = profile.WriteFileAtomic(filepath.Join(profileDir, ".active_effort"), []byte(activeEffort+"\n"), 0600)
	} else {
		_ = os.Remove(filepath.Join(profileDir, ".active_effort"))
	}

	// Clean up stale session context on fresh new interactive session start
	isResume := false
	resumeConvID := ""
	for i := 0; i < len(originalUserArgs); i++ {
		a := originalUserArgs[i]
		if a == "-c" || a == "--continue" || a == "-r" || a == "--resume" {
			isResume = true
			break
		}
		if strings.HasPrefix(a, "--conversation=") {
			isResume = true
			resumeConvID = strings.TrimPrefix(a, "--conversation=")
			break
		}
		if strings.HasPrefix(a, "--resume=") {
			isResume = true
			resumeConvID = strings.TrimPrefix(a, "--resume=")
			break
		}
		if (a == "--conversation" || a == "--resume") && i+1 < len(originalUserArgs) {
			isResume = true
			resumeConvID = originalUserArgs[i+1]
			break
		}
	}
	if !isResume && IsInteractiveSession(originalUserArgs) {
		_ = profile.ResetSessionContext(profileDir)
	} else if isResume && resumeConvID != "" {
		if existingState, ok := profile.GetSessionContextState(profileDir); ok && existingState != nil {
			if existingState.ConversationID != "" && existingState.ConversationID != resumeConvID {
				_ = profile.ResetSessionContext(profileDir)
			}
		}
	}

	_ = profile.SyncStatusLineSettings(profileDir)

	if profile.IsInHerdrEnvironment() {
		_ = profile.SyncHerdrIntegration(profileDir)
		profile.SetTerminalTitle(targetProfile)
		fastQuota, hasFast := profile.GetProfileFullQuotaDetailsFast(targetProfile, activeModel)
		if hasFast && fastQuota != nil {
			_ = profile.ReportHerdrMetadataWithModel(ctx, targetProfile, activeModel, fastQuota)
		} else {
			go func() {
				bgCtx, bgCancel := context.WithTimeout(context.Background(), 8*time.Second)
				defer bgCancel()
				_ = profile.ReportHerdrMetadataWithModel(bgCtx, targetProfile, activeModel)
			}()
		}

		stopWatcher := profile.StartHerdrQuotaWatcher(ctx, targetProfile, activeModel)
		defer func() {
			stopWatcher()
			_ = profile.ClearHerdrMetadata(context.Background())
		}()
	}

	runErr := profile.RunCmdWithSignalsInDir(ctx, profileDir, opts.WorkingDir, agyArgs...)

	profile.SyncKeychainTokenToDisk(profileDir, expectedRefreshToken)
	idAfter, _, _ := profile.GetLatestConversationFileInfo(targetProfile)
	isInteractive := IsInteractiveSession(originalUserArgs)

	if idAfter != "" && isInteractive {
		_ = profile.SaveLastConversation(idAfter)

		var preservedFlags []string
		for i := 0; i < len(originalUserArgs); i++ {
			arg := originalUserArgs[i]
			if arg == "--conversation" {
				i++
				continue
			}
			if strings.HasPrefix(arg, "--conversation=") {
				continue
			}
			if arg == "-c" || arg == "--continue" {
				continue
			}
			preservedFlags = append(preservedFlags, arg)
		}

		var extraFlags string
		if len(preservedFlags) > 0 {
			_ = profile.SaveSessionFlags(idAfter, preservedFlags)
			var quotedFlags []string
			for _, f := range preservedFlags {
				quotedFlags = append(quotedFlags, sshproxy.ShellQuote(f))
			}
			extraFlags = " " + strings.Join(quotedFlags, " ")
		}

		isTTY := false
		if fi, statErr := os.Stdout.Stat(); statErr == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
			isTTY = true
		}

		sshServer := os.Getenv("AGYS_SSH_SERVER")
		sshPath := os.Getenv("AGYS_SSH_PATH")

		var resumeCmdStr string
		if sshServer != "" {
			pathArg := ""
			if sshPath != "" {
				pathArg = " " + sshproxy.ShellQuote(sshPath)
			}
			resumeCmdStr = fmt.Sprintf("agys ssh %s%s %s -- --conversation=%s%s", sshServer, pathArg, targetProfile, idAfter, extraFlags)
		} else {
			resumeCmdStr = fmt.Sprintf("agys run %s -- --conversation=%s%s", targetProfile, idAfter, extraFlags)
		}

		if isTTY {
			fmt.Print("\x1b[1A\x1b[2K\r\x1b[1A\x1b[2K\r")
			fmt.Printf("Resume with 'agys run -c' (or command below):\n%s\n", resumeCmdStr)
		} else {
			fmt.Fprintln(opts.Stdout, resumeCmdStr)
		}
	}

	return runErr
}

// ResolveDefaultProfile resolves the default profile name from the current environment.
func ResolveDefaultProfile() (string, error) {
	current, err := profile.GetCurrent()
	if err != nil {
		return "", err
	}
	if current == "" {
		return "", nil
	}
	if profile.IsAuto(current) {
		return profile.AutoProfileKeyword, nil
	}
	currentExists, _, err := profile.Exists(current)
	if err != nil {
		return "", err
	}
	if currentExists {
		return current, nil
	}
	return "", nil
}
