package doctor

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// mockChecker implements Checker for testing.
type mockChecker struct {
	report Report
	err    error
}

func (m *mockChecker) RunDiagnostics(ctx context.Context) (Report, error) {
	if m.err != nil {
		return Report{}, m.err
	}
	return m.report, nil
}

func TestService_ExecuteSuccess(t *testing.T) {
	mockRep := Report{
		Sections: []Section{
			{
				Title: "Test Section",
				Items: []Item{
					{Status: StatusOK, Message: "All good"},
					{Status: StatusWarning, Message: "Look out"},
					{Status: StatusIssue, Message: "Something broken"},
				},
			},
		},
		Issues:   1,
		Warnings: 1,
	}

	checker := &mockChecker{report: mockRep}
	reporter := NewConsoleReporter()
	svc := NewService(checker, reporter)

	var buf bytes.Buffer
	err := svc.Execute(context.Background(), &buf)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Test Section") {
		t.Errorf("output missing section title: %s", out)
	}
	if !strings.Contains(out, "All good") {
		t.Errorf("output missing OK item: %s", out)
	}
	if !strings.Contains(out, "1 issue(s) and 1 warning(s)") {
		t.Errorf("output missing summary: %s", out)
	}
}

func TestService_ExecuteCheckerError(t *testing.T) {
	expectedErr := errors.New("check failure")
	checker := &mockChecker{err: expectedErr}
	svc := NewService(checker, nil)

	var buf bytes.Buffer
	err := svc.Execute(context.Background(), &buf)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

func TestFormatRemainingTime(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{-5 * time.Minute, "0m"},
		{0, "0m"},
		{45 * time.Minute, "45m"},
		{60 * time.Minute, "1h00m"},
		{125 * time.Minute, "2h05m"},
	}

	for _, tt := range tests {
		got := formatRemainingTime(tt.duration)
		if got != tt.expected {
			t.Errorf("formatRemainingTime(%v) = %q, expected %q", tt.duration, got, tt.expected)
		}
	}
}
