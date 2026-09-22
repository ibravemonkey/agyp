package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	tokenStatsFilename = "token_stats.json"
)

// ProfileTokens stores input, output, and cache token metrics for a profile.
type ProfileTokens struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	CacheTokens  int64 `json:"cache_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}

// ModelTokens stores input, output, and cache token metrics for a specific model.
type ModelTokens struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	CacheTokens  int64 `json:"cache_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}

// TokenUsageDay records token consumption aggregated per day (YYYY-MM-DD).
type TokenUsageDay struct {
	Date         string                    `json:"date"`
	InputTokens  int64                     `json:"input_tokens"`
	OutputTokens int64                     `json:"output_tokens"`
	CacheTokens  int64                     `json:"cache_tokens"`
	TotalTokens  int64                     `json:"total_tokens"`
	Profiles     map[string]*ProfileTokens `json:"profiles,omitempty"`
	Models       map[string]*ModelTokens   `json:"models,omitempty"`
}

// ConvTokenSnapshot stores the last seen token metrics for an ongoing conversation.
type ConvTokenSnapshot struct {
	Profile     string `json:"profile"`
	Model       string `json:"model"`
	LastInput   int64  `json:"last_input"`
	LastOutput  int64  `json:"last_output"`
	LastCache   int64  `json:"last_cache"`
	LastUpdated string `json:"last_updated"`
}

// TokenStatsStore represents the on-disk persistent token usage database.
type TokenStatsStore struct {
	Version       int                          `json:"version"`
	Daily         map[string]*TokenUsageDay    `json:"daily"`
	Conversations map[string]ConvTokenSnapshot `json:"conversations"`
	UpdatedAt     time.Time                    `json:"updated_at"`
}

var statsMu sync.Mutex

// GetTokenStatsPath returns the path to ~/.agyp/token_stats.json.
func GetTokenStatsPath() (string, error) {
	agypDir, err := GetAgypDir()
	if err != nil {
		home, hErr := os.UserHomeDir()
		if hErr != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		agypDir = filepath.Join(home, ".agyp")
	}
	return filepath.Join(agypDir, tokenStatsFilename), nil
}

// LoadTokenStats loads the stats store from disk or returns an initialized empty store.
func LoadTokenStats() (*TokenStatsStore, error) {
	statsPath, err := GetTokenStatsPath()
	if err != nil {
		return &TokenStatsStore{
			Version:       1,
			Daily:         make(map[string]*TokenUsageDay),
			Conversations: make(map[string]ConvTokenSnapshot),
			UpdatedAt:     time.Now(),
		}, nil
	}

	data, err := os.ReadFile(statsPath)
	if err != nil {
		store := &TokenStatsStore{
			Version:       1,
			Daily:         make(map[string]*TokenUsageDay),
			Conversations: make(map[string]ConvTokenSnapshot),
			UpdatedAt:     time.Now(),
		}
		// Try to seed from existing session contexts on fresh install
		_ = SeedTokenStatsFromProfiles(store)
		return store, nil
	}

	var store TokenStatsStore
	if err := json.Unmarshal(data, &store); err != nil {
		return &TokenStatsStore{
			Version:       1,
			Daily:         make(map[string]*TokenUsageDay),
			Conversations: make(map[string]ConvTokenSnapshot),
			UpdatedAt:     time.Now(),
		}, nil
	}

	if store.Daily == nil {
		store.Daily = make(map[string]*TokenUsageDay)
	}
	if store.Conversations == nil {
		store.Conversations = make(map[string]ConvTokenSnapshot)
	}

	return &store, nil
}

// SaveTokenStats persists the stats store to disk atomically.
func SaveTokenStats(store *TokenStatsStore) error {
	if store == nil {
		return nil
	}
	statsPath, err := GetTokenStatsPath()
	if err != nil {
		return err
	}

	store.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomicNoSync(statsPath, append(data, '\n'), 0600)
}
// ResetTokenStats clears the token stats store and saves an empty database.
func ResetTokenStats() error {
	statsMu.Lock()
	defer statsMu.Unlock()

	emptyStore := &TokenStatsStore{
		Version:       1,
		Daily:         make(map[string]*TokenUsageDay),
		Conversations: make(map[string]ConvTokenSnapshot),
		UpdatedAt:     time.Now(),
	}
	return SaveTokenStats(emptyStore)
}

