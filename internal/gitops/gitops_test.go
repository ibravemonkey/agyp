package gitops

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

type mockGitClient struct {
	isRepo      bool
	stagedFiles []string
	diff        string
	stat        string
	committed   string
	pushed      bool
}

func (m *mockGitClient) IsRepository(dir string) bool {
	return m.isRepo
}

func (m *mockGitClient) StageTrackedFiles(dir string) error {
	return nil
}

func (m *mockGitClient) StageAllFiles(dir string) error {
	return nil
}

func (m *mockGitClient) GetStagedFiles(dir string) ([]string, error) {
	return m.stagedFiles, nil
}

func (m *mockGitClient) GetStagedDiff(dir string) (string, string, error) {
	return m.diff, m.stat, nil
}

func (m *mockGitClient) Commit(dir string, message string) error {
	m.committed = message
	return nil
}

func (m *mockGitClient) Push(dir string) error {
	m.pushed = true
	return nil
}

type mockReviewer struct {
	summary string
	msg     string
	err     error
}

func (m *mockReviewer) ReviewAndGenerate(ctx context.Context, profileDir string, stagedFiles []string, diff, stat, userMsg string, noCheck bool, model, effort, prompt string) (string, string, error) {
	if m.err != nil {
		return "", "", m.err
	}
	return m.summary, m.msg, nil
}

func TestCommitService_NotGitRepo(t *testing.T) {
	git := &mockGitClient{isRepo: false}
	svc := NewService(git, nil, nil, nil, nil)

	err := svc.Execute(context.Background(), CommitOptions{})
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("expected 'not a git repository' error, got: %v", err)
	}
}

func TestCommitService_NoStagedFiles(t *testing.T) {
	git := &mockGitClient{isRepo: true, stagedFiles: nil}
	var out bytes.Buffer
	svc := NewService(git, nil, nil, &out, nil)

	err := svc.Execute(context.Background(), CommitOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "No staged changes found") {
		t.Errorf("expected 'No staged changes found' message, got: %s", out.String())
	}
}

func TestCommitService_DryRun(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("AGYS_DIR", filepath.Join(tempHome, ".agys"))

	_, err := profile.Create("test-prof")
	if err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	git := &mockGitClient{
		isRepo:      true,
		stagedFiles: []string{"main.go"},
		diff:        "+package main",
	}
	reviewer := &mockReviewer{
		summary: "Looks clean",
		msg:     "feat: initial setup",
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	svc := NewService(git, reviewer, nil, &out, &errOut)

	opts := CommitOptions{
		ProfileName: "test-prof",
		DryRun:      true,
		NoCheck:     true,
		Message:     "feat: manual message",
	}

	err = svc.Execute(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if git.committed != "" {
		t.Errorf("expected no commit during dry run, but committed: %q", git.committed)
	}
	if !strings.Contains(out.String(), "Dry-run complete") {
		t.Errorf("expected dry-run notification in output: %s", out.String())
	}
}
