package profile

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultTTL is the default retention period for conversation sessions (14 days).
const DefaultTTL = 14 * 24 * time.Hour

// DefaultKeepLast is the default minimum number of recent sessions to preserve.
const DefaultKeepLast = 5

// CleanOptions configures profile cleaning parameters.
type CleanOptions struct {
	TTL          time.Duration // Sessions older than this duration are eligible for cleanup
	KeepLast     int           // Minimum number of most recent sessions to preserve
	DryRun       bool          // If true, simulate cleanup and calculate space without deleting
	CacheOnly    bool          // If true, clean only caches and logs, preserving all sessions
	SessionsOnly bool          // If true, clean only old sessions, preserving caches
	RemoveLocks  bool          // If true, remove stale .agyp.lock files
}

// CleanReport summarizes the actions taken (or simulated) during a profile cleanup.
type CleanReport struct {
	ProfileName     string   `json:"profile_name"`
	SessionsScanned int      `json:"sessions_scanned"`
	SessionsCleaned int      `json:"sessions_cleaned"`
	SessionsKept    int      `json:"sessions_kept"`
	CachesCleaned   []string `json:"caches_cleaned"`
	BytesFreed      int64    `json:"bytes_freed"`
	CleanedConvIDs  []string `json:"cleaned_conv_ids"`
	DryRun          bool     `json:"dry_run"`
	Errors          []string `json:"errors,omitempty"`
}

var (
	daysRegex   = regexp.MustCompile(`^(\d+)\s*(?:d|day|days)$`)
	weeksRegex  = regexp.MustCompile(`^(\d+)\s*(?:w|week|weeks)$`)
	monthsRegex = regexp.MustCompile(`^(\d+)\s*(?:month|months)$`)
)

// ParseTTL parses human-readable duration strings supporting days ("14d", "7days"),
// weeks ("2w", "1week"), months ("1month"), as well as standard Go durations ("48h", "30m").
// An empty string returns DefaultTTL.
func ParseTTL(s string) (time.Duration, error) {
	trimmed := strings.ToLower(strings.TrimSpace(s))
	if trimmed == "" {
		return DefaultTTL, nil
	}

	if m := daysRegex.FindStringSubmatch(trimmed); len(m) == 2 {
		days, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, fmt.Errorf("invalid days count %q: %w", m[1], err)
		}
		if days < 0 {
			return 0, fmt.Errorf("duration cannot be negative")
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	if m := weeksRegex.FindStringSubmatch(trimmed); len(m) == 2 {
		weeks, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, fmt.Errorf("invalid weeks count %q: %w", m[1], err)
		}
		if weeks < 0 {
			return 0, fmt.Errorf("duration cannot be negative")
		}
		return time.Duration(weeks) * 7 * 24 * time.Hour, nil
	}

	if m := monthsRegex.FindStringSubmatch(trimmed); len(m) == 2 {
		months, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, fmt.Errorf("invalid months count %q: %w", m[1], err)
		}
		if months < 0 {
			return 0, fmt.Errorf("duration cannot be negative")
		}
		// Approximate 30 days per month
		return time.Duration(months) * 30 * 24 * time.Hour, nil
	}

	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (use e.g. '14d', '7days', '48h', '1w'): %w", s, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("duration cannot be negative")
	}
	return d, nil
}

// FormatBytes formats byte count into human-readable representation (e.g. "12.4 MB").
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// DirSize calculates the total size of all regular files in a directory tree.
func DirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

type sessionSortItem struct {
	convID         string
	convDir        string
	modTime        time.Time
	size           int64
	transcriptPath string
}