// RecordTokenUsage records incremental token consumption for a profile, model, and conversation.
func RecordTokenUsage(profileName, modelName, convID string, inputTokens, outputTokens, cacheTokens int64) error {
	if profileName == "" || (inputTokens <= 0 && outputTokens <= 0 && cacheTokens <= 0) {
		return nil
	}

	statsMu.Lock()
	defer statsMu.Unlock()

	store, err := LoadTokenStats()
	if err != nil {
		return err
	}

	now := time.Now()
	today := now.Format("2006-01-02")

	cleanModel := NormalizeModelName(modelName)
	if cleanModel == "" {
		cleanModel = "gemini-flash"
	}

	// Calculate incremental deltas
	var deltaInput, deltaOutput, deltaCache int64

	if convID != "" {
		if snap, exists := store.Conversations[convID]; exists {
			if inputTokens > snap.LastInput {
				deltaInput = inputTokens - snap.LastInput
			}
			if outputTokens > snap.LastOutput {
				deltaOutput = outputTokens - snap.LastOutput
			}
			if cacheTokens > snap.LastCache {
				deltaCache = cacheTokens - snap.LastCache
			}

			// If no new tokens were added, skip writing to disk
			if deltaInput == 0 && deltaOutput == 0 && deltaCache == 0 {
				return nil
			}

			// Update snapshot with highest seen values
			snap.LastInput = max(snap.LastInput, inputTokens)
			snap.LastOutput = max(snap.LastOutput, outputTokens)
			snap.LastCache = max(snap.LastCache, cacheTokens)
			snap.LastUpdated = today
			snap.Model = cleanModel
			snap.Profile = profileName
			store.Conversations[convID] = snap
		} else {
			deltaInput = inputTokens
			deltaOutput = outputTokens
			deltaCache = cacheTokens
			store.Conversations[convID] = ConvTokenSnapshot{
				Profile:     profileName,
				Model:       cleanModel,
				LastInput:   inputTokens,
				LastOutput:  outputTokens,
				LastCache:   cacheTokens,
				LastUpdated: today,
			}
		}
	} else {
		deltaInput = inputTokens
		deltaOutput = outputTokens
		deltaCache = cacheTokens
	}

	deltaTotal := deltaInput + deltaOutput + deltaCache
	if deltaTotal <= 0 {
		return nil
	}

	// Update daily aggregation
	day, exists := store.Daily[today]
	if !exists || day == nil {
		day = &TokenUsageDay{
			Date:     today,
			Profiles: make(map[string]*ProfileTokens),
			Models:   make(map[string]*ModelTokens),
		}
		store.Daily[today] = day
	}

	day.InputTokens += deltaInput
	day.OutputTokens += deltaOutput
	day.CacheTokens += deltaCache
	day.TotalTokens += deltaTotal

	// Update profile aggregation
	if day.Profiles == nil {
		day.Profiles = make(map[string]*ProfileTokens)
	}
	pTokens, pExists := day.Profiles[profileName]
	if !pExists || pTokens == nil {
		pTokens = &ProfileTokens{}
		day.Profiles[profileName] = pTokens
	}
	pTokens.InputTokens += deltaInput
	pTokens.OutputTokens += deltaOutput
	pTokens.CacheTokens += deltaCache
	pTokens.TotalTokens += deltaTotal

	// Update model aggregation
	if day.Models == nil {
		day.Models = make(map[string]*ModelTokens)
	}
	mTokens, mExists := day.Models[cleanModel]
	if !mExists || mTokens == nil {
		mTokens = &ModelTokens{}
		day.Models[cleanModel] = mTokens
	}
	mTokens.InputTokens += deltaInput
	mTokens.OutputTokens += deltaOutput
	mTokens.CacheTokens += deltaCache
	mTokens.TotalTokens += deltaTotal

	return SaveTokenStats(store)
}

