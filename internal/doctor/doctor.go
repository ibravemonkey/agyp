package doctor

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ibravemonkey/agyp/pkg/profile"
	"github.com/ibravemonkey/agyp/pkg/updater"
	"github.com/ibravemonkey/agyp/pkg/version"
)

// Status represents the health check status level.
type Status int

const (
	StatusOK Status = iota
	StatusWarning
	StatusIssue
	StatusInfo
)

// Item represents a sub-item in a diagnostic section.
type Item struct {
	Status  Status
	Message string
}

// Section represents a group of related health checks.
type Section struct {
	Title string
	Items []Item
}

// Report holds the aggregated findings of a health check run.
type Report struct {
	Sections []Section
	Issues   int
	Warnings int
}

// Checker defines the contract for running diagnostic checks.
type Checker interface {
	RunDiagnostics(ctx context.Context) (Report, error)
}

// Reporter defines the contract for rendering a diagnostic report.
type Reporter interface {
	Render(w io.Writer, report Report) error
}

// Service coordinates running diagnostics and rendering results.
type Service interface {
	Execute(ctx context.Context, w io.Writer) error
}

type service struct {
	checker  Checker
	reporter Reporter
}

// NewService creates a Service with the provided checker and reporter.
// If nil, default implementations are used.
func NewService(checker Checker, reporter Reporter) Service {
	if checker == nil {
		checker = NewDefaultChecker()
	}
	if reporter == nil {
		reporter = NewConsoleReporter()
	}
	return &service{
		checker:  checker,
		reporter: reporter,
	}
}

func (s *service) Execute(ctx context.Context, w io.Writer) error {
	report, err := s.checker.RunDiagnostics(ctx)
	if err != nil {
		return err
	}
	return s.reporter.Render(w, report)
}

// DefaultChecker implements Checker using live system checks.
type DefaultChecker struct{}

// NewDefaultChecker creates a new live system checker.
func NewDefaultChecker() *DefaultChecker {
	return &DefaultChecker{}
}

// RunDiagnostics executes all health checks and produces a Report.
func (c *DefaultChecker) RunDiagnostics(ctx context.Context) (Report, error) {
	var report Report

	// 1. agys Binary Info
	report.Sections = append(report.Sections, c.checkAgys())

	// 2. agy Binary Check
	secAgy, issuesAgy, warnAgy := c.checkAgy(ctx)
	report.Sections = append(report.Sections, secAgy)
	report.Issues += issuesAgy
	report.Warnings += warnAgy

	// 3. Multi-Account Profiles Check
	secProf, issuesProf, warnProf := c.checkProfiles(ctx)
	report.Sections = append(report.Sections, secProf)
	report.Issues += issuesProf
	report.Warnings += warnProf

	// 4. Customizations & Model Discovery Check
	secModel, warnModel := c.checkModels()
	report.Sections = append(report.Sections, secModel)
	report.Warnings += warnModel

	// 5. Herdr Multi-Agent Environment Check
	secHerdr, warnHerdr := c.checkHerdr()
	report.Sections = append(report.Sections, secHerdr)
	report.Warnings += warnHerdr

	return report, nil
}

func (c *DefaultChecker) checkAgys() Section {
	sec := Section{Title: "agys Switcher"}
	sec.Items = append(sec.Items, Item{
		Status:  StatusOK,
		Message: fmt.Sprintf("Version: v%s (%s/%s, %s)", updater.CleanVersion(version.Version), runtime.GOOS, runtime.GOARCH, runtime.Version()),
	})
	return sec
}