// CleanProfile performs disk cleanup on a single profile according to CleanOptions.
func CleanProfile(profileName string, opts CleanOptions) (*CleanReport, error) {
	exists, profileDir, err := Exists(profileName)
	if err != nil {
		return nil, fmt.Errorf("failed checking profile existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("profile %q does not exist", profileName)
	}

	if opts.TTL <= 0 {
		opts.TTL = DefaultTTL
	}
	if opts.KeepLast < 0 {
		opts.KeepLast = DefaultKeepLast
	}

	report := &CleanReport{
		ProfileName: profileName,
		DryRun:      opts.DryRun,
	}

	// 1. Clean transient caches unless SessionsOnly is set
	if !opts.SessionsOnly {
		cleanProfileCaches(profileDir, opts, report)
	}

	// 2. Clean old sessions unless CacheOnly is set
	if !opts.CacheOnly {
		cleanProfileSessions(profileDir, opts, report)
	}

	return report, nil
}

// CleanAllProfiles performs disk cleanup on all configured profiles.
func CleanAllProfiles(opts CleanOptions) ([]*CleanReport, error) {
	profiles, err := List()
	if err != nil {
		return nil, fmt.Errorf("failed to list profiles: %w", err)
	}

	var reports []*CleanReport
	for _, p := range profiles {
		rep, err := CleanProfile(p, opts)
		if err != nil {
			rep = &CleanReport{
				ProfileName: p,
				DryRun:      opts.DryRun,
				Errors:      []string{err.Error()},
			}
		}
		reports = append(reports, rep)
	}

	return reports, nil
}

// transientCacheRelativeDirs lists directories inside a profile that contain temporary caches/logs.
var transientCacheRelativeDirs = []string{
	filepath.Join("ide-data", "logs"),
	filepath.Join("ide-data", "blob_storage"),
	filepath.Join("Crashpad"),
	filepath.Join("crashes"),
	filepath.Join("Caches"),
	filepath.Join(".cache"),
	filepath.Join(".npm"),
	filepath.Join(".Trash"),
}

func cleanProfileCaches(profileDir string, opts CleanOptions, report *CleanReport) {
	for _, rel := range transientCacheRelativeDirs {
		targetPath := filepath.Join(profileDir, rel)
		info, err := os.Stat(targetPath)
		if err != nil || !info.IsDir() {
			continue
		}

		size := DirSize(targetPath)
		if size > 0 || !opts.DryRun {
			report.BytesFreed += size
			report.CachesCleaned = append(report.CachesCleaned, rel)
			if !opts.DryRun {
				// Remove directory content or directory itself, then recreate empty dir
				if err := os.RemoveAll(targetPath); err != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("failed removing cache %s: %v", rel, err))
				}
			}
		}
	}

	// Clean stale .agyp.lock if requested or older than 1 hour
	if opts.RemoveLocks {
		lockFile := filepath.Join(profileDir, ".agyp.lock")
		if info, err := os.Stat(lockFile); err == nil && !info.IsDir() {
			if time.Since(info.ModTime()) > time.Hour {
				report.BytesFreed += info.Size()
				report.CachesCleaned = append(report.CachesCleaned, ".agyp.lock")
				if !opts.DryRun {
					_ = os.Remove(lockFile)
				}
			}
		}
	}
}

