package selector

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/quaywin/agys/pkg/profile"
	"golang.org/x/term"
)

// Selector abstracts interactive and non-interactive conversation session picking.
type Selector interface {
	SelectSession(sessions []profile.ConversationSession) (*profile.ConversationSession, error)
}

// GroupedSessionItem associates a session with its index in the flattened list.
type GroupedSessionItem struct {
	Index   int
	Session profile.ConversationSession
}

// SessionGroup groups conversation sessions belonging to a specific workspace directory/project.
type SessionGroup struct {
	ProjectName string
	ProjectPath string
	Sessions    []GroupedSessionItem
}

// GetTerminalWidth returns the current terminal column width, or default 100.
func GetTerminalWidth() int {
	fd := int(os.Stdout.Fd())
	if term.IsTerminal(fd) {
		w, _, err := term.GetSize(fd)
		if err == nil && w > 30 {
			return w
		}
	}
	return 100
}

// TruncatePrompt strips newlines/excess spaces and truncates to maxRunes cleanly.
func TruncatePrompt(prompt string, maxRunes int) string {
	prompt = strings.TrimSpace(prompt)
	prompt = strings.ReplaceAll(prompt, "\r\n", " ")
	prompt = strings.ReplaceAll(prompt, "\n", " ")
	prompt = strings.ReplaceAll(prompt, "\r", " ")
	prompt = strings.ReplaceAll(prompt, "\t", " ")
	for strings.Contains(prompt, "  ") {
		prompt = strings.ReplaceAll(prompt, "  ", " ")
	}

	runes := []rune(prompt)
	if len(runes) <= maxRunes {
		return prompt
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

// FormatSessionLine formats a single session in compact 1-line style with ANSI colors.
func FormatSessionLine(num int, s profile.ConversationSession, selected bool, termWidth int) string {
	relTime := profile.FormatRelativeTime(s.ModTime)

	numStrLen := utf8.RuneCountInString(fmt.Sprintf("[%d] ", num))
	profStrLen := utf8.RuneCountInString(fmt.Sprintf("[%s] ", s.Profile))
	timeStrLen := utf8.RuneCountInString(fmt.Sprintf("(%s)", relTime))
	metaWidth := 4 + numStrLen + profStrLen + timeStrLen + 2

	maxPromptWidth := termWidth - metaWidth
	if maxPromptWidth < 15 {
		maxPromptWidth = 15
	}

	promptClean := TruncatePrompt(s.UserPrompt, maxPromptWidth)

	if selected {
		return fmt.Sprintf("\033[1;36m  ❯\033[0m \033[1;36m[%d]\033[0m \033[1;37m%s\033[0m \033[1;35m[%s]\033[0m \033[90m(%s)\033[0m",
			num, promptClean, s.Profile, relTime)
	}
	return fmt.Sprintf("    \033[90m[%d]\033[0m %s \033[35m[%s]\033[0m \033[90m(%s)\033[0m",
		num, promptClean, s.Profile, relTime)
}

// GroupSessions organizes sessions by their project for grouped display view.
func GroupSessions(sessions []profile.ConversationSession) []SessionGroup {
	var groups []SessionGroup
	groupMap := make(map[string]int)

	for i, s := range sessions {
		pName := s.ProjectName
		if pName == "" {
			pName = "(global)"
		}
		pPath := s.ProjectPath
		key := pName + "::" + pPath

		gIdx, exists := groupMap[key]
		if !exists {
			gIdx = len(groups)
			groupMap[key] = gIdx
			groups = append(groups, SessionGroup{
				ProjectName: pName,
				ProjectPath: pPath,
				Sessions:    nil,
			})
		}
		groups[gIdx].Sessions = append(groups[gIdx].Sessions, GroupedSessionItem{
			Index:   i,
			Session: s,
		})
	}
	return groups
}

// FilterSessions returns sessions matching query in prompt, profile, project, or conversation ID.
func FilterSessions(sessions []profile.ConversationSession, query string) []profile.ConversationSession {
	trimmed := strings.TrimSpace(strings.ToLower(query))
	if trimmed == "" {
		return sessions
	}

	var matched []profile.ConversationSession
	for _, s := range sessions {
		promptLower := strings.ToLower(s.UserPrompt)
		profileLower := strings.ToLower(s.Profile)
		projLower := strings.ToLower(s.ProjectName)
		convLower := strings.ToLower(s.ConvID)

		if strings.Contains(promptLower, trimmed) ||
			strings.Contains(profileLower, trimmed) ||
			strings.Contains(projLower, trimmed) ||
			strings.Contains(convLower, trimmed) {
			matched = append(matched, s)
		}
	}
	return matched
}

// TerminalSelector implements Selector via raw interactive terminal navigation.
type TerminalSelector struct{}

// NewTerminalSelector creates a new TerminalSelector.
func NewTerminalSelector() *TerminalSelector {
	return &TerminalSelector{}
}

// SelectSession launches an interactive arrow-key selector in the terminal.
func (t *TerminalSelector) SelectSession(sessions []profile.ConversationSession) (*profile.ConversationSession, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return nil, errors.New("stdin is not a terminal")
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}

	restored := false
	restore := func() {
		if !restored {
			_ = term.Restore(fd, oldState)
			fmt.Print("\033[?25h") // Ensure cursor is visible
			restored = true
		}
	}
	defer restore()

	// Hide cursor during interactive menu
	fmt.Print("\033[?25l")

	cursor := 0
	searchQuery := ""
	searchMode := false
	groupedMode := false
	prevLines := 0

	render := func() {
		termWidth := GetTerminalWidth()
		displayed := FilterSessions(sessions, searchQuery)
		numDisplayed := len(displayed)

		if cursor >= numDisplayed && numDisplayed > 0 {
			cursor = numDisplayed - 1
		} else if cursor < 0 {
			cursor = 0
		}

		if prevLines > 0 {
			fmt.Printf("\033[%dA", prevLines)
			for range prevLines {
				fmt.Print("\r\033[2K\r\n")
			}
			fmt.Printf("\033[%dA", prevLines)
		}

		linesDrawn := 0

		if searchMode || searchQuery != "" {
			searchHint := fmt.Sprintf("\033[1;33m  Search: \033[0m\033[1;37m%s\033[0m\033[1;36m_\033[0m", searchQuery)
			fmt.Printf("\r\033[2K%s\r\n", searchHint)
			linesDrawn++
		}

		if numDisplayed == 0 {
			fmt.Printf("\r\033[2K    \033[90m(No conversations matching %q)\033[0m\r\n", searchQuery)
			linesDrawn++
		} else if groupedMode {
			groups := GroupSessions(displayed)
			for _, g := range groups {
				header := fmt.Sprintf("  \033[1;34m📁 %s\033[0m \033[90m(%s)\033[0m", g.ProjectName, g.ProjectPath)
				fmt.Printf("\r\033[2K%s\r\n", header)
				linesDrawn++

				for _, item := range g.Sessions {
					selected := (item.Index == cursor)
					line := FormatSessionLine(item.Index+1, item.Session, selected, termWidth)
					fmt.Printf("\r\033[2K%s\r\n", line)
					linesDrawn++
				}
			}
		} else {
			for i, s := range displayed {
				selected := (i == cursor)
				line := FormatSessionLine(i+1, s, selected, termWidth)
				fmt.Printf("\r\033[2K%s\r\n", line)
				linesDrawn++
			}
		}

		// Blank spacer line
		fmt.Print("\r\033[2K\r\n")
		linesDrawn++

		// Footer navigation hint
		var footer string
		if searchMode {
			footer = "\033[90m  Filter active • Esc: clear/exit • Enter: select • ↑/↓: navigate\033[0m"
		} else if groupedMode {
			footer = fmt.Sprintf("\033[90m  Navigate: ↑/↓, j/k • Select: Enter • Filter: / • Grouped (Ctrl+F to flatten) • Quit: Esc, q\033[0m")
		} else if numDisplayed <= 9 {
			footer = fmt.Sprintf("\033[90m  Navigate: ↑/↓, j/k • Select: Enter • Filter: / • Group: Ctrl+F • Jump: 1-%d • Quit: Esc, q\033[0m", numDisplayed)
		} else {
			footer = "\033[90m  Navigate: ↑/↓, j/k • Select: Enter • Filter: / • Group: Ctrl+F • Quit: Esc, q\033[0m"
		}
		fmt.Printf("\r\033[2K%s\r\n", footer)
		linesDrawn++

		prevLines = linesDrawn
	}

	render()

	// Cleanly handle Ctrl+C signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	go func() {
		<-sigChan
		restore()
		os.Exit(0)
	}()

	clearMenu := func() {
		if prevLines > 0 {
			fmt.Printf("\033[%dA", prevLines)
			for range prevLines {
				fmt.Print("\r\033[2K\r\n")
			}
			fmt.Printf("\033[%dA", prevLines)
		}
	}

	buf := make([]byte, 16)
	for {
		n, readErr := os.Stdin.Read(buf)
		if readErr != nil {
			return nil, nil
		}
		if n == 0 {
			continue
		}

		displayed := FilterSessions(sessions, searchQuery)
		numDisplayed := len(displayed)

		// Enter -> confirm selection
		if buf[0] == '\r' || buf[0] == '\n' {
			if numDisplayed > 0 {
				clearMenu()
				restore()
				return &displayed[cursor], nil
			}
			if searchMode {
				searchMode = false
				render()
				continue
			}
		}

		// Space selection in non-search mode
		if buf[0] == ' ' && n == 1 && !searchMode {
			if numDisplayed > 0 {
				clearMenu()
				restore()
				return &displayed[cursor], nil
			}
		}

		// Ctrl+C (3), Ctrl+D (4)
		if (buf[0] == 3 || buf[0] == 4) && n == 1 {
			clearMenu()
			restore()
			return nil, nil
		}

		// Ctrl+F (6) -> toggle grouped workspace view
		if buf[0] == 6 && n == 1 {
			groupedMode = !groupedMode
			render()
			continue
		}

		// Search mode character processing
		if searchMode {
			// Escape in search mode: exit search mode, clear search query
			if buf[0] == 27 && n == 1 {
				searchMode = false
				searchQuery = ""
				cursor = 0
				render()
				continue
			}

			// Backspace in search mode (8 or 127)
			if (buf[0] == 8 || buf[0] == 127) && n == 1 {
				runes := []rune(searchQuery)
				if len(runes) > 0 {
					searchQuery = string(runes[:len(runes)-1])
					cursor = 0
					render()
				}
				continue
			}

			// Printable ASCII characters in search mode (32..126)
			if n == 1 && buf[0] >= 32 && buf[0] <= 126 {
				searchQuery += string(buf[:1])
				cursor = 0
				render()
				continue
			}
		}

		// Non-search mode: Escape alone -> Quit
		if buf[0] == 27 && n == 1 && !searchMode {
			if searchQuery != "" {
				searchQuery = ""
				cursor = 0
				render()
				continue
			}
			clearMenu()
			restore()
			return nil, nil
		}

		// Non-search mode: Quit ('q', 'Q')
		if (buf[0] == 'q' || buf[0] == 'Q') && n == 1 && !searchMode {
			clearMenu()
			restore()
			return nil, nil
		}

		// Non-search mode: Slash ('/') -> Enter search mode
		if buf[0] == '/' && n == 1 && !searchMode {
			searchMode = true
			render()
			continue
		}

		// Escape sequences (arrows, home, end, pgup, pgdn)
		if buf[0] == 27 && n >= 3 {
			if buf[1] == '[' || buf[1] == 'O' {
				switch buf[2] {
				case 'A': // Up
					if numDisplayed > 0 {
						cursor = (cursor - 1 + numDisplayed) % numDisplayed
						render()
					}
					continue
				case 'B': // Down
					if numDisplayed > 0 {
						cursor = (cursor + 1) % numDisplayed
						render()
					}
					continue
				case 'H', '1', '7': // Home
					cursor = 0
					render()
					continue
				case 'F', '4', '8': // End
					if numDisplayed > 0 {
						cursor = numDisplayed - 1
					}
					render()
					continue
				case '5': // Page Up
					cursor -= 5
					if cursor < 0 {
						cursor = 0
					}
					render()
					continue
				case '6': // Page Down
					cursor += 5
					if cursor >= numDisplayed && numDisplayed > 0 {
						cursor = numDisplayed - 1
					}
					render()
					continue
				}
			}
		}

		// Single character inputs in normal (non-search) mode
		if n == 1 && !searchMode {
			switch buf[0] {
			case 'k', 'K', 16: // k, K, Ctrl+P (Up)
				if numDisplayed > 0 {
					cursor = (cursor - 1 + numDisplayed) % numDisplayed
					render()
				}
				continue
			case 'j', 'J', 14: // j, J, Ctrl+N (Down)
				if numDisplayed > 0 {
					cursor = (cursor + 1) % numDisplayed
					render()
				}
				continue
			case 'g': // Jump to top
				cursor = 0
				render()
				continue
			case 'G': // Jump to bottom
				if numDisplayed > 0 {
					cursor = numDisplayed - 1
				}
				render()
				continue
			}

			// Number jump 1..9
			if buf[0] >= '1' && buf[0] <= '9' {
				num := int(buf[0] - '1')
				if num < numDisplayed {
					cursor = num
					render()
					continue
				}
			}
		}
	}
}

