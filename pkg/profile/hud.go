package profile

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// EmoteState represents the agent's current activity state for avatar selection.
type EmoteState string

const (
	EmoteIdle    EmoteState = "idle"
	EmoteThink   EmoteState = "think"
	EmoteTool    EmoteState = "tool"
	EmoteTalk    EmoteState = "talk"
	EmoteSuccess EmoteState = "success"
	EmoteFailure EmoteState = "failure"
	EmoteCompact EmoteState = "compact"
	EmoteHi      EmoteState = "hi"
)

// TerminalProtocol indicates graphics capability.
type TerminalProtocol int

const (
	ProtocolASCII TerminalProtocol = iota
	ProtocolKitty
)

var (
	imageCacheMu sync.RWMutex
	imageCache   = make(map[string][]byte)
)

// DetectTerminalProtocol checks whether the current terminal supports Kitty graphics (Ghostty) or ASCII (Zed/fallback).
func DetectTerminalProtocol() TerminalProtocol {
	override := strings.ToLower(strings.TrimSpace(os.Getenv("AGYP_RENDER")))
	if override == "" {
		override = strings.ToLower(strings.TrimSpace(os.Getenv("AGYP_EMOTE_RENDER")))
	}
	if override == "kitty" || override == "image" {
		return ProtocolKitty
	}
	if override == "ascii" || override == "braille" || override == "text" {
		return ProtocolASCII
	}

	termProgram := strings.ToLower(strings.TrimSpace(os.Getenv("TERM_PROGRAM")))
	if termProgram == "ghostty" || os.Getenv("GHOSTTY_RESOURCES_DIR") != "" {
		return ProtocolKitty
	}

	// Zed and other default terminals fall back to Braille ASCII
	return ProtocolASCII
}

// MapAgentStateToEmote resolves the agent state string to an EmoteState.
func MapAgentStateToEmote(state string) EmoteState {
	raw := strings.ToLower(strings.TrimSpace(state))
	switch raw {
	case "think", "thinking", "reasoning":
		return EmoteThink
	case "tool", "busy", "active", "running", "working", "generating", "read", "write", "edit", "bash":
		return EmoteTool
	case "talk", "streaming", "output":
		return EmoteTalk
	case "done", "success", "finished", "completed":
		return EmoteSuccess
	case "failure", "failed", "error":
		return EmoteFailure
	case "compact", "memory":
		return EmoteCompact
	case "hi", "start", "init":
		return EmoteHi
	default:
		return EmoteIdle
	}
}

// GetEmoteSearchDirs returns directories where emote assets (base_stickers) are located.
func GetEmoteSearchDirs() []string {
	var dirs []string

	if customDir := os.Getenv("AGYP_EMOTES_DIR"); customDir != "" {
		dirs = append(dirs, customDir)
	}

	realHome, err := GetRealUserHome()
	if err == nil && realHome != "" {
		// 1. ~/.agyp/emotes/base_stickers
		dirs = append(dirs, filepath.Join(realHome, ".agyp", "emotes", "base_stickers"))
		// 2. ~/.omp/custom-plugins/pi-emote/emotes/base_stickers
		dirs = append(dirs, filepath.Join(realHome, ".omp", "custom-plugins", "pi-emote", "emotes", "base_stickers"))
		// 3. ~/.omp/plugins/node_modules/pi-emote/emotes/base_stickers
		dirs = append(dirs, filepath.Join(realHome, ".omp", "plugins", "node_modules", "pi-emote", "emotes", "base_stickers"))
	}

	if curHome := os.Getenv("HOME"); curHome != "" && curHome != realHome {
		dirs = append(dirs, filepath.Join(curHome, ".omp", "custom-plugins", "pi-emote", "emotes", "base_stickers"))
	}

	return dirs
}

