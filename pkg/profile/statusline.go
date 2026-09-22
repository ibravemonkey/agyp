package profile

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	sessionContextFilename = ".session_context"
	statuslineBackupFile   = "statusline.original.json"
)

// SessionContextState stores cached context window metrics for an active session.
type SessionContextState struct {
	UsedPercentage      float64   `json:"used_percentage"`
	InputTokens         int64     `json:"input_tokens,omitempty"`
	OutputTokens        int64     `json:"output_tokens,omitempty"`
	CacheReadTokens     int64     `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens int64     `json:"cache_creation_tokens,omitempty"`
	DurationSeconds     float64   `json:"duration_seconds,omitempty"`
	Speed               float64   `json:"speed,omitempty"`
	ModelID             string    `json:"model_id,omitempty"`
	ModelDisplayName    string    `json:"model_display_name,omitempty"`
	ConversationTitle   string    `json:"conversation_title,omitempty"`
	ConversationID      string    `json:"conversation_id,omitempty"`
	Cost                float64   `json:"cost,omitempty"`
	Effort              string    `json:"effort,omitempty"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// StatusLinePayload represents the JSON payload streamed to stdin by Antigravity CLI statusLine command.
type StatusLinePayload struct {
	ConversationID       string  `json:"conversation_id,omitempty"`
	SessionID            string  `json:"session_id,omitempty"`
	ConversationTitle    string  `json:"conversation_title"`
	ConversationTitleAlt string  `json:"conversationTitle,omitempty"`
	Title                string   `json:"title,omitempty"`
	AgentState           string   `json:"agent_state,omitempty"`
	State                string   `json:"state,omitempty"`
	Status               string   `json:"status,omitempty"`
	Workspace            string   `json:"workspace,omitempty"`
	Workspaces           []string `json:"workspaces,omitempty"`
	Root                 string   `json:"root,omitempty"`
	Cwd                  string   `json:"cwd,omitempty"`
	Branch               string   `json:"branch,omitempty"`
	Timestamp            string   `json:"timestamp,omitempty"`
	Cost                 float64  `json:"cost"`
	Effort               string   `json:"effort,omitempty"`
	ReasoningEffort      string   `json:"reasoning_effort,omitempty"`
	DurationMs           float64  `json:"duration_ms,omitempty"`
	Duration             float64  `json:"duration,omitempty"`
	LatencyMs            float64  `json:"latency_ms,omitempty"`
	Speed                float64  `json:"speed,omitempty"`
	TokensPerSecond      float64  `json:"tokens_per_second,omitempty"`
	OutputTokens         int64    `json:"output_tokens,omitempty"`
	InputTokens          int64    `json:"input_tokens,omitempty"`
	CacheTokens          int64    `json:"cache_tokens,omitempty"`
	Model                struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	ContextWindow *struct {
		UsedPercentage      float64 `json:"used_percentage"`
		UsedPercentageAlt   float64 `json:"usedPercentage,omitempty"`
		TotalInputTokens    int64   `json:"total_input_tokens,omitempty"`
		TotalOutputTokens   int64   `json:"total_output_tokens,omitempty"`
		CurrentUsage        struct {
			InputTokens            int64   `json:"input_tokens"`
			InputTokensAlt         int64   `json:"inputTokens,omitempty"`
			OutputTokens           int64   `json:"output_tokens"`
			OutputTokensAlt        int64   `json:"outputTokens,omitempty"`
			CacheReadTokens        int64   `json:"cache_read_input_tokens"`
			CacheReadTokensAlt     int64   `json:"cacheReadInputTokens,omitempty"`
			CacheCreationTokens    int64   `json:"cache_creation_input_tokens"`
			CacheCreationTokensAlt int64   `json:"cacheCreationInputTokens,omitempty"`
			DurationMs             float64 `json:"duration_ms,omitempty"`
			Speed                  float64 `json:"speed,omitempty"`
		} `json:"current_usage"`
	} `json:"context_window"`
	ContextWindowAlt *struct {
		UsedPercentage float64 `json:"usedPercentage"`
	} `json:"contextWindow"`
	Quota map[string]struct {
		RemainingFraction    float64 `json:"remaining_fraction"`
		RemainingFractionAlt float64 `json:"remainingFraction"`
		ResetTime            string  `json:"reset_time"`
		ResetTimeAlt         string  `json:"resetTime"`
		ResetInSeconds       uint64  `json:"reset_in_seconds"`
		ResetInSecondsAlt    uint64  `json:"resetInSeconds"`
	} `json:"quota"`
}

func getSessionContextPathForPane(profileDir, paneID string) string {
	if paneID != "" {
		sanitized := strings.ReplaceAll(paneID, ":", "_")
		sanitized = strings.ReplaceAll(sanitized, "/", "_")
		return filepath.Join(profileDir, fmt.Sprintf(".session_context_%s.json", sanitized))
	}
	return filepath.Join(profileDir, sessionContextFilename)
}

func getSessionContextPath(profileDir string) string {
	return getSessionContextPathForPane(profileDir, os.Getenv("HERDR_PANE_ID"))
}

// SaveSessionContextForPane saves session context for a specific pane.
func SaveSessionContextForPane(profileDir, paneID string, state *SessionContextState) error {
	if profileDir == "" || state == nil {
		return nil
	}
	state.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	targetPath := getSessionContextPathForPane(profileDir, paneID)
	return WriteFileAtomicNoSync(targetPath, data, 0600)
}

// SaveSessionContext saves the context window percentage and metrics to the profile directory.
func SaveSessionContext(profileDir string, state *SessionContextState) error {
	return SaveSessionContextForPane(profileDir, os.Getenv("HERDR_PANE_ID"), state)
}

// ResetSessionContext removes the cached session context file for a profile.
func ResetSessionContext(profileDir string) error {
	if profileDir == "" {
		return nil
	}
	targetPath := getSessionContextPath(profileDir)
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// GetSessionContextStateForPane returns the cached session context state for a specific pane.
func GetSessionContextStateForPane(profileDir, paneID string) (*SessionContextState, bool) {
	if profileDir == "" {
		return nil, false
	}
	targetPath := getSessionContextPathForPane(profileDir, paneID)
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, false
	}

	var state SessionContextState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, false
	}

	return &state, true
}

// GetSessionContextState returns the cached session context state if valid.
func GetSessionContextState(profileDir string) (*SessionContextState, bool) {
	return GetSessionContextStateForPane(profileDir, os.Getenv("HERDR_PANE_ID"))
}

// GetSessionContext returns the cached context window percentage (0-100) if valid and not expired (TTL 2 hours).
func GetSessionContext(profileDir string) (int, bool) {
	state, ok := GetSessionContextState(profileDir)
	if !ok || state == nil {
		return 0, false
	}
	if time.Since(state.UpdatedAt) > 2*time.Hour {
		return 0, false
	}
	pct := int(state.UsedPercentage + 0.5)
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	return pct, true
}

