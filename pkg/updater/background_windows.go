//go:build windows

package updater

import (
	"github.com/ibravemonkey/agyp/pkg/profile"
)

// SpawnDetachedProcess launches a process detached from the console on Windows.
func SpawnDetachedProcess(execPath string, args []string, logFile string) error {
	return profile.SpawnDetachedProcess(execPath, args, logFile)
}