// LoadEmotePNG loads the PNG bytes for a given emote state from base_stickers.
func LoadEmotePNG(state EmoteState) ([]byte, error) {
	cacheKey := string(state)
	imageCacheMu.RLock()
	if data, ok := imageCache[cacheKey]; ok {
		imageCacheMu.RUnlock()
		return data, nil
	}
	imageCacheMu.RUnlock()

	var fileCandidates []string
	switch state {
	case EmoteThink:
		fileCandidates = []string{"think/think.png"}
	case EmoteTool:
		fileCandidates = []string{"tool/tool_1.png", "write/write_1.png"}
	case EmoteTalk:
		fileCandidates = []string{"talk/talk_close.png", "talk/talk_mid.png"}
	case EmoteSuccess:
		fileCandidates = []string{"success/success_1.png"}
	case EmoteFailure:
		fileCandidates = []string{"failure/failure_1.png"}
	case EmoteCompact:
		fileCandidates = []string{"compact/compact_1.png"}
	case EmoteHi:
		fileCandidates = []string{"hi/hi_1.png"}
	default:
		fileCandidates = []string{"idle/idle.png", "idle/idle_blink.png"}
	}

	searchDirs := GetEmoteSearchDirs()
	for _, baseDir := range searchDirs {
		for _, rel := range fileCandidates {
			fullPath := filepath.Join(baseDir, rel)
			if data, err := os.ReadFile(fullPath); err == nil && len(data) > 0 {
				imageCacheMu.Lock()
				imageCache[cacheKey] = data
				imageCacheMu.Unlock()
				return data, nil
			}
		}
	}

	return nil, fmt.Errorf("no png emote found for state %s", state)
}

const (
	kittyChunkSize = 4096
	kittyImageID   = 1337
)

// BuildKittyImageSequence builds Kitty graphics protocol escape sequence with chunking.
func BuildKittyImageSequence(pngBytes []byte, cols, rows int) string {
	b64 := base64.StdEncoding.EncodeToString(pngBytes)
	paramStr := fmt.Sprintf("a=T,f=100,q=2,C=1,c=%d,r=%d,i=%d", cols, rows, kittyImageID)
	delSeq := fmt.Sprintf("\033_Ga=d,d=i,i=%d,q=2\033\\", kittyImageID)

	if len(b64) <= kittyChunkSize {
		return delSeq + fmt.Sprintf("\033_G%s;%s\033\\", paramStr, b64)
	}

	var sb strings.Builder
	sb.WriteString(delSeq)
	offset := 0
	isFirst := true
	for offset < len(b64) {
		end := offset + kittyChunkSize
		isLast := end >= len(b64)
		if isLast {
			end = len(b64)
		}
		chunk := b64[offset:end]

		if isFirst {
			sb.WriteString(fmt.Sprintf("\033_G%s,m=1;%s\033\\", paramStr, chunk))
			isFirst = false
		} else if isLast {
			sb.WriteString(fmt.Sprintf("\033_Gm=0;%s\033\\", chunk))
		} else {
			sb.WriteString(fmt.Sprintf("\033_Gm=1;%s\033\\", chunk))
		}
		offset += kittyChunkSize
	}

	return sb.String()
}

// GetBrailleArtLines returns the 5-row, 24-character Braille art anime avatar for Zed/fallback.
func GetBrailleArtLines(useColor bool) []string {
	rawLines := []string{
		"⠀⠀⠀⠀⡤⡀⢈⢻⣬⣿⠟⢁⣤⣶⣿⣿⡿⠿⠿⠛⠛⢀⣄⠀",
		"⠀⠀⢢⣘⣿⣿⣶⣿⣯⣤⣾⣿⣿⣿⠟⠁⠄⠀⣾⡇⣼⢻⣿⣾",
		"⣰⠞⠛⢉⣩⣿⣿⣿⣿⣿⣿⣿⣿⠋⣼⣧⣤⣴⠟⣠⣿⢰⣿⣿",
		"⣶⡾⠿⠿⠿⢿⣿⣿⣿⣿⣿⣿⣿⣈⣩⣤⡶⠟⢛⣩⣴⣿⣿⡟",
		"⣠⣄⠈⠀⣰⡦⠙⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣟⡛⠛⠛⠁",
	}

	if !useColor {
		return rawLines
	}

	colored := make([]string, len(rawLines))
	for i, l := range rawLines {
		colored[i] = fmt.Sprintf("\033[36m%s\033[0m", l)
	}
	return colored
}