// FormatCost formats a dollar cost amount with 2 to 4 decimal places (e.g. $0.00, $0.0042, $1.25).
func FormatCost(cost float64) string {
	if cost <= 0 {
		return "$0.00"
	}
	s := fmt.Sprintf("%.4f", cost)
	s = strings.TrimRight(s, "0")
	if strings.HasSuffix(s, ".") {
		s += "00"
	} else {
		parts := strings.Split(s, ".")
		if len(parts) == 2 && len(parts[1]) < 2 {
			s += strings.Repeat("0", 2-len(parts[1]))
		}
	}
	if strings.HasPrefix(s, ".") {
		s = "0" + s
	}
	return "$" + s
}

// TokenTelemetry holds turn-level token, latency, and throughput metrics.
type TokenTelemetry struct {
	InputTokens     int64
	OutputTokens    int64
	CacheTokens     int64
	CtxPct          int
	HasCtx          bool
	DurationSeconds float64
	Speed           float64
}

// HasData reports whether any telemetry metric is present.
func (t TokenTelemetry) HasData() bool {
	return t.InputTokens > 0 || t.OutputTokens > 0 || t.CacheTokens > 0 || (t.HasCtx && t.CtxPct > 0) || t.DurationSeconds > 0 || t.Speed > 0
}
// FormatTokenCount formats a token number (e.g. 2300 -> "2.3K", 918 -> "918", 130000 -> "130K", 1500000 -> "1.5M").
func FormatTokenCount(n int64) string {
	if n <= 0 {
		return "0"
	}
	if n >= 1_000_000 {
		f := float64(n) / 1_000_000.0
		if f == float64(int64(f)) {
			return fmt.Sprintf("%dM", int64(f))
		}
		return fmt.Sprintf("%.1fM", f)
	}
	if n >= 1_000 {
		f := float64(n) / 1_000.0
		if n < 10_000 {
			if f == float64(int64(f)) {
				return fmt.Sprintf("%dK", int64(f))
			}
			return fmt.Sprintf("%.1fK", f)
		}
		return fmt.Sprintf("%dK", int64((float64(n)+500)/1000.0))
	}
	return fmt.Sprintf("%d", n)
}

// FormatDurationSec formats latency / turn duration (e.g. 2.8 -> "2.8s", 74.0 -> "1m14s").
func FormatDurationSec(d float64) string {
	if d <= 0 {
		return ""
	}
	if d < 60 {
		return fmt.Sprintf("%.1fs", d)
	}
	mins := int(d) / 60
	secs := int(d) % 60
	return fmt.Sprintf("%dm%02ds", mins, secs)
}

// FormatSpeed formats token generation speed (e.g. 179.2 -> "179.2/s").
func FormatSpeed(s float64) string {
	if s <= 0 {
		return ""
	}
	return fmt.Sprintf("%.1f/s", s)
}

// FormatTokenTelemetry renders the active generation speed telemetry (e.g.  179.2/s).
func FormatTokenTelemetry(t TokenTelemetry, useColor bool) string {
	if t.Speed <= 0 {
		return ""
	}

	cIcon := "\033[38;5;103m"
	cVal := "\033[38;5;252m"
	cRst := "\033[0m"
	if !useColor {
		cIcon = ""
		cVal = ""
		cRst = ""
	}

	return fmt.Sprintf("%s\uf0e4%s %s%s%s", cIcon, cRst, cVal, FormatSpeed(t.Speed), cRst)
}

// ResolveActiveEffort determines the active reasoning effort for a profile and model.
func ResolveActiveEffort(profileDir, modelName, explicitEffort string) string {
	if explicitEffort != "" {
		return explicitEffort
	}
	if profileDir != "" {
		// 1. Check .active_effort cache
		data, err := os.ReadFile(filepath.Join(profileDir, ".active_effort"))
		if err == nil && len(strings.TrimSpace(string(data))) > 0 {
			return strings.TrimSpace(string(data))
		}
		// 2. Check settings.json
		for _, subDir := range []string{"antigravity-cli", "antigravity", "antigravity-ide"} {
			sPath := filepath.Join(profileDir, ".gemini", subDir, "settings.json")
			if sData, err := os.ReadFile(sPath); err == nil {
				var settings map[string]interface{}
				if json.Unmarshal(sData, &settings) == nil {
					if eff, ok := settings["effort"].(string); ok && eff != "" {
						return eff
					}
					if eff, ok := settings["reasoning_effort"].(string); ok && eff != "" {
						return eff
					}
				}
			}
		}
	}
	// 3. Default to "high" for models supporting effort
	if ModelSupportsEffort(modelName) {
		return "high"
	}
	return ""
}

// TurnTimingState stores timestamps and metrics for calculating turn duration and generation speed.
type TurnTimingState struct {
	BusyStartTime    time.Time `json:"busy_start_time"`
	LastDuration     float64   `json:"last_duration"`
	LastSpeed        float64   `json:"last_speed"`
	LastOutputTokens int64     `json:"last_output_tokens"`
}

func getTurnTimingPath(profileDir string) string {
	return filepath.Join(profileDir, ".agyp_turn_timing.json")
}

func loadTurnTiming(profileDir string) *TurnTimingState {
	if profileDir == "" {
		return &TurnTimingState{}
	}
	data, err := os.ReadFile(getTurnTimingPath(profileDir))
	if err != nil {
		return &TurnTimingState{}
	}
	var state TurnTimingState
	if err := json.Unmarshal(data, &state); err != nil {
		return &TurnTimingState{}
	}
	return &state
}

func saveTurnTiming(profileDir string, state *TurnTimingState) {
	if profileDir == "" || state == nil {
		return
	}
	data, err := json.Marshal(state)
	if err == nil {
		_ = WriteFileAtomicNoSync(getTurnTimingPath(profileDir), data, 0600)
	}
}