func (c *DefaultChecker) checkAgy(ctx context.Context) (Section, int, int) {
	sec := Section{Title: "Antigravity CLI (agy)"}
	issues := 0
	warnings := 0

	agyPath, err := exec.LookPath("agy")
	if err != nil {
		home, _ := profile.GetRealUserHome()
		candidates := []string{
			filepath.Join(home, ".local", "bin", "agy"),
			"/opt/homebrew/bin/agy",
			"/usr/local/bin/agy",
		}
		for _, candidate := range candidates {
			if _, statErr := os.Stat(candidate); statErr == nil {
				agyPath = candidate
				break
			}
		}
	}

	if agyPath == "" {
		sec.Items = append(sec.Items, Item{
			Status:  StatusIssue,
			Message: "agy binary: Not found in PATH or standard locations (~/.local/bin, /opt/homebrew/bin)",
		})
		issues++
	} else {
		sec.Items = append(sec.Items, Item{
			Status:  StatusOK,
			Message: fmt.Sprintf("Binary found: %s", agyPath),
		})

		verCmd := exec.CommandContext(ctx, agyPath, "--version")
		verOut, verErr := verCmd.Output()
		if verErr == nil {
			installedAgyVer := strings.TrimSpace(string(verOut))
			sec.Items = append(sec.Items, Item{
				Status:  StatusOK,
				Message: fmt.Sprintf("Installed version: %s", installedAgyVer),
			})

			rel, relErr := updater.FetchLatestRelease("google-antigravity", "antigravity-cli")
			if relErr == nil && rel != nil {
				latestVer := updater.CleanVersion(rel.TagName)
				cleanInstalled := updater.CleanVersion(installedAgyVer)
				if updater.IsNewer(cleanInstalled, latestVer) {
					sec.Items = append(sec.Items, Item{
						Status:  StatusWarning,
						Message: fmt.Sprintf("New agy release available: v%s (run 'agy update' to upgrade)", latestVer),
					})
					warnings++
				} else {
					sec.Items = append(sec.Items, Item{
						Status:  StatusOK,
						Message: fmt.Sprintf("Status: Up to date with latest release (v%s)", latestVer),
					})
				}
			}
		} else {
			sec.Items = append(sec.Items, Item{
				Status:  StatusWarning,
				Message: fmt.Sprintf("Failed to execute 'agy --version': %v", verErr),
			})
			warnings++
		}
	}

	return sec, issues, warnings
}

func (c *DefaultChecker) checkProfiles(ctx context.Context) (Section, int, int) {
	sec := Section{}
	issues := 0
	warnings := 0

	profiles, err := profile.List()
	if err != nil {
		sec.Title = "Sandboxed Profiles"
		sec.Items = append(sec.Items, Item{
			Status:  StatusIssue,
			Message: fmt.Sprintf("Failed to list profiles: %v", err),
		})
		issues++
		return sec, issues, warnings
	}

	currentProf, _ := profile.GetCurrent()
	sec.Title = fmt.Sprintf("Sandboxed Profiles (%d configured)", len(profiles))

	if len(profiles) == 0 {
		sec.Items = append(sec.Items, Item{
			Status:  StatusWarning,
			Message: "No profiles found. Create one using 'agys add <name>'",
		})
		warnings++
		return sec, issues, warnings
	}

	for _, p := range profiles {
		isDefault := (p == currentProf)
		suffix := ""
		if isDefault {
			suffix = " \033[1;36m(default)\033[0m"
		}

		pDir, _ := profile.GetProfileDir(p)
		token, tokenErr := profile.ReadToken(p)
		email, _ := profile.GetCachedEmail(p)

		if tokenErr != nil || token == nil {
			sec.Items = append(sec.Items, Item{
				Status:  StatusWarning,
				Message: fmt.Sprintf("Profile '%s'%s: Not authenticated or token missing\n    - Sandbox: %s\n    - Tip: Run 'agys run %s -- auth login' to authenticate", p, suffix, pDir, p),
			})
			warnings++
			continue
		}

		emailDisplay := email
		if emailDisplay == "" {
			emailDisplay = "(Email not cached yet)"
		}

		detailsMsg := fmt.Sprintf("Profile '%s'%s\n    - Google Account: %s", p, suffix, emailDisplay)

		if !token.Token.Expiry.IsZero() {
			rem := time.Until(token.Token.Expiry)
			if rem > 0 {
				detailsMsg += fmt.Sprintf("\n    - OAuth Token: Valid (expires in %s, auto-refresh armed)", formatRemainingTime(rem))
			} else {
				detailsMsg += "\n    - OAuth Token: Expired (auto-refresh armed via refresh_token)"
			}
		} else {
			detailsMsg += "\n    - OAuth Token: Present"
		}

		if summary, ok := profile.GetCachedQuota(p, 24*time.Hour); ok && summary != nil {
			details, _ := profile.GetProfileFullQuotaDetailsForModel(ctx, p, "")
			if details != nil {
				pct5h := "N/A"
				if details.Fraction5H >= 0 {
					pct5h = fmt.Sprintf("%.1f%% (%s)", details.Fraction5H*100, details.ResetStr5H)
				}
				pctWk := "N/A"
				if details.FractionWeekly >= 0 {
					pctWk = fmt.Sprintf("%.1f%% (%s)", details.FractionWeekly*100, details.ResetStrWeekly)
				}
				detailsMsg += fmt.Sprintf("\n    - Quota HUD: 5H: %s • Weekly: %s", pct5h, pctWk)
			}
		}

		if runtime.GOOS == "darwin" {
			profileKeychainsDir := filepath.Join(pDir, "Library", "Keychains")
			if info, lErr := os.Lstat(profileKeychainsDir); lErr == nil && (info.Mode()&os.ModeSymlink != 0) {
				detailsMsg += "\n    - macOS Keychain: Linked and isolated"
			}
		}

		sec.Items = append(sec.Items, Item{
			Status:  StatusOK,
			Message: detailsMsg,
		})
	}

	return sec, issues, warnings
}