// formatCwdCompact formats the current working directory cleanly, replacing realHome with ~ and truncating if long.
func formatCwdCompact(dir string, maxLen int) string {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	realHome, err := GetRealUserHome()
	if err == nil && realHome != "" && strings.HasPrefix(dir, realHome) {
		dir = "~" + strings.TrimPrefix(dir, realHome)
	} else if curHome := os.Getenv("HOME"); curHome != "" && strings.HasPrefix(dir, curHome) {
		dir = "~" + strings.TrimPrefix(dir, curHome)
	}

	if len(dir) <= maxLen {
		return dir
	}

	parts := strings.Split(strings.Trim(dir, "/"), "/")
	if len(parts) <= 2 {
		return dir
	}

	last := parts[len(parts)-1]
	secondLast := parts[len(parts)-2]
	prefix := ""
	if strings.HasPrefix(dir, "~") {
		prefix = "~/"
	}

	cand := fmt.Sprintf("%s…/%s/%s", prefix, secondLast, last)
	if len(cand) <= maxLen {
		return cand
	}

	return fmt.Sprintf("%s…/%s", prefix, last)
}

// BuildHUDLines generates the 5 right-hand lines of telemetry with the exact color scheme from agyp.
func BuildHUDLines(profileName, workspaceName, gitBranch, agentState, modelName, effort string, cost float64, ctxPct int, hasCtx bool, quotaDetails *ModelQuotaDetails, useColor bool, telemetry ...TokenTelemetry) []string {
	sep := " · "
	if useColor {
		sep = "\033[90m · \033[0m"
	}

	// --- Line 1: Profile & Dynamic Status Badge ---
	pStr := profileName
	if pStr == "" {
		pStr = "agy"
	}
	if useColor {
		pStr = fmt.Sprintf("\033[1;36m[%s]\033[0m", pStr)
	} else {
		pStr = fmt.Sprintf("[%s]", pStr)
	}

	dashSep := " ── "
	if useColor {
		dashSep = "\033[90m ── \033[0m"
	}

	rawState := strings.ToLower(strings.TrimSpace(agentState))
	var statusBadge string
	switch rawState {
	case "think", "thinking", "reasoning":
		if useColor {
			statusBadge = "\033[1;36m✦ THINKING\033[0m \033[90m[ reasoning ]\033[0m"
		} else {
			statusBadge = "✦ THINKING [ reasoning ]"
		}
	case "busy", "active", "running", "working", "generating", "tool", "read", "write", "edit", "bash":
		detail := rawState
		if detail == "busy" || detail == "active" || detail == "running" || detail == "generating" {
			detail = "working"
		}
		if useColor {
			statusBadge = fmt.Sprintf("\033[1;33m⚙ WORKING\033[0m \033[90m[ %s ]\033[0m", detail)
		} else {
			statusBadge = fmt.Sprintf("⚙ WORKING [ %s ]", detail)
		}
	case "talk", "streaming", "output":
		if useColor {
			statusBadge = "\033[1;36m💬 STREAMING\033[0m \033[90m[ output ]\033[0m"
		} else {
			statusBadge = "💬 STREAMING [ output ]"
		}
	case "done", "success", "finished", "completed":
		if useColor {
			statusBadge = "\033[1;32m✔ SUCCESS\033[0m \033[90m[ done ]\033[0m"
		} else {
			statusBadge = "✔ SUCCESS [ done ]"
		}
	case "failure", "failed", "error":
		if useColor {
			statusBadge = "\033[1;31m✖ ERROR\033[0m \033[90m[ failed ]\033[0m"
		} else {
			statusBadge = "✖ ERROR [ failed ]"
		}
	case "waiting", "paused", "wait", "user_input", "waiting_for_input":
		if useColor {
			statusBadge = "\033[1;33m⏳ WAITING\033[0m \033[90m[ input ]\033[0m"
		} else {
			statusBadge = "⏳ WAITING [ input ]"
		}
	case "compact", "memory":
		if useColor {
			statusBadge = "\033[1;90m◆ COMPACT\033[0m \033[90m[ memory ]\033[0m"
		} else {
			statusBadge = "◆ COMPACT [ memory ]"
		}
	default:
		if useColor {
			statusBadge = "\033[1;32m● ONLINE\033[0m \033[90m[ idle ]\033[0m"
		} else {
			statusBadge = "● ONLINE [ idle ]"
		}
	}
	line1 := pStr + dashSep + statusBadge

	// --- Line 2: Robot Icon + Model Name & Effort ---
	mName := modelName
	if mName == "" {
		mName = "Gemini 3.8 Flash"
	}
	var line2 string
	botIcon := "🤖 "
	if effort != "" {
		if useColor {
			line2 = fmt.Sprintf("%s\033[94m%s\033[0m \033[36m(%s)\033[0m", botIcon, mName, effort)
		} else {
			line2 = fmt.Sprintf("%s%s (%s)", botIcon, mName, effort)
		}
	} else {
		if useColor {
			line2 = fmt.Sprintf("%s\033[94m%s\033[0m", botIcon, mName)
		} else {
			line2 = fmt.Sprintf("%s%s", botIcon, mName)
		}
	}

	// --- Line 3: Git Branch & Directory ---
	var line3Parts []string
	if gitBranch != "" {
		if useColor {
			line3Parts = append(line3Parts, fmt.Sprintf("\033[35m %s\033[0m", gitBranch))
		} else {
			line3Parts = append(line3Parts, fmt.Sprintf(" %s", gitBranch))
		}
	}

	cwd, _ := os.Getwd()
	formattedPath := formatCwdCompact(cwd, 32)
	if workspaceName != "" && formattedPath == "" {
		formattedPath = workspaceName
	}
	if formattedPath != "" {
		if useColor {
			line3Parts = append(line3Parts, fmt.Sprintf("📁 \033[33m%s\033[0m", formattedPath))
		} else {
			line3Parts = append(line3Parts, fmt.Sprintf("📁 %s", formattedPath))
		}
	}
	line3 := strings.Join(line3Parts, sep)

	// --- Line 4: Rate Limits (5h & 7d) ---
	clockIcon := "⏱ "
	var line4Parts []string
	if quotaDetails != nil && quotaDetails.Fraction5H >= 0 {
		pct5h := int(quotaDetails.Fraction5H*100 + 0.5)
		resetStr := quotaDetails.CompactReset5H
		if resetStr == "" {
			resetStr = "ready"
		}
		if useColor {
			c5 := "\033[32m"
			if pct5h < 5 {
				c5 = "\033[1;31m"
			} else if pct5h < 20 {
				c5 = "\033[33m"
			}
			line4Parts = append(line4Parts, fmt.Sprintf("\033[90m5h\033[0m %s%d%%\033[0m \033[90m(%s)\033[0m", c5, pct5h, resetStr))
		} else {
			line4Parts = append(line4Parts, fmt.Sprintf("5h %d%% (%s)", pct5h, resetStr))
		}
	} else {
		if useColor {
			line4Parts = append(line4Parts, "\033[90m5h -\033[0m")
		} else {
			line4Parts = append(line4Parts, "5h -")
		}
	}

	if quotaDetails != nil && quotaDetails.FractionWeekly >= 0 {
		pctWk := int(quotaDetails.FractionWeekly*100 + 0.5)
		resetWk := quotaDetails.CompactResetWeekly
		if resetWk == "" {
			resetWk = "ready"
		}
		if useColor {
			cW := "\033[35m"
			if pctWk < 5 {
				cW = "\033[1;31m"
			} else if pctWk < 20 {
				cW = "\033[33m"
			}
			line4Parts = append(line4Parts, fmt.Sprintf("\033[90m7d\033[0m %s%d%%\033[0m \033[90m(%s)\033[0m", cW, pctWk, resetWk))
		} else {
			line4Parts = append(line4Parts, fmt.Sprintf("7d %d%% (%s)", pctWk, resetWk))
		}
	} else {
		if useColor {
			line4Parts = append(line4Parts, "\033[90m7d -\033[0m")
		} else {
			line4Parts = append(line4Parts, "7d -")
		}
	}
	line4 := clockIcon + strings.Join(line4Parts, sep)

	// --- Line 5: Context & Generation Speed ---
	cpuIcon := "⚙ "
	var line5Parts []string
	if hasCtx {
		if useColor {
			cCtx := "\033[36m"
			if ctxPct >= 80 {
				cCtx = "\033[1;31m"
			} else if ctxPct >= 50 {
				cCtx = "\033[33m"
			}
			line5Parts = append(line5Parts, fmt.Sprintf("%s%s%d%% ctx\033[0m", cpuIcon, cCtx, ctxPct))
		} else {
			line5Parts = append(line5Parts, fmt.Sprintf("%s%d%% ctx", cpuIcon, ctxPct))
		}
	} else {
		if useColor {
			line5Parts = append(line5Parts, fmt.Sprintf("%s\033[36m0%% ctx\033[0m", cpuIcon))
		} else {
			line5Parts = append(line5Parts, fmt.Sprintf("%s0%% ctx", cpuIcon))
		}
	}

	speedVal := 0.0
	if len(telemetry) > 0 && telemetry[0].Speed > 0 {
		speedVal = telemetry[0].Speed
	}
	lightningIcon := "⚡ "
	if speedVal > 0 {
		if useColor {
			line5Parts = append(line5Parts, fmt.Sprintf("%s\033[33m%.1f/s\033[0m", lightningIcon, speedVal))
		} else {
			line5Parts = append(line5Parts, fmt.Sprintf("%s%.1f/s", lightningIcon, speedVal))
		}
	} else {
		if useColor {
			line5Parts = append(line5Parts, fmt.Sprintf("%s\033[90m-\033[0m", lightningIcon))
		} else {
			line5Parts = append(line5Parts, fmt.Sprintf("%s-", lightningIcon))
		}
	}
	line5 := strings.Join(line5Parts, sep)

	return []string{line1, line2, line3, line4, line5}
}