func parseLastTurnDurationFromTranscript(profileDir, convID string) float64 {
	if profileDir == "" || convID == "" {
		return 0
	}
	for _, bDir := range getProfileBrainDirs(profileDir) {
		trPath := filepath.Join(bDir, convID, ".system_generated", "logs", "transcript.jsonl")
		file, err := os.Open(trPath)
		if err != nil {
			continue
		}

		stat, err := file.Stat()
		if err != nil || stat.Size() == 0 {
			_ = file.Close()
			continue
		}

		offset := int64(0)
		readSize := stat.Size()
		if readSize > 32768 {
			offset = stat.Size() - 32768
			readSize = 32768
		}

		buf := make([]byte, readSize)
		_, err = file.ReadAt(buf, offset)
		_ = file.Close()
		if err != nil && err != io.EOF {
			continue
		}

		lines := strings.Split(string(buf), "\n")
		var modelTime, userTime time.Time

		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}

			var entry struct {
				Source    string `json:"source"`
				Type      string `json:"type"`
				CreatedAt string `json:"created_at"`
			}
			if err := json.Unmarshal([]byte(line), &entry); err == nil && entry.CreatedAt != "" {
				tVal, err := time.Parse(time.RFC3339, entry.CreatedAt)
				if err != nil {
					tVal, err = time.Parse("2006-01-02T15:04:05Z", entry.CreatedAt)
				}
				if err == nil {
					if modelTime.IsZero() && (entry.Source == "MODEL" || entry.Type == "PLANNER_RESPONSE" || entry.Type == "GENERIC") {
						modelTime = tVal
					} else if !modelTime.IsZero() && (entry.Source == "USER_EXPLICIT" || entry.Type == "USER_INPUT") {
						userTime = tVal
						break
					}
				}
			}
		}

		if !modelTime.IsZero() && !userTime.IsZero() && modelTime.After(userTime) {
			diff := modelTime.Sub(userTime).Seconds()
			if diff >= 0.1 && diff <= 600 {
				return diff
			}
		}
	}
	return 0
}

func resolveTurnDurationAndSpeed(profileDir, convID, agentState string, outputTokens int64, explicitDuration, explicitSpeed float64) (float64, float64) {
	if explicitDuration > 0 {
		speed := explicitSpeed
		if speed == 0 && outputTokens > 0 {
			speed = float64(outputTokens) / explicitDuration
		}
		return explicitDuration, speed
	}

	timing := loadTurnTiming(profileDir)
	curState := strings.ToLower(strings.TrimSpace(agentState))
	isBusy := (curState == "working" || curState == "thinking" || curState == "generating" || curState == "busy" || curState == "running")
	isDone := (curState == "done" || curState == "idle" || curState == "success" || curState == "completed" || curState == "waiting")

	now := time.Now()

	if isBusy {
		if timing.BusyStartTime.IsZero() || now.Sub(timing.BusyStartTime) > 15*time.Minute {
			timing.BusyStartTime = now
			saveTurnTiming(profileDir, timing)
		}
	} else if isDone {
		if !timing.BusyStartTime.IsZero() {
			dur := now.Sub(timing.BusyStartTime).Seconds()
			if dur >= 0.2 && dur < 600 {
				timing.LastDuration = float64(int(dur*10)) / 10.0
				if outputTokens > 0 {
					timing.LastSpeed = float64(outputTokens) / timing.LastDuration
				}
				timing.LastOutputTokens = outputTokens
			}
			timing.BusyStartTime = time.Time{}
			saveTurnTiming(profileDir, timing)
		}
	}

	duration := timing.LastDuration
	speed := timing.LastSpeed

	if duration == 0 && convID != "" && profileDir != "" {
		if trDur := parseLastTurnDurationFromTranscript(profileDir, convID); trDur > 0 {
			duration = float64(int(trDur*10)) / 10.0
			if outputTokens > 0 {
				speed = float64(outputTokens) / duration
			}
			timing.LastDuration = duration
			timing.LastSpeed = speed
			saveTurnTiming(profileDir, timing)
		}
	}

	if duration == 0 && outputTokens > 0 {
		// Realistic baseline for Gemini Flash generation (~160-190 tok/s)
		estDur := float64(outputTokens) / 180.0
		if estDur < 0.5 {
			estDur = 0.5
		}
		duration = float64(int(estDur*10)) / 10.0
		speed = float64(outputTokens) / duration
	}

	return duration, speed
}

