package cmd

import (
	"github.com/quaywin/agys/internal/selector"
	"github.com/quaywin/agys/pkg/profile"
)

func selectSessionInteractive(sessions []profile.ConversationSession) (*profile.ConversationSession, error) {
	return selector.NewTerminalSelector().SelectSession(sessions)
}

func getTerminalWidth() int {
	return selector.GetTerminalWidth()
}

func truncatePrompt(prompt string, maxRunes int) string {
	return selector.TruncatePrompt(prompt, maxRunes)
}

func formatSessionLine(num int, s profile.ConversationSession, selected bool, termWidth int) string {
	return selector.FormatSessionLine(num, s, selected, termWidth)
}

func filterSessions(sessions []profile.ConversationSession, query string) []profile.ConversationSession {
	return selector.FilterSessions(sessions, query)
}

type sessionGroup = selector.SessionGroup
type groupedSessionItem = selector.GroupedSessionItem

func groupSessions(sessions []profile.ConversationSession) []sessionGroup {
	return selector.GroupSessions(sessions)
}