// FormatStatusLineHUD renders the avatar (Kitty graphics in Ghostty or Braille in Zed) beside the 5-row HUD.
func FormatStatusLineHUD(profileName, workspaceName, gitBranch, agentState, modelName, effort string, cost float64, ctxPct int, hasCtx bool, quotaDetails *ModelQuotaDetails, useColor bool, telemetry ...TokenTelemetry) string {
	proto := DetectTerminalProtocol()
	emoteState := MapAgentStateToEmote(agentState)
	hudLines := BuildHUDLines(profileName, workspaceName, gitBranch, agentState, modelName, effort, cost, ctxPct, hasCtx, quotaDetails, useColor, telemetry...)

	// 1. Try Kitty graphics protocol if Ghostty is detected
	if proto == ProtocolKitty {
		pngBytes, err := LoadEmotePNG(emoteState)
		if err == nil && len(pngBytes) > 0 {
			imageCols := 10
			imageRows := 5
			kittySeq := BuildKittyImageSequence(pngBytes, imageCols, imageRows)
			avatarPad := strings.Repeat(" ", imageCols)
			spacing := "  "

			var outLines []string
			for i := 0; i < imageRows; i++ {
				info := ""
				if i < len(hudLines) {
					info = hudLines[i]
				}
				if i == 0 {
					outLines = append(outLines, kittySeq+avatarPad+spacing+info)
				} else {
					outLines = append(outLines, avatarPad+spacing+info)
				}
			}
			return strings.Join(outLines, "\n")
		}
	}

	// 2. Zed / Fallback: 24x5 Braille ASCII Art
	brailleLines := GetBrailleArtLines(useColor)
	spacing := "  "
	var outLines []string
	rowCount := len(brailleLines)
	if len(hudLines) > rowCount {
		rowCount = len(hudLines)
	}

	for i := 0; i < rowCount; i++ {
		bLine := ""
		if i < len(brailleLines) {
			bLine = brailleLines[i]
		} else {
			bLine = strings.Repeat(" ", 24)
		}

		hLine := ""
		if i < len(hudLines) {
			hLine = hudLines[i]
		}
		outLines = append(outLines, bLine+spacing+hLine)
	}

	return strings.Join(outLines, "\n")
}
