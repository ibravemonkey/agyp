package selector

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

func TestTruncatePrompt(t *testing.T) {
	cases := []struct {
		input    string
		maxRunes int
		expected string
	}{
		{"hello world", 20, "hello world"},
		{"hello world", 11, "hello world"},
		{"hello world", 8, "hello..."},
		{"hello\nworld\tfoo", 20, "hello world foo"},
		{"abc", 2, "ab"},
		{"abc", 3, "abc"},
	}

	for _, c := range cases {
		got := TruncatePrompt(c.input, c.maxRunes)
		if got != c.expected {
			t.Errorf("TruncatePrompt(%q, %d) = %q; want %q", c.input, c.maxRunes, got, c.expected)
		}
	}
}

func TestFormatSessionLine(t *testing.T) {
	sess := profile.ConversationSession{
		Profile:     "davidnguyen",
		ConvID:      "conv-12345",
		ModTime:     time.Now().Add(-2 * time.Minute),
		ProjectPath: "/home/user/project",
		UserPrompt:  "fix login bug",
	}

	// Selected line
	selectedLine := FormatSessionLine(1, sess, true, 100)
	if !strings.Contains(selectedLine, "❯") {
		t.Errorf("expected selected line to contain arrow '❯', got: %s", selectedLine)
	}
	if !strings.Contains(selectedLine, "[1]") || !strings.Contains(selectedLine, "[davidnguyen]") {
		t.Errorf("expected selected line to contain [1] and [davidnguyen], got: %s", selectedLine)
	}
	if !strings.Contains(selectedLine, "fix login bug") {
		t.Errorf("expected selected line to contain prompt summary, got: %s", selectedLine)
	}

	// Unselected line
	unselectedLine := FormatSessionLine(2, sess, false, 100)
	if strings.Contains(unselectedLine, "❯") {
		t.Errorf("expected unselected line NOT to contain arrow '❯', got: %s", unselectedLine)
	}
	if !strings.Contains(unselectedLine, "[2]") || !strings.Contains(unselectedLine, "[davidnguyen]") {
		t.Errorf("expected unselected line to contain [2] and [davidnguyen], got: %s", unselectedLine)
	}
}

func TestFilterSessions(t *testing.T) {
	sessions := []profile.ConversationSession{
		{Profile: "work", ProjectName: "agys", UserPrompt: "fix login bug", ConvID: "conv-111"},
		{Profile: "personal", ProjectName: "website", UserPrompt: "add dark mode styling", ConvID: "conv-222"},
		{Profile: "work", ProjectName: "backend", UserPrompt: "implement payment api", ConvID: "conv-333"},
	}

	// Empty query returns all
	if got := FilterSessions(sessions, ""); len(got) != 3 {
		t.Errorf("expected 3 sessions for empty query, got %d", len(got))
	}

	// Filter by prompt keyword
	matchedPrompt := FilterSessions(sessions, "dark mode")
	if len(matchedPrompt) != 1 || matchedPrompt[0].ConvID != "conv-222" {
		t.Errorf("expected conv-222 for 'dark mode', got %v", matchedPrompt)
	}

	// Filter by profile
	matchedProfile := FilterSessions(sessions, "work")
	if len(matchedProfile) != 2 {
		t.Errorf("expected 2 sessions for profile 'work', got %d", len(matchedProfile))
	}

	// Filter by project
	matchedProject := FilterSessions(sessions, "agys")
	if len(matchedProject) != 1 || matchedProject[0].ConvID != "conv-111" {
		t.Errorf("expected conv-111 for project 'agys', got %v", matchedProject)
	}

	// Filter with no matches
	if got := FilterSessions(sessions, "nonexistent query"); len(got) != 0 {
		t.Errorf("expected 0 matches, got %d", len(got))
	}
}

func TestGroupSessions(t *testing.T) {
	sessions := []profile.ConversationSession{
		{ProjectName: "agys", ProjectPath: "/path/agys", UserPrompt: "task 1"},
		{ProjectName: "website", ProjectPath: "/path/web", UserPrompt: "task 2"},
		{ProjectName: "agys", ProjectPath: "/path/agys", UserPrompt: "task 3"},
	}

	groups := GroupSessions(sessions)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	if groups[0].ProjectName != "agys" || len(groups[0].Sessions) != 2 {
		t.Errorf("expected group 'agys' with 2 sessions, got %s with %d", groups[0].ProjectName, len(groups[0].Sessions))
	}
	if groups[1].ProjectName != "website" || len(groups[1].Sessions) != 1 {
		t.Errorf("expected group 'website' with 1 session, got %s with %d", groups[1].ProjectName, len(groups[1].Sessions))
	}
	if groups[0].Sessions[0].Index != 0 || groups[0].Sessions[1].Index != 2 {
		t.Errorf("expected indices 0 and 2 for agys sessions, got %d and %d", groups[0].Sessions[0].Index, groups[0].Sessions[1].Index)
	}
}

func TestPromptSelector(t *testing.T) {
	sessions := []profile.ConversationSession{
		{Profile: "dev", UserPrompt: "first task", ConvID: "c1"},
		{Profile: "prod", UserPrompt: "second task", ConvID: "c2"},
	}

	// User picks option 2
	input := strings.NewReader("2\n")
	var output bytes.Buffer
	sel := NewPromptSelector(input, &output)

	chosen, err := sel.SelectSession(sessions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chosen == nil || chosen.ConvID != "c2" {
		t.Fatalf("expected choice c2, got %v", chosen)
	}

	// User presses Enter without choice -> nil (exit)
	inputEmpty := strings.NewReader("\n")
	output.Reset()
	selEmpty := NewPromptSelector(inputEmpty, &output)
	chosenEmpty, errEmpty := selEmpty.SelectSession(sessions)
	if errEmpty != nil {
		t.Fatalf("unexpected error: %v", errEmpty)
	}
	if chosenEmpty != nil {
		t.Fatalf("expected nil on empty choice, got %v", chosenEmpty)
	}
}