// HandleStatusLine processes the statusLine input from Antigravity CLI, updates local session cache,
// reports real-time metadata to Herdr, and chains previous statusLine command if one was configured.
func HandleStatusLine(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	if os.Getenv("AGYP_INTERNAL_EXEC") != "" {
		return nil
	}

	var input []byte
	if stdin != nil {
		input, _ = io.ReadAll(stdin)
	}

	var payload StatusLinePayload
	if len(input) > 0 {
		_ = json.Unmarshal(input, &payload)
	}

	// Resolve active profile and profile directory from current session environment
	currentProfile, profileDir := ResolveProfileFromEnv()

	// Extract context window metrics & token telemetry
	var ctxUsedPct float64
	var hasCtx bool
	var inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens int64
	var durationSec, speedVal float64

	if payload.ContextWindow != nil {
		ctxUsedPct = payload.ContextWindow.UsedPercentage
		if ctxUsedPct == 0 {
			ctxUsedPct = payload.ContextWindow.UsedPercentageAlt
		}
		hasCtx = true
		inputTokens = payload.ContextWindow.CurrentUsage.InputTokens
		if inputTokens == 0 {
			inputTokens = payload.ContextWindow.CurrentUsage.InputTokensAlt
		}
		outputTokens = payload.ContextWindow.CurrentUsage.OutputTokens
		if outputTokens == 0 {
			outputTokens = payload.ContextWindow.CurrentUsage.OutputTokensAlt
		}
		cacheReadTokens = payload.ContextWindow.CurrentUsage.CacheReadTokens
		if cacheReadTokens == 0 {
			cacheReadTokens = payload.ContextWindow.CurrentUsage.CacheReadTokensAlt
		}
		cacheCreationTokens = payload.ContextWindow.CurrentUsage.CacheCreationTokens
		if cacheCreationTokens == 0 {
			cacheCreationTokens = payload.ContextWindow.CurrentUsage.CacheCreationTokensAlt
		}
		if payload.ContextWindow.CurrentUsage.DurationMs > 0 {
			durationSec = payload.ContextWindow.CurrentUsage.DurationMs / 1000.0
		}
		if payload.ContextWindow.CurrentUsage.Speed > 0 {
			speedVal = payload.ContextWindow.CurrentUsage.Speed
		}
	} else if payload.ContextWindowAlt != nil {
		ctxUsedPct = payload.ContextWindowAlt.UsedPercentage
		hasCtx = true
	}

	// Fallback to top-level fields
	if inputTokens == 0 && payload.InputTokens > 0 {
		inputTokens = payload.InputTokens
	}
	if outputTokens == 0 && payload.OutputTokens > 0 {
		outputTokens = payload.OutputTokens
	}
	if durationSec == 0 {
		if payload.DurationMs > 0 {
			durationSec = payload.DurationMs / 1000.0
		} else if payload.Duration > 0 {
			if payload.Duration > 100 {
				durationSec = payload.Duration / 1000.0
			} else {
				durationSec = payload.Duration
			}
		} else if payload.LatencyMs > 0 {
			durationSec = payload.LatencyMs / 1000.0
		}
	}
	if speedVal == 0 {
		if payload.Speed > 0 {
			speedVal = payload.Speed
		} else if payload.TokensPerSecond > 0 {
			speedVal = payload.TokensPerSecond
		} else if outputTokens > 0 && durationSec > 0 {
			speedVal = float64(outputTokens) / durationSec
		}
	}

	cacheTokens := cacheReadTokens + cacheCreationTokens
	if cacheTokens == 0 && payload.CacheTokens > 0 {
		cacheTokens = payload.CacheTokens
	}

	activeModel := payload.Model.ID
	if activeModel == "" {
		activeModel = payload.Model.DisplayName
	}
	if profileDir != "" {
		activeModel = ResolveActiveModel(profileDir, activeModel)
	}

	var existingState *SessionContextState
	if profileDir != "" {
		existingState, _ = GetSessionContextState(profileDir)
	}

	costVal := payload.Cost
	effortVal := payload.Effort
	if effortVal == "" {
		effortVal = payload.ReasoningEffort
	}
	if effortVal == "" && profileDir != "" {
		effortVal = ResolveActiveEffort(profileDir, activeModel, "")
	}
	convID := payload.ConversationID
	if convID == "" {
		convID = payload.SessionID
	}
	if convID == "" && existingState != nil && existingState.ConversationID != "" {
		convID = existingState.ConversationID
	}
	if convID == "" && profileDir != "" {
		if latestID, _, err := GetLatestConversationFileInfo(filepath.Base(profileDir)); err == nil && latestID != "" {
			convID = latestID
		}
	}
	convTitle := payload.ConversationTitle
	if convTitle == "" {
		convTitle = payload.ConversationTitleAlt
	}
	if convTitle == "" {
		convTitle = payload.Title
	}
	if convTitle == "" && existingState != nil && existingState.ConversationTitle != "" {
		if convID == "" || existingState.ConversationID == "" || existingState.ConversationID == convID {
			convTitle = existingState.ConversationTitle
		}
	}
	if convTitle == "" && profileDir != "" && convID != "" {
		convTitle = ResolveConversationTitle(profileDir, convID)
	}
	if convTitle != "" {
		convTitle = cleanPromptSummary(convTitle)
		if convTitle == "(No prompt summary)" {
			convTitle = ""
		}
	}

	if profileDir != "" && (hasCtx || convTitle != "" || payload.Cost > 0 || convID != "" || inputTokens > 0 || outputTokens > 0) {
		state := &SessionContextState{
			UsedPercentage:      ctxUsedPct,
			InputTokens:         inputTokens,
			OutputTokens:        outputTokens,
			CacheReadTokens:     cacheTokens,
			CacheCreationTokens: cacheCreationTokens,
			DurationSeconds:     durationSec,
			Speed:               speedVal,
			ModelID:             payload.Model.ID,
			ModelDisplayName:    payload.Model.DisplayName,
			ConversationTitle:   convTitle,
			ConversationID:      convID,
			Cost:                payload.Cost,
			Effort:              effortVal,
		}
		if existingState != nil {
			isSameConv := true
			if convID != "" && existingState.ConversationID != "" && convID != existingState.ConversationID {
				isSameConv = false
			}
			if isSameConv {
				if !hasCtx {
					state.UsedPercentage = existingState.UsedPercentage
					state.InputTokens = existingState.InputTokens
					state.OutputTokens = existingState.OutputTokens
					state.CacheReadTokens = existingState.CacheReadTokens
					state.CacheCreationTokens = existingState.CacheCreationTokens
					state.DurationSeconds = existingState.DurationSeconds
					state.Speed = existingState.Speed
				} else {
					if state.InputTokens == 0 && existingState.InputTokens > 0 {
						state.InputTokens = existingState.InputTokens
					}
					if state.OutputTokens == 0 && existingState.OutputTokens > 0 {
						state.OutputTokens = existingState.OutputTokens
					}
					if state.CacheReadTokens == 0 && existingState.CacheReadTokens > 0 {
						state.CacheReadTokens = existingState.CacheReadTokens
					}
					if state.CacheCreationTokens == 0 && existingState.CacheCreationTokens > 0 {
						state.CacheCreationTokens = existingState.CacheCreationTokens
					}
					if state.DurationSeconds == 0 && existingState.DurationSeconds > 0 {
						state.DurationSeconds = existingState.DurationSeconds
					}
					if state.Speed == 0 && existingState.Speed > 0 {
						state.Speed = existingState.Speed
					}
				}
				if state.ConversationTitle == "" && existingState.ConversationTitle != "" {
					state.ConversationTitle = existingState.ConversationTitle
				}
				if state.ConversationID == "" {
					state.ConversationID = existingState.ConversationID
				}
				if state.Cost == 0 {
					state.Cost = existingState.Cost
				}
			}
			if state.ModelID == "" {
				state.ModelID = existingState.ModelID
			}
			if state.ModelDisplayName == "" {
				state.ModelDisplayName = existingState.ModelDisplayName
			}
			if state.Effort == "" {
				state.Effort = existingState.Effort
			}
		}
		if existingState == nil || !isSessionContextStateEqual(existingState, state) {
			_ = SaveSessionContext(profileDir, state)
		}
		if inputTokens > 0 || outputTokens > 0 || cacheTokens > 0 {
			_ = RecordTokenUsage(currentProfile, activeModel, convID, inputTokens, outputTokens, cacheTokens)
		}
		costVal = state.Cost
		if state.Effort != "" {
			effortVal = state.Effort
		}
	}

	// If active model is provided in payload, update .active_model cache and settings.json only if changed
	if payload.Model.ID != "" && profileDir != "" {
		activeModelPath := filepath.Join(profileDir, ".active_model")
		if curr, err := os.ReadFile(activeModelPath); err != nil || strings.TrimSpace(string(curr)) != payload.Model.ID {
			_ = WriteFileAtomic(activeModelPath, []byte(payload.Model.ID+"\n"), 0600)
			SyncModelToSettings(profileDir, payload.Model.ID)
		}
	}

	// Retrieve real-time quota details for CLI statusline footer & Herdr update (0ms non-blocking)
	var quotaDetails *ModelQuotaDetails

	// 1. Check local cache (0ms, Stale-While-Revalidate with async background refresh)
	if currentProfile != "" {
		if fast, ok := GetProfileFullQuotaDetailsFast(currentProfile, activeModel); ok && fast != nil {
			quotaDetails = fast
		}
	}

	// 2. Fallback to quota in stdin payload ONLY if local cache was missing or empty
	if (quotaDetails == nil || quotaDetails.Fraction5H < 0) && len(payload.Quota) > 0 {
		if fb := parsePayloadQuota(payload.Quota, activeModel); fb != nil {
			quotaDetails = fb
		}
	}

	// If inside Herdr environment, trigger immediate metadata refresh for instant zero-latency sidebar update
	if IsInHerdrEnvironment() && currentProfile != "" {
		reportCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		_ = ReportHerdrMetadataWithModel(reportCtx, currentProfile, activeModel, quotaDetails)
	}

	// Extract workspace name
	var workspaceName string
	cwd, _ := os.Getwd()
	if payload.Workspace != "" {
		workspaceName = payload.Workspace
	} else if len(payload.Workspaces) > 0 && payload.Workspaces[0] != "" {
		workspaceName = payload.Workspaces[0]
	} else if payload.Root != "" {
		base := filepath.Base(payload.Root)
		if base != "/" && base != "." {
			workspaceName = base
		}
	} else if cwd != "" {
		base := filepath.Base(cwd)
		if base != "/" && base != "." {
			workspaceName = base
		}
	}

	// Detect git branch
	searchDir := cwd
	if payload.Root != "" {
		searchDir = payload.Root
	} else if payload.Cwd != "" {
		searchDir = payload.Cwd
	}
	gitBranch := payload.Branch
	if gitBranch == "" {
		gitBranch = findGitBranch(searchDir)
	}

	// Extract agent state
	agentState := payload.AgentState
	if agentState == "" {
		agentState = payload.State
	}
	if agentState == "" {
		agentState = payload.Status
	}

	// Trigger completion audio/notification if transitioning to done
	triggerCompletionSound(profileDir, currentProfile, agentState)
	// Resolve turn duration and speed (tracks live Working->Done latency or calculates from transcript)
	dur, spd := resolveTurnDurationAndSpeed(profileDir, convID, agentState, outputTokens, durationSec, speedVal)
	if dur > 0 {
		durationSec = dur
	}
	if spd > 0 {
		speedVal = spd
	}

	// Format high-contrast real-time telemetry string for Antigravity CLI footer
	useColor := os.Getenv("NO_COLOR") == ""
	ctxPct := int(ctxUsedPct + 0.5)

	telemetry := TokenTelemetry{
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		CacheTokens:     cacheTokens,
		CtxPct:          ctxPct,
		HasCtx:          hasCtx,
		DurationSeconds: durationSec,
		Speed:           speedVal,
	}
	if existingState != nil {
		if telemetry.InputTokens == 0 {
			telemetry.InputTokens = existingState.InputTokens
		}
		if telemetry.OutputTokens == 0 {
			telemetry.OutputTokens = existingState.OutputTokens
		}
		if telemetry.CacheTokens == 0 {
			telemetry.CacheTokens = existingState.CacheReadTokens
		}
		if telemetry.DurationSeconds == 0 {
			telemetry.DurationSeconds = existingState.DurationSeconds
		}
		if telemetry.Speed == 0 {
			telemetry.Speed = existingState.Speed
		}
	}

	statusLineStr := FormatStatusLineTextExtended(currentProfile, workspaceName, gitBranch, agentState, activeModel, effortVal, costVal, ctxPct, hasCtx, quotaDetails, useColor, telemetry)
	if quotaAlert := CheckAndHandleInFlightQuota(ctx, currentProfile, profileDir, convID, quotaDetails, useColor); quotaAlert != "" {
		statusLineStr += quotaAlert
	}
	if os.Getenv("AGYP_STATUSLINE") == "off" || os.Getenv("AGYP_NO_STATUSLINE") != "" {
		// Output suppressed
	} else {
		if stdout != nil && statusLineStr != "" {
			fmt.Fprintln(stdout, statusLineStr)
		}
	}

	// Chain previous statusLine command if one was preserved
	if profileDir != "" {
		chainPreviousStatusLine(ctx, profileDir, input, stdout, stderr)
	}

	return nil
}