func (c *DefaultChecker) checkModels() (Section, int) {
	sec := Section{Title: "Model Catalog & Discovery"}
	warnings := 0

	dm := profile.GetOrRefreshModels()
	if dm != nil && (dm.LatestFlash != "" || dm.LatestPro != "") {
		sec.Items = append(sec.Items,
			Item{Status: StatusOK, Message: fmt.Sprintf("Discovered Flash: %s", dm.LatestFlash)},
			Item{Status: StatusOK, Message: fmt.Sprintf("Discovered Pro:   %s", dm.LatestPro)},
			Item{Status: StatusOK, Message: fmt.Sprintf("Cache timestamp:  %s", dm.FetchedAt.Format(time.RFC3339))},
		)
	} else {
		sec.Items = append(sec.Items, Item{
			Status:  StatusWarning,
			Message: "Model cache empty. Run 'agys models -r' to discover available models from agy.",
		})
		warnings++
	}

	return sec, warnings
}

func (c *DefaultChecker) checkHerdr() (Section, int) {
	sec := Section{Title: "Herdr Integration"}
	warnings := 0

	if profile.IsInHerdrEnvironment() {
		paneID := os.Getenv("HERDR_PANE_ID")
		sockPath := os.Getenv("HERDR_SOCKET_PATH")
		details := fmt.Sprintf("Running inside Herdr workspace\n    - Active Pane ID: %s", paneID)

		if sockPath != "" {
			conn, netErr := net.DialTimeout("unix", sockPath, 500*time.Millisecond)
			if netErr == nil {
				_ = conn.Close()
				details += fmt.Sprintf("\n    - Socket RPC: Reachable (%s)", sockPath)
			} else {
				details += fmt.Sprintf("\n    - Socket RPC: Unreachable (%v)", netErr)
				warnings++
			}
		}
		sec.Items = append(sec.Items, Item{
			Status:  StatusOK,
			Message: details,
		})
	} else {
		sec.Items = append(sec.Items, Item{
			Status:  StatusInfo,
			Message: "Environment: Standalone Terminal (not inside Herdr pane)",
		})
	}

	return sec, warnings
}

// ConsoleReporter renders reports in standard terminal ANSI styling.
type ConsoleReporter struct{}

// NewConsoleReporter creates a new ConsoleReporter.
func NewConsoleReporter() *ConsoleReporter {
	return &ConsoleReporter{}
}

// Render prints the Report to w.
func (r *ConsoleReporter) Render(w io.Writer, report Report) error {
	fmt.Fprintf(w, "\n\033[1;36m[agys]\033[0m \033[1;37mRunning Antigravity & Herdr Environment Health Check...\033[0m\n\n")

	for i, sec := range report.Sections {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "\033[1;34m● %s\033[0m\n", sec.Title)
		for _, item := range sec.Items {
			prefix := "  "
			switch item.Status {
			case StatusOK:
				prefix += "\033[1;32m✓\033[0m "
			case StatusWarning:
				prefix += "\033[1;33m!\033[0m "
			case StatusIssue:
				prefix += "\033[1;31m✗\033[0m "
			case StatusInfo:
				prefix += "- "
			}
			fmt.Fprintf(w, "%s%s\n", prefix, item.Message)
		}
	}

	fmt.Fprintln(w)
	if report.Issues == 0 && report.Warnings == 0 {
		fmt.Fprintf(w, "\033[1;32m✓ All system checks passed with zero issues!\033[0m\n\n")
	} else if report.Issues == 0 {
		fmt.Fprintf(w, "\033[1;33m! Health check completed: 0 issues, %d warning(s).\033[0m\n\n", report.Warnings)
	} else {
		fmt.Fprintf(w, "\033[1;31m✗ Health check found %d issue(s) and %d warning(s).\033[0m\n\n", report.Issues, report.Warnings)
	}

	return nil
}

func formatRemainingTime(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