func cleanProfileSessions(profileDir string, opts CleanOptions, report *CleanReport) {
	brainDirs := getProfileBrainDirs(profileDir)
	if len(brainDirs) == 0 {
		return
	}

	now := time.Now()
	var sessions []sessionSortItem

	for _, bDir := range brainDirs {
		entries, err := os.ReadDir(bDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			convID := entry.Name()
			convDir := filepath.Join(bDir, convID)
			transcriptPath := filepath.Join(convDir, ".system_generated", "logs", "transcript.jsonl")

			modTime := entryModTime(convDir, transcriptPath)
			size := DirSize(convDir)

			sessions = append(sessions, sessionSortItem{
				convID:         convID,
				convDir:        convDir,
				modTime:        modTime,
				size:           size,
				transcriptPath: transcriptPath,
			})
		}
	}

	report.SessionsScanned = len(sessions)
	if len(sessions) == 0 {
		return
	}

	// Sort sessions by modTime descending (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].modTime.After(sessions[j].modTime)
	})

	var toClean []sessionSortItem
	var keptCount int

	for i, sess := range sessions {
		// Preserve at least opts.KeepLast newest sessions
		if i < opts.KeepLast {
			keptCount++
			continue
		}

		// Check TTL
		age := now.Sub(sess.modTime)
		if age > opts.TTL {
			toClean = append(toClean, sess)
		} else {
			keptCount++
		}
	}

	report.SessionsKept = keptCount

	if len(toClean) == 0 {
		return
	}

	var cleanedIDs []string
	for _, sess := range toClean {
		report.BytesFreed += sess.size
		report.SessionsCleaned++
		cleanedIDs = append(cleanedIDs, sess.convID)

		if !opts.DryRun {
			if err := os.RemoveAll(sess.convDir); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("failed to remove session %s: %v", sess.convID, err))
			}
		}
	}
	report.CleanedConvIDs = cleanedIDs

	if !opts.DryRun && len(cleanedIDs) > 0 {
		// Prune cleaned IDs from history.jsonl
		pruneHistoryEntries(profileDir, cleanedIDs)

		// Invalidate / update session cache
		pruneSessionCacheEntries(report.ProfileName, cleanedIDs)
	}
}

func entryModTime(convDir, transcriptPath string) time.Time {
	if info, err := os.Stat(transcriptPath); err == nil {
		return info.ModTime()
	}
	if info, err := os.Stat(convDir); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

// pruneHistoryEntries removes records of deleted conversation IDs from history.jsonl.
func pruneHistoryEntries(profileDir string, deletedConvIDs []string) {
	if len(deletedConvIDs) == 0 {
		return
	}

	delSet := make(map[string]bool, len(deletedConvIDs))
	for _, id := range deletedConvIDs {
		delSet[id] = true
	}

	historyPaths := []string{
		filepath.Join(profileDir, ".gemini", "antigravity-cli", "history.jsonl"),
		filepath.Join(profileDir, ".gemini", "antigravity", "history.jsonl"),
	}

	for _, hp := range historyPaths {
		data, err := os.ReadFile(hp)
		if err != nil {
			continue
		}

		var updatedLines [][]byte
		scanner := bufio.NewScanner(bytes.NewReader(data))
		// Increase buffer size in case of long lines
		buf := make([]byte, 1024*1024)
		scanner.Buffer(buf, 10*1024*1024)

		modified := false
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			// Fast check before JSON parse
			shouldDelete := false
			for id := range delSet {
				if bytes.Contains(line, []byte(id)) {
					// Verify via JSON unmarshal
					var item struct {
						ConversationID string `json:"conversationId"`
					}
					if err := json.Unmarshal(line, &item); err == nil && delSet[item.ConversationID] {
						shouldDelete = true
						break
					}
				}
			}

			if shouldDelete {
				modified = true
			} else {
				// Copy line to avoid scanner reuse
				copied := make([]byte, len(line))
				copy(copied, line)
				updatedLines = append(updatedLines, copied)
			}
		}

		if modified {
			var out bytes.Buffer
			for _, l := range updatedLines {
				out.Write(l)
				out.WriteByte('\n')
			}
			_ = WriteFileAtomic(hp, out.Bytes(), 0600)
		}
	}
}

// pruneSessionCacheEntries removes pruned conversation IDs from the global session_cache.json.
func pruneSessionCacheEntries(profileName string, deletedConvIDs []string) {
	cache, err := LoadSessionCache()
	if err != nil || cache == nil {
		return
	}

	modified := false
	for _, id := range deletedConvIDs {
		key := profileName + ":" + id
		if _, exists := cache[key]; exists {
			delete(cache, key)
			modified = true
		}
	}

	if modified {
		_ = SaveSessionCache(cache)
	}
}