// CheckAndHandleInFlightQuota detects 5h quota exhaustion (<= 5%), searches for an alternative
// profile with available quota, writes an auto-switch pending marker, stages the fresh token,
// and returns an alert message for the statusline.
func CheckAndHandleInFlightQuota(ctx context.Context, currentProfile, profileDir, convID string, quotaDetails *ModelQuotaDetails, useColor bool) string {
	if currentProfile == "" || profileDir == "" || quotaDetails == nil {
		return ""
	}
	if quotaDetails.Fraction5H < 0 || quotaDetails.Fraction5H > 0.05 {
		return ""
	}

	// Avoid excessive re-checks if checked within last 15 seconds
	markerPath := filepath.Join(profileDir, ".auto_switch_pending")
	if info, err := os.Stat(markerPath); err == nil {
		if time.Since(info.ModTime()) < 15*time.Second {
			if data, readErr := os.ReadFile(markerPath); readErr == nil {
				var cached struct {
					NextProfile string  `json:"next_profile"`
					Score       float64 `json:"score"`
				}
				if json.Unmarshal(data, &cached) == nil && cached.NextProfile != "" {
					if useColor {
						return fmt.Sprintf(" \033[1;33m⚡ 429: квота исчерпана -> готов %s (%.0f%%)\033[0m", cached.NextProfile, cached.Score*100)
					}
					return fmt.Sprintf(" [⚡ 429: квота исчерпана -> готов %s (%.0f%%)]", cached.NextProfile, cached.Score*100)
				}
			}
		}
	}

	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	candidate, candidateScore, err := SelectBestProfileFiltered(checkCtx, func(p string) bool {
		return p != currentProfile
	})
	if err != nil || candidate == "" || candidateScore <= 0.05 {
		return ""
	}

	candidateDir, dirErr := GetProfileDir(candidate)
	if dirErr == nil {
		// Stage token in background for potential in-flight re-read
		if tokData, tokErr := ReadRawTokenData(candidateDir); tokErr == nil && len(tokData) > 0 {
			_ = WriteTokenToProfile(profileDir, string(tokData))
		}
	}

	// Record pending auto switch
	markerData, _ := json.Marshal(map[string]interface{}{
		"current_profile": currentProfile,
		"next_profile":    candidate,
		"conversation_id": convID,
		"score":           candidateScore,
		"updated_at":      time.Now(),
	})
	_ = WriteFileAtomic(markerPath, markerData, 0600)

	if useColor {
		return fmt.Sprintf(" \033[1;33m⚡ 429: квота исчерпана -> готов %s (%.0f%%)\033[0m", candidate, candidateScore*100)
	}
	return fmt.Sprintf(" [⚡ 429: квота исчерпана -> готов %s (%.0f%%)]", candidate, candidateScore*100)
}