// PromptSelector implements Selector using standard line-based input.
type PromptSelector struct {
	In  io.Reader
	Out io.Writer
}

// NewPromptSelector creates a line-based prompt selector.
func NewPromptSelector(in io.Reader, out io.Writer) *PromptSelector {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	return &PromptSelector{In: in, Out: out}
}

// SelectSession prompts user via numbered options on Out, reading choice from In.
func (p *PromptSelector) SelectSession(sessions []profile.ConversationSession) (*profile.ConversationSession, error) {
	if len(sessions) == 0 {
		return nil, nil
	}

	termWidth := GetTerminalWidth()
	for i, s := range sessions {
		fmt.Fprintln(p.Out, FormatSessionLine(i+1, s, false, termWidth))
	}
	fmt.Fprintln(p.Out)
	fmt.Fprintf(p.Out, "Select session to resume [1-%d, or Enter to exit]: ", len(sessions))

	reader := bufio.NewReader(p.In)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, nil
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	choice, parseErr := strconv.Atoi(input)
	if parseErr != nil || choice < 1 || choice > len(sessions) {
		return nil, fmt.Errorf("invalid choice %q: must be between 1 and %d", input, len(sessions))
	}
	return &sessions[choice-1], nil
}

// AdaptiveSelector tries terminal interactive selector first, falling back to line prompt if non-interactive.
type AdaptiveSelector struct {
	terminal Selector
	prompt   Selector
}

// NewAdaptiveSelector creates an adaptive selector choosing based on terminal presence.
func NewAdaptiveSelector(termSel Selector, promptSel Selector) *AdaptiveSelector {
	if termSel == nil {
		termSel = NewTerminalSelector()
	}
	if promptSel == nil {
		promptSel = NewPromptSelector(nil, nil)
	}
	return &AdaptiveSelector{
		terminal: termSel,
		prompt:   promptSel,
	}
}

// SelectSession picks using terminal or fallback prompt.
func (a *AdaptiveSelector) SelectSession(sessions []profile.ConversationSession) (*profile.ConversationSession, error) {
	chosen, err := a.terminal.SelectSession(sessions)
	if err != nil {
		return a.prompt.SelectSession(sessions)
	}
	return chosen, nil
}