// SeedTokenStatsFromProfiles scans all profiles and initialises the store with currently active session context states.
func SeedTokenStatsFromProfiles(store *TokenStatsStore) bool {
	if store == nil {
		return false
	}
	profiles, err := List()
	if err != nil || len(profiles) == 0 {
		return false
	}

	seededAny := false
	for _, p := range profiles {
		pDir, pErr := GetProfileDir(p)
		if pErr != nil {
			continue
		}

		// Check primary session context
		if state, ok := GetSessionContextState(pDir); ok && state != nil {
			if state.InputTokens > 0 || state.OutputTokens > 0 || state.CacheReadTokens > 0 {
				convID := state.ConversationID
				if convID == "" {
					convID = fmt.Sprintf("seed_%s", p)
				}
				today := state.UpdatedAt.Format("2006-01-02")
				if today == "" || state.UpdatedAt.IsZero() {
					today = time.Now().Format("2006-01-02")
				}
				cleanModel := NormalizeModelName(state.ModelID)
				if cleanModel == "" {
					cleanModel = "gemini-flash"
				}

				total := state.InputTokens + state.OutputTokens + state.CacheReadTokens
				store.Conversations[convID] = ConvTokenSnapshot{
					Profile:     p,
					Model:       cleanModel,
					LastInput:   state.InputTokens,
					LastOutput:  state.OutputTokens,
					LastCache:   state.CacheReadTokens,
					LastUpdated: today,
				}

				day, exists := store.Daily[today]
				if !exists || day == nil {
					day = &TokenUsageDay{
						Date:     today,
						Profiles: make(map[string]*ProfileTokens),
						Models:   make(map[string]*ModelTokens),
					}
					store.Daily[today] = day
				}
				day.InputTokens += state.InputTokens
				day.OutputTokens += state.OutputTokens
				day.CacheTokens += state.CacheReadTokens
				day.TotalTokens += total

				pTok := day.Profiles[p]
				if pTok == nil {
					pTok = &ProfileTokens{}
					day.Profiles[p] = pTok
				}
				pTok.InputTokens += state.InputTokens
				pTok.OutputTokens += state.OutputTokens
				pTok.CacheTokens += state.CacheReadTokens
				pTok.TotalTokens += total

				mTok := day.Models[cleanModel]
				if mTok == nil {
					mTok = &ModelTokens{}
					day.Models[cleanModel] = mTok
				}
				mTok.InputTokens += state.InputTokens
				mTok.OutputTokens += state.OutputTokens
				mTok.CacheTokens += state.CacheReadTokens
				mTok.TotalTokens += total

				seededAny = true
			}
		}
	}

	return seededAny
}

// TokenStatsSummary represents formatted aggregated statistics ready for display.
type TokenStatsSummary struct {
	DaysRequested int                   `json:"days_requested"`
	TotalInput    int64                 `json:"total_input_tokens"`
	TotalOutput   int64                 `json:"total_output_tokens"`
	TotalCache    int64                 `json:"total_cache_tokens"`
	TotalTokens   int64                 `json:"total_tokens"`
	AveragePerDay int64                 `json:"average_per_day"`
	DailyRows     []DailyStatRow        `json:"daily_breakdown"`
	ProfileRows   []ProfileStatRow      `json:"profile_breakdown"`
	ModelRows     []ModelStatRow        `json:"model_breakdown"`
}

// DailyStatRow represents one day's token statistics.
type DailyStatRow struct {
	Date         string  `json:"date"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CacheTokens  int64   `json:"cache_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	Percentage   float64 `json:"percentage"`
}

// ProfileStatRow represents aggregated statistics for a specific profile.
type ProfileStatRow struct {
	ProfileName  string  `json:"profile_name"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CacheTokens  int64   `json:"cache_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	Percentage   float64 `json:"percentage"`
}