func parsePayloadQuota(quotaMap map[string]struct {
	RemainingFraction    float64 `json:"remaining_fraction"`
	RemainingFractionAlt float64 `json:"remainingFraction"`
	ResetTime            string  `json:"reset_time"`
	ResetTimeAlt         string  `json:"resetTime"`
	ResetInSeconds       uint64  `json:"reset_in_seconds"`
	ResetInSecondsAlt    uint64  `json:"resetInSeconds"`
}, activeModel ...string) *ModelQuotaDetails {
	if len(quotaMap) == 0 {
		return nil
	}
	details := &ModelQuotaDetails{
		Fraction5H:     -1.0,
		FractionWeekly: -1.0,
	}
	modelFilter := ""
	if len(activeModel) > 0 {
		modelFilter = strings.ToLower(strings.TrimSpace(activeModel[0]))
	}
	is3P := strings.Contains(modelFilter, "claude") || strings.Contains(modelFilter, "sonnet") || strings.Contains(modelFilter, "opus") ||
		strings.Contains(modelFilter, "gpt") || strings.Contains(modelFilter, "openai") || strings.HasPrefix(modelFilter, "o1") || strings.HasPrefix(modelFilter, "o3")

	// Sort keys deterministically
	keys := make([]string, 0, len(quotaMap))
	for k := range quotaMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	is3PKey := func(k string) bool {
		return strings.Contains(k, "3p") || strings.Contains(k, "claude") || strings.Contains(k, "gpt") ||
			strings.Contains(k, "anthropic") || strings.Contains(k, "openai")
	}

	parseKey := func(key string) {
		q := quotaMap[key]
		k := strings.ToLower(key)

		frac := q.RemainingFraction
		if frac == 0 && q.RemainingFractionAlt > 0 {
			frac = q.RemainingFractionAlt
		}
		rTime := q.ResetTime
		if rTime == "" {
			rTime = q.ResetTimeAlt
		}
		resetSec := q.ResetInSeconds
		if resetSec == 0 && q.ResetInSecondsAlt > 0 {
			resetSec = q.ResetInSecondsAlt
		}
		var parsedReset time.Time
		if rTime != "" {
			if tVal, tErr := time.Parse(time.RFC3339, rTime); tErr == nil {
				parsedReset = tVal
			} else if tVal, tErr := time.Parse("2006-01-02T15:04:05Z", rTime); tErr == nil {
				parsedReset = tVal
			}
		} else if resetSec > 0 {
			parsedReset = time.Now().Add(time.Duration(resetSec) * time.Second)
		}
		isWeekly := strings.Contains(k, "week") || strings.Contains(k, "7d")
		is5H := strings.Contains(k, "5h") || (strings.Contains(k, "gemini") && !isWeekly)

		if isWeekly && details.FractionWeekly < 0 {
			details.FractionWeekly = frac
			details.ResetTimeWeekly = parsedReset
			details.CompactResetWeekly = FormatCompactResetTime(parsedReset, frac)
		} else if is5H && details.Fraction5H < 0 {
			details.Fraction5H = frac
			details.ResetTime5H = parsedReset
			details.CompactReset5H = FormatCompactResetTime(parsedReset, frac)
		}
	}

	// Pass 1: Strict matching by active model family
	for _, key := range keys {
		k := strings.ToLower(key)
		if !is3P && is3PKey(k) {
			continue // Skip 3P keys when Gemini is active
		}
		if is3P && !is3PKey(k) {
			continue // Skip Gemini keys when 3P model is active
		}
		parseKey(key)
	}

	// Pass 2: Fallback for any missing buckets from remaining keys
	if details.Fraction5H < 0 || details.FractionWeekly < 0 {
		for _, key := range keys {
			parseKey(key)
		}
	}

	if details.Fraction5H >= 0 || details.FractionWeekly >= 0 {
		return details
	}
	return nil
}

func formatAgentState(state string, useColor bool) string {
	raw := strings.TrimSpace(strings.ToLower(state))
	var label, color string
	switch raw {
	case "busy", "active", "running", "working", "generating":
		label = " Working"
		color = "\033[1;33m"
	case "done", "success", "finished", "completed":
		label = " Done"
		color = "\033[1;32m"
	case "thinking":
		label = "󰧑 Thinking"
		color = "\033[1;36m"
	case "waiting", "paused", "wait", "user_input", "waiting_for_input":
		label = " Waiting"
		color = "\033[33m"
	case "idle":
		label = " Idle"
		color = "\033[32m"
	default:
		if raw == "" {
			label = " Idle"
			color = "\033[32m"
		} else {
			label = strings.ToUpper(raw[:1]) + raw[1:]
			color = "\033[32m"
		}
	}
	if useColor {
		return fmt.Sprintf("%s%s\033[0m", color, label)
	}
	return label
}

func triggerCompletionSound(profileDir, profileName, state string) {
	if state == "" {
		return
	}
	curState := strings.TrimSpace(strings.ToLower(state))
	if profileDir == "" {
		profileDir = os.Getenv("AGYP_REAL_HOME")
	}
	stateFile := filepath.Join(profileDir, fmt.Sprintf(".agys_last_state_%s", profileName))
	if profileName == "" {
		stateFile = filepath.Join(profileDir, ".agys_last_state_default")
	}

	prevStateBytes, _ := os.ReadFile(stateFile)
	prevState := strings.TrimSpace(strings.ToLower(string(prevStateBytes)))

	// Check if transition indicates task completion
	isDoneNow := (curState == "done" || curState == "success" || curState == "finished" || curState == "completed" || curState == "idle")
	wasBusyBefore := (prevState == "busy" || prevState == "running" || prevState == "working" || prevState == "generating" || prevState == "thinking")

	if wasBusyBefore && isDoneNow {
		// Portable lookup: check environment override, ~/.gemini/config/bin, ~/.local/bin, PATH
		scriptPath := os.Getenv("AGYP_NOTIFY_SOUND_SCRIPT")
		if scriptPath == "" {
			home, _ := os.UserHomeDir()
			candidates := []string{
				filepath.Join(home, ".gemini", "config", "bin", "notify-sound.sh"),
				filepath.Join(home, ".local", "bin", "notify-sound.sh"),
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					scriptPath = c
					break
				}
			}
		}
		if scriptPath == "" {
			if p, err := exec.LookPath("notify-sound.sh"); err == nil {
				scriptPath = p
			}
		}

		if scriptPath != "" {
			cmd := exec.Command(scriptPath, "finish")
			_ = cmd.Start()
		}
	}

	_ = os.WriteFile(stateFile, []byte(curState), 0644)
}

// FormatStatusLineText formats the real-time statusline text rendered in Antigravity CLI's footer bar (backward compatibility wrapper).
func FormatStatusLineText(profileName, modelName, effort string, cost float64, ctxPct int, hasCtx bool, quotaDetails *ModelQuotaDetails, useColor bool) string {
	return FormatStatusLineTextExtended(profileName, "", "", "", modelName, effort, cost, ctxPct, hasCtx, quotaDetails, useColor)
}