// ModelStatRow represents aggregated statistics for a specific model.
type ModelStatRow struct {
	ModelName    string  `json:"model_name"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CacheTokens  int64   `json:"cache_tokens"`
	TotalTokens  int64   `json:"total_tokens"`
	Percentage   float64 `json:"percentage"`
}

// GetTokenStatsSummary computes the aggregated token statistics for the requested number of days.
func GetTokenStatsSummary(days int, profileFilter string) (*TokenStatsSummary, error) {
	if days <= 0 {
		days = 7
	}

	store, err := LoadTokenStats()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	startDate := now.AddDate(0, 0, -(days - 1))

	var totalInput, totalOutput, totalCache, totalTokens int64
	dailyMap := make(map[string]*TokenUsageDay)
	profileTotals := make(map[string]*ProfileTokens)
	modelTotals := make(map[string]*ModelTokens)

	// Collect statistics within date range
	for dStr, day := range store.Daily {
		t, parseErr := time.Parse("2006-01-02", dStr)
		if parseErr != nil {
			continue
		}
		if t.Before(startDate.Truncate(24*time.Hour)) && !isSameDay(t, startDate) {
			continue
		}

		if profileFilter != "" {
			pTokens := day.Profiles[profileFilter]
			if pTokens == nil || pTokens.TotalTokens == 0 {
				continue
			}
			filteredDay := &TokenUsageDay{
				Date:         dStr,
				InputTokens:  pTokens.InputTokens,
				OutputTokens: pTokens.OutputTokens,
				CacheTokens:  pTokens.CacheTokens,
				TotalTokens:  pTokens.TotalTokens,
			}
			dailyMap[dStr] = filteredDay
			totalInput += pTokens.InputTokens
			totalOutput += pTokens.OutputTokens
			totalCache += pTokens.CacheTokens
			totalTokens += pTokens.TotalTokens

			pTot := profileTotals[profileFilter]
			if pTot == nil {
				pTot = &ProfileTokens{}
				profileTotals[profileFilter] = pTot
			}
			pTot.InputTokens += pTokens.InputTokens
			pTot.OutputTokens += pTokens.OutputTokens
			pTot.CacheTokens += pTokens.CacheTokens
			pTot.TotalTokens += pTokens.TotalTokens
		} else {
			dailyMap[dStr] = day
			totalInput += day.InputTokens
			totalOutput += day.OutputTokens
			totalCache += day.CacheTokens
			totalTokens += day.TotalTokens

			for pName, pTokens := range day.Profiles {
				pTot := profileTotals[pName]
				if pTot == nil {
					pTot = &ProfileTokens{}
					profileTotals[pName] = pTot
				}
				pTot.InputTokens += pTokens.InputTokens
				pTot.OutputTokens += pTokens.OutputTokens
				pTot.CacheTokens += pTokens.CacheTokens
				pTot.TotalTokens += pTokens.TotalTokens
			}

			for mName, mTokens := range day.Models {
				mTot := modelTotals[mName]
				if mTot == nil {
					mTot = &ModelTokens{}
					modelTotals[mName] = mTot
				}
				mTot.InputTokens += mTokens.InputTokens
				mTot.OutputTokens += mTokens.OutputTokens
				mTot.CacheTokens += mTokens.CacheTokens
				mTot.TotalTokens += mTokens.TotalTokens
			}
		}
	}

	// Generate ordered days (chronological)
	dailyRows := make([]DailyStatRow, 0, days)
	for i := range days {
		dayTime := startDate.AddDate(0, 0, i)
		dStr := dayTime.Format("2006-01-02")
		row := DailyStatRow{
			Date: dStr,
		}
		if dData, ok := dailyMap[dStr]; ok && dData != nil {
			row.InputTokens = dData.InputTokens
			row.OutputTokens = dData.OutputTokens
			row.CacheTokens = dData.CacheTokens
			row.TotalTokens = dData.TotalTokens
			if totalTokens > 0 {
				row.Percentage = float64(dData.TotalTokens) / float64(totalTokens) * 100.0
			}
		}
		dailyRows = append(dailyRows, row)
	}

	// Sort profile breakdown by total descending
	profileRows := make([]ProfileStatRow, 0, len(profileTotals))
	for pName, pTokens := range profileTotals {
		row := ProfileStatRow{
			ProfileName:  pName,
			InputTokens:  pTokens.InputTokens,
			OutputTokens: pTokens.OutputTokens,
			CacheTokens:  pTokens.CacheTokens,
			TotalTokens:  pTokens.TotalTokens,
		}
		if totalTokens > 0 {
			row.Percentage = float64(pTokens.TotalTokens) / float64(totalTokens) * 100.0
		}
		profileRows = append(profileRows, row)
	}
	sort.Slice(profileRows, func(i, j int) bool {
		return profileRows[i].TotalTokens > profileRows[j].TotalTokens
	})

	// Sort model breakdown by total descending
	modelRows := make([]ModelStatRow, 0, len(modelTotals))
	for mName, mTokens := range modelTotals {
		row := ModelStatRow{
			ModelName:    mName,
			InputTokens:  mTokens.InputTokens,
			OutputTokens: mTokens.OutputTokens,
			CacheTokens:  mTokens.CacheTokens,
			TotalTokens:  mTokens.TotalTokens,
		}
		if totalTokens > 0 {
			row.Percentage = float64(mTokens.TotalTokens) / float64(totalTokens) * 100.0
		}
		modelRows = append(modelRows, row)
	}
	sort.Slice(modelRows, func(i, j int) bool {
		return modelRows[i].TotalTokens > modelRows[j].TotalTokens
	})

	avgPerDay := int64(0)
	if days > 0 {
		avgPerDay = totalTokens / int64(days)
	}

	return &TokenStatsSummary{
		DaysRequested: days,
		TotalInput:    totalInput,
		TotalOutput:   totalOutput,
		TotalCache:    totalCache,
		TotalTokens:   totalTokens,
		AveragePerDay: avgPerDay,
		DailyRows:     dailyRows,
		ProfileRows:   profileRows,
		ModelRows:     modelRows,
	}, nil
}

func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// RenderProgressBar renders an ASCII/Unicode progress bar with fixed width.
func RenderProgressBar(fraction float64, width int, useColor bool) string {
	if width <= 0 {
		width = 16
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1.0 {
		fraction = 1.0
	}

	filledLen := int(fraction*float64(width) + 0.5)
	if filledLen > width {
		filledLen = width
	}
	emptyLen := width - filledLen

	filledStr := strings.Repeat("█", filledLen)
	emptyStr := strings.Repeat("░", emptyLen)

	if useColor {
		colorCode := "\033[36m" // Cyan
		if fraction >= 0.8 {
			colorCode = "\033[32m" // Green
		} else if fraction >= 0.4 {
			colorCode = "\033[34m" // Blue
		}
		return fmt.Sprintf("%s%s\033[90m%s\033[0m", colorCode, filledStr, emptyStr)
	}

	return filledStr + emptyStr
}

// FormatNumber formats an integer with thousands separator commas.
func FormatNumber(n int64) string {
	in := fmt.Sprintf("%d", n)
	var out []byte
	l := len(in)
	for i, c := range []byte(in) {
		if i > 0 && (l-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

// FormatTokenStats renders a rich terminal report for token usage statistics.
func FormatTokenStats(summary *TokenStatsSummary, useColor bool) string {
	if summary == nil {
		return "No token statistics available.\n"
	}

	var sb strings.Builder

	cBold := ""
	cDim := ""
	cCyan := ""
	cGreen := ""
	cYellow := ""
	cBlue := ""
	cRst := ""

	if useColor {
		cBold = "\033[1m"
		cDim = "\033[90m"
		cCyan = "\033[36m"
		cGreen = "\033[32m"
		cYellow = "\033[33m"
		cBlue = "\033[34m"
		cRst = "\033[0m"
	}

	// 1. Header
	fmt.Fprintf(&sb, "\n%s⚡ Antigravity Token Usage Statistics%s %s(Last %d Days)%s\n\n",
		cBold+cCyan, cRst, cDim, summary.DaysRequested, cRst)

	if summary.TotalTokens == 0 {
		fmt.Fprintf(&sb, "%sℹ️  No recorded token usage yet.%s Tokens are automatically tracked as you execute prompts with agyp/agy.\n\n", cYellow, cRst)
		return sb.String()
	}

	// Find highest day for bar scaling
	var maxDailyTokens int64
	for _, row := range summary.DailyRows {
		if row.TotalTokens > maxDailyTokens {
			maxDailyTokens = row.TotalTokens
		}
	}

	// 2. Daily Activity Table
	fmt.Fprintf(&sb, "%s📅 Daily Consumption%s\n", cBold, cRst)
	fmt.Fprintf(&sb, "%s%-12s  %12s  %12s  %12s  %12s   %-24s%s\n",
		cDim, "Date", "Input", "Output", "Cached", "Total", "Activity", cRst)
	fmt.Fprintf(&sb, "%s%s%s\n", cDim, strings.Repeat("─", 88), cRst)

	for _, row := range summary.DailyRows {
		frac := 0.0
		if maxDailyTokens > 0 {
			frac = float64(row.TotalTokens) / float64(maxDailyTokens)
		}
		bar := RenderProgressBar(frac, 16, useColor)
		compact := FormatTokenCount(row.TotalTokens)

		dateFmt := row.Date
		if useColor && (row.Date == time.Now().Format("2006-01-02")) {
			dateFmt = fmt.Sprintf("%s%s (today)%s", cBold+cGreen, row.Date, cRst)
		}

		fmt.Fprintf(&sb, "%-12s  %12s  %12s  %12s  %12s   %s %s%-6s%s\n",
			dateFmt,
			FormatNumber(row.InputTokens),
			FormatNumber(row.OutputTokens),
			FormatNumber(row.CacheTokens),
			FormatNumber(row.TotalTokens),
			bar,
			cDim, compact, cRst,
		)
	}
	fmt.Fprintln(&sb)

	// 3. Profile Breakdown
	if len(summary.ProfileRows) > 0 {
		fmt.Fprintf(&sb, "%s👤 Profile Breakdown%s\n", cBold, cRst)
		fmt.Fprintf(&sb, "%s%-16s  %12s  %12s  %12s  %12s   %-24s%s\n",
			cDim, "Profile", "Input", "Output", "Cached", "Total", "Share", cRst)
		fmt.Fprintf(&sb, "%s%s%s\n", cDim, strings.Repeat("─", 88), cRst)

		for _, row := range summary.ProfileRows {
			frac := 0.0
			if summary.TotalTokens > 0 {
				frac = float64(row.TotalTokens) / float64(summary.TotalTokens)
			}
			bar := RenderProgressBar(frac, 14, useColor)

			profName := row.ProfileName
			if useColor {
				profName = fmt.Sprintf("%s%s%s", cCyan, row.ProfileName, cRst)
			}

			fmt.Fprintf(&sb, "%-16s  %12s  %12s  %12s  %12s   %5.1f%% %s\n",
				profName,
				FormatNumber(row.InputTokens),
				FormatNumber(row.OutputTokens),
				FormatNumber(row.CacheTokens),
				FormatNumber(row.TotalTokens),
				row.Percentage,
				bar,
			)
		}
		fmt.Fprintln(&sb)
	}

	// 4. Model Breakdown
	if len(summary.ModelRows) > 0 {
		fmt.Fprintf(&sb, "%s🤖 Model Breakdown%s\n", cBold, cRst)
		fmt.Fprintf(&sb, "%s%-26s  %12s  %12s  %12s  %12s%s\n",
			cDim, "Model", "Input", "Output", "Cached", "Total", cRst)
		fmt.Fprintf(&sb, "%s%s%s\n", cDim, strings.Repeat("─", 88), cRst)

		for _, row := range summary.ModelRows {
			modelName := row.ModelName
			if useColor {
				modelName = fmt.Sprintf("%s%s%s", cBlue, row.ModelName, cRst)
			}
			fmt.Fprintf(&sb, "%-26s  %12s  %12s  %12s  %12s\n",
				modelName,
				FormatNumber(row.InputTokens),
				FormatNumber(row.OutputTokens),
				FormatNumber(row.CacheTokens),
				FormatNumber(row.TotalTokens),
			)
		}
		fmt.Fprintln(&sb)
	}

	// 5. Totals
	fmt.Fprintf(&sb, "%s📊 Summary Totals (%d Days):%s\n", cBold, summary.DaysRequested, cRst)
	fmt.Fprintf(&sb, "  • %sInput Tokens:%s   %s%s%s\n", cDim, cRst, cBold, FormatNumber(summary.TotalInput), cRst)
	fmt.Fprintf(&sb, "  • %sOutput Tokens:%s  %s%s%s\n", cDim, cRst, cBold, FormatNumber(summary.TotalOutput), cRst)
	fmt.Fprintf(&sb, "  • %sCached Tokens:%s  %s%s%s\n", cDim, cRst, cBold, FormatNumber(summary.TotalCache), cRst)
	fmt.Fprintf(&sb, "  • %sTotal Tokens:%s   %s%s%s %s(%s avg/day)%s\n\n",
		cDim, cRst, cBold+cGreen, FormatNumber(summary.TotalTokens), cRst, cDim, FormatTokenCount(summary.AveragePerDay), cRst)

	return sb.String()
}