// FormatStatusLineTextExtended formats the enhanced real-time statusline text into organized lines:
// Line 1: Profile, workspace/project, git branch, agent state, % context window.
// Line 2: Active model & effort, 5H quota (with reset time), weekly quota (with reset time), generation speed.
func FormatStatusLineTextExtended(profileName, workspaceName, gitBranch, agentState, modelName, effort string, cost float64, ctxPct int, hasCtx bool, quotaDetails *ModelQuotaDetails, useColor bool, telemetry ...TokenTelemetry) string {
	sep := " · "
	if useColor {
		sep = "\033[90m · \033[0m"
	}

	var line1Parts []string

	// 1. Profile Name
	if profileName != "" {
		pStr := fmt.Sprintf("[%s]", profileName)
		if useColor {
			pStr = fmt.Sprintf("\033[1;36m[%s]\033[0m", profileName)
		}
		line1Parts = append(line1Parts, pStr)
	}

	// 2. Workspace
	if workspaceName != "" {
		wStr := fmt.Sprintf("📦 %s", workspaceName)
		if useColor {
			wStr = fmt.Sprintf("📦 \033[1m%s\033[0m", workspaceName)
		}
		line1Parts = append(line1Parts, wStr)
	}

	// 3. Git Branch
	if gitBranch != "" {
		bStr := fmt.Sprintf(" %s", gitBranch)
		if useColor {
			bStr = fmt.Sprintf("\033[35m %s\033[0m", gitBranch)
		}
		line1Parts = append(line1Parts, bStr)
	}

	// 4. Agent State
	if agentState != "" {
		line1Parts = append(line1Parts, formatAgentState(agentState, useColor))
	}

	// 5. % Context Window
	if hasCtx {
		ctxStr := fmt.Sprintf("%d%% ctx", ctxPct)
		if useColor {
			if ctxPct >= 80 {
				ctxStr = fmt.Sprintf("\033[1;31m%s\033[0m", ctxStr)
			} else if ctxPct >= 50 {
				ctxStr = fmt.Sprintf("\033[33m%s\033[0m", ctxStr)
			} else {
				ctxStr = fmt.Sprintf("\033[36m%s\033[0m", ctxStr)
			}
		}
		line1Parts = append(line1Parts, ctxStr)
	}
	var line2Parts []string

	// 6. Active Model & Effort
	if modelName != "" {
		mStr := modelName
		if effort != "" {
			if useColor {
				mStr = fmt.Sprintf("\033[94m%s\033[0m \033[36m(%s)\033[0m", modelName, effort)
			} else {
				mStr = fmt.Sprintf("%s (%s)", modelName, effort)
			}
		} else {
			if useColor {
				mStr = fmt.Sprintf("\033[94m%s\033[0m", modelName)
			}
		}
		line2Parts = append(line2Parts, mStr)
	}

	// 7. 5H Quota
	if quotaDetails != nil && quotaDetails.Fraction5H >= 0 {
		pct5h := int(quotaDetails.Fraction5H*100 + 0.5)
		q5hText := fmt.Sprintf("%d%%", pct5h)
		if quotaDetails.CompactReset5H != "" {
			q5hText = fmt.Sprintf("%d%% (%s)", pct5h, quotaDetails.CompactReset5H)
		}
		q5hStr := fmt.Sprintf("5H: %s", q5hText)
		if useColor {
			colorCode := "\033[32m"
			if pct5h < 5 {
				colorCode = "\033[1;31m"
			} else if pct5h < 20 {
				colorCode = "\033[33m"
			}
			q5hStr = fmt.Sprintf("\033[90m5H:\033[0m %s%s\033[0m", colorCode, q5hText)
		}
		line2Parts = append(line2Parts, q5hStr)
	}

	// 8. Weekly Quota
	if quotaDetails != nil && quotaDetails.FractionWeekly >= 0 {
		pctWk := int(quotaDetails.FractionWeekly*100 + 0.5)
		qWkText := fmt.Sprintf("%d%%", pctWk)
		if quotaDetails.CompactResetWeekly != "" {
			qWkText = fmt.Sprintf("%d%% (%s)", pctWk, quotaDetails.CompactResetWeekly)
		}
		qWkStr := fmt.Sprintf("Week: %s", qWkText)
		if useColor {
			colorCode := "\033[35m"
			if pctWk < 5 {
				colorCode = "\033[1;31m"
			} else if pctWk < 20 {
				colorCode = "\033[33m"
			}
			qWkStr = fmt.Sprintf("\033[90mWeek:\033[0m %s%s\033[0m", colorCode, qWkText)
		}
		line2Parts = append(line2Parts, qWkStr)
	}

	// 9. Generation Speed (tok/s telemetry)
	if len(telemetry) > 0 && telemetry[0].Speed > 0 {
		if spdStr := FormatTokenTelemetry(telemetry[0], useColor); spdStr != "" {
			line2Parts = append(line2Parts, spdStr)
		}
	}


	var lines []string
	if len(line1Parts) > 0 {
		lines = append(lines, strings.Join(line1Parts, sep))
	}
	if len(line2Parts) > 0 {
		lines = append(lines, strings.Join(line2Parts, sep))
	}

	return strings.Join(lines, "\n")
}

var (
	gitBranchCacheMu sync.RWMutex
	gitBranchCache   = make(map[string]gitBranchCacheEntry)
)

type gitBranchCacheEntry struct {
	branch    string
	expiresAt time.Time
}

func findGitBranch(dir string) string {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if dir == "" {
		return ""
	}

	gitBranchCacheMu.RLock()
	if entry, ok := gitBranchCache[dir]; ok && time.Now().Before(entry.expiresAt) {
		gitBranchCacheMu.RUnlock()
		return entry.branch
	}
	gitBranchCacheMu.RUnlock()

	branch := resolveGitBranch(dir)

	gitBranchCacheMu.Lock()
	gitBranchCache[dir] = gitBranchCacheEntry{
		branch:    branch,
		expiresAt: time.Now().Add(3 * time.Second),
	}
	gitBranchCacheMu.Unlock()

	return branch
}

func resolveGitBranch(dir string) string {
	for range 6 {
		gitPath := filepath.Join(dir, ".git")
		fi, err := os.Stat(gitPath)
		if err == nil {
			if fi.IsDir() {
				headBytes, err := os.ReadFile(filepath.Join(gitPath, "HEAD"))
				if err == nil {
					return parseGitHead(string(headBytes))
				}
			} else {
				gitFile, err := os.ReadFile(gitPath)
				if err == nil {
					for _, line := range strings.Split(string(gitFile), "\n") {
						line = strings.TrimSpace(line)
						if strings.HasPrefix(line, "gitdir:") {
							gitDir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
							if !filepath.IsAbs(gitDir) {
								gitDir = filepath.Join(dir, gitDir)
							}
							headBytes, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
							if err == nil {
								return parseGitHead(string(headBytes))
							}
						}
					}
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func isSessionContextStateEqual(a, b *SessionContextState) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.UsedPercentage == b.UsedPercentage &&
		a.InputTokens == b.InputTokens &&
		a.OutputTokens == b.OutputTokens &&
		a.CacheReadTokens == b.CacheReadTokens &&
		a.CacheCreationTokens == b.CacheCreationTokens &&
		a.DurationSeconds == b.DurationSeconds &&
		a.Speed == b.Speed &&
		a.ModelID == b.ModelID &&
		a.ModelDisplayName == b.ModelDisplayName &&
		a.ConversationTitle == b.ConversationTitle &&
		a.ConversationID == b.ConversationID &&
		a.Cost == b.Cost &&
		a.Effort == b.Effort
}

func parseGitHead(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/")
	}
	if len(content) >= 7 {
		return content[:7]
	}
	return content
}

func hasChainedStatusLine(profileDir string) bool {
	if profileDir == "" {
		return false
	}
	backupPath := filepath.Join(profileDir, ".gemini", "config", statuslineBackupFile)
	data, err := os.ReadFile(backupPath)
	if err != nil || len(data) == 0 {
		return false
	}
	var original struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	if err := json.Unmarshal(data, &original); err != nil {
		return false
	}
	cmd := strings.TrimSpace(original.Command)
	if cmd == "" {
		return false
	}
	return !strings.Contains(cmd, "statusline-hook")
}

func chainPreviousStatusLine(ctx context.Context, profileDir string, input []byte, stdout, stderr io.Writer) {
	backupPath := filepath.Join(profileDir, ".gemini", "config", statuslineBackupFile)
	data, err := os.ReadFile(backupPath)
	if err != nil || len(data) == 0 {
		return
	}

	var original struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	if err := json.Unmarshal(data, &original); err != nil || original.Command == "" {
		return
	}

	// Avoid infinite recursion if command points to agyp statusline-hook
	if strings.Contains(original.Command, "statusline-hook") {
		return
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, "cmd.exe", "/c", original.Command)
	} else {
		cmd = exec.CommandContext(cmdCtx, "sh", "-c", original.Command)
	}
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	_ = cmd.Run()
}

// SyncStatusLineSettings configures the "statusLine" entry in settings.json to call agyp statusline-hook,
// preserving any pre-existing custom statusLine command in statusline.original.json.
func SyncStatusLineSettings(profileDir string) error {
	cliPath := filepath.Join(profileDir, ".gemini", "antigravity-cli", "settings.json")
	candidatePaths := []string{
		cliPath,
		filepath.Join(profileDir, ".gemini", "antigravity", "settings.json"),
		filepath.Join(profileDir, ".gemini", "antigravity-ide", "settings.json"),
	}

	hookCommand := "agyp statusline-hook"
	for _, sPath := range candidatePaths {
		// Only auto-create directory for CLI settings; for GUI/IDE only update if already initialized
		if sPath != cliPath {
			if _, err := os.Stat(sPath); os.IsNotExist(err) {
				continue
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(sPath), 0700); err != nil {
				continue
			}
		}

		var settings map[string]interface{}
		data, err := os.ReadFile(sPath)
		if err == nil {
			_ = json.Unmarshal(data, &settings)
		}
		if settings == nil {
			settings = make(map[string]interface{})
		}

		// Check if already installed
		if sl, ok := settings["statusLine"].(map[string]interface{}); ok {
			if cmdStr, ok := sl["command"].(string); ok && strings.Contains(cmdStr, "statusline-hook") {
				continue
			}
			// Backup original statusLine if not already backed up
			backupPath := filepath.Join(profileDir, ".gemini", "config", statuslineBackupFile)
			if _, err := os.Stat(backupPath); os.IsNotExist(err) {
				_ = os.MkdirAll(filepath.Dir(backupPath), 0700)
				bData, _ := json.MarshalIndent(sl, "", "  ")
				_ = WriteFileAtomic(backupPath, bData, 0600)
			}
		}

		settings["statusLine"] = map[string]interface{}{
			"type":    "command",
			"command": hookCommand,
		}

		out, err := json.MarshalIndent(settings, "", "  ")
		if err == nil {
			_ = WriteFileAtomic(sPath, []byte(string(out)+"\n"), 0600)
		}
	}

	return nil
}

// ResolveConversationTitle attempts to find a meaningful conversation title from:
// 1. Brain transcript.jsonl by conversation ID
// 2. Profile history.jsonl by conversation ID
func ResolveConversationTitle(profileDir, convID string) string {
	if profileDir == "" || convID == "" {
		return ""
	}

	// 1. Check transcript.jsonl if convID is provided
	for _, subDir := range []string{"antigravity-cli", "antigravity", "antigravity-ide"} {
		tPath := filepath.Join(profileDir, ".gemini", subDir, "brain", convID, ".system_generated", "logs", "transcript.jsonl")
		if title := ResolveConversationTitleFromTranscript(tPath); title != "" {
			return title
		}
	}

	// 2. Check history.jsonl by conversation ID
	for _, subDir := range []string{"antigravity-cli", "antigravity", "antigravity-ide"} {
		hPath := filepath.Join(profileDir, ".gemini", subDir, "history.jsonl")
		f, err := os.Open(hPath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(f)
		buf := make([]byte, 128*1024)
		scanner.Buffer(buf, 1024*1024)

		var matchingTitles []string
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 || !bytes.Contains(line, []byte("display")) {
				continue
			}

			var item struct {
				Display        string `json:"display"`
				ConversationID string `json:"conversationId"`
			}
			if err := json.Unmarshal(line, &item); err == nil && item.ConversationID == convID && item.Display != "" {
				disp := strings.TrimSpace(item.Display)
				if !strings.HasPrefix(disp, "/") && !strings.HasPrefix(disp, "[AGYP_INTERNAL_") {
					cleaned := cleanPromptSummary(disp)
					if cleaned != "" && cleaned != "(No prompt summary)" && !strings.HasPrefix(cleaned, "/") {
						matchingTitles = append(matchingTitles, cleaned)
					}
				}
			}
		}
		_ = f.Close()

		if len(matchingTitles) > 0 {
			return matchingTitles[0]
		}
	}

	return ""
}

// ResolveConversationTitleFromTranscript extracts the initial user prompt from transcript.jsonl.
func ResolveConversationTitleFromTranscript(transcriptPath string) string {
	if transcriptPath == "" {
		return ""
	}
	f, err := os.Open(transcriptPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	lineCount := 0
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			break
		}
		lineCount++
		if bytes.Contains(line, []byte("<USER_REQUEST>")) || bytes.Contains(line, []byte(`"USER_INPUT"`)) {
			var data struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			}
			if json.Unmarshal(line, &data) == nil && data.Content != "" {
				if data.Type != "" && data.Type != "USER_INPUT" {
					continue
				}
				prompt := data.Content
				match := userRequestRegex.FindStringSubmatch(data.Content)
				if len(match) > 1 {
					prompt = match[1]
				}
				prompt = strings.TrimSpace(prompt)
				if prompt != "" && !strings.HasPrefix(prompt, "[AGYP_INTERNAL_") {
					cleaned := cleanPromptSummary(prompt)
					if cleaned != "" && cleaned != "(No prompt summary)" && !strings.HasPrefix(cleaned, "/") {
						return cleaned
					}
				}
			}
		}
		if lineCount > 100 || err != nil {
			break
		}
	}
	return ""
}
