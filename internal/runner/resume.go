package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

// ResolveResumeProfile detects if a resume flag is passed and routes or migrates the conversation.
func ResolveResumeProfile(profileName string, agyArgs []string, errOut io.Writer, workingDir ...string) (string, []string, error) {
	if errOut == nil {
		errOut = os.Stderr
	}

	var detectedProfile string
	var detectedConvID string
	var detectErr error

	for i := 0; i < len(agyArgs); i++ {
		arg := agyArgs[i]
		if (arg == "--conversation" || arg == "--resume") && i+1 < len(agyArgs) {
			detectedConvID = agyArgs[i+1]
			detectedProfile, detectErr = profile.FindProfileByConversation(detectedConvID)
			break
		} else if strings.HasPrefix(arg, "--conversation=") {
			detectedConvID = strings.TrimPrefix(arg, "--conversation=")
			detectedProfile, detectErr = profile.FindProfileByConversation(detectedConvID)
			break
		} else if strings.HasPrefix(arg, "--resume=") {
			detectedConvID = strings.TrimPrefix(arg, "--resume=")
			detectedProfile, detectErr = profile.FindProfileByConversation(detectedConvID)
			break
		} else if arg == "-c" || arg == "--continue" || arg == "-r" || arg == "--resume" {
			targetDir := ""
			if len(workingDir) > 0 && workingDir[0] != "" {
				targetDir = workingDir[0]
			} else {
				targetDir, _ = os.Getwd()
			}
			detectedProfile, detectedConvID, detectErr = profile.FindProfileAndConvByLatestConversationInWorkspace(targetDir)
			break
		}
	}

	if detectErr != nil || detectedProfile == "" {
		return profileName, agyArgs, nil
	}

	if profile.IsAuto(profileName) {
		// Auto profile mode: check if owning profile has sufficient quota.
		// If owning profile has low/exhausted quota (<= 5% or error), and another profile has better quota,
		// seamlessly auto-migrate conversation to the winning profile.
		bestProfile := detectedProfile
		if detectedConvID != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			detectedScore := -1.0
			if summary, err := profile.FetchQuota(ctx, detectedProfile); err == nil {
				detectedScore = profile.Calculate5HQuotaScore(summary)
			}

			if detectedScore <= 0.05 {
				if candidate, candidateScore, err := profile.SelectBestProfileFiltered(ctx, func(p string) bool {
					return p != detectedProfile
				}); err == nil && candidate != "" && candidateScore > detectedScore {
					bestProfile = candidate
					scoreStr := "0.0%"
					if detectedScore > 0 {
						scoreStr = fmt.Sprintf("%.1f%%", detectedScore*100)
					}
					fmt.Fprintf(errOut, "[agyp] Auto-mode: profile %q quota low/exhausted (%s). Auto-migrating conversation %s to %q (quota: %.1f%%)...\n",
						detectedProfile, scoreStr, detectedConvID, bestProfile, candidateScore*100)
					if migErr := profile.MigrateConversation(detectedConvID, detectedProfile, bestProfile); migErr != nil {
						fmt.Fprintf(errOut, "[agyp] Warning: migration failed: %v. Continuing on %q\n", migErr, detectedProfile)
						bestProfile = detectedProfile
					} else {
						// Replace shorthand resume flags in agyArgs with explicit --conversation=<detectedConvID>
						for i := range agyArgs {
							if agyArgs[i] == "-c" || agyArgs[i] == "--continue" || agyArgs[i] == "-r" || agyArgs[i] == "--resume" {
								agyArgs[i] = "--conversation=" + detectedConvID
							}
						}
					}
				}
			}
		}

		if bestProfile == detectedProfile && profileName != detectedProfile {
			fmt.Fprintf(errOut, "[agyp] Resumed conversation detected. Auto-switching profile %q -> %q\n", profileName, detectedProfile)
		}
		return bestProfile, agyArgs, nil
	}

	if profileName != detectedProfile {
		// Explicit profile specified, different from detected owner:
		// Migrate the conversation brain to the specified profile so the user can continue with the new profile.
		fmt.Fprintf(errOut, "[agyp] Resuming conversation %s on specified profile %q (migrating brain from %q)...\n", detectedConvID, profileName, detectedProfile)
		if err := profile.MigrateConversation(detectedConvID, detectedProfile, profileName); err != nil {
			return profileName, agyArgs, fmt.Errorf("failed to migrate conversation %s from %s to %s: %w", detectedConvID, detectedProfile, profileName, err)
		}
		// Replace shorthand resume flags in agyArgs with explicit --conversation=<detectedConvID>
		for i := range agyArgs {
			if agyArgs[i] == "-c" || agyArgs[i] == "--continue" || agyArgs[i] == "-r" || agyArgs[i] == "--resume" {
				agyArgs[i] = "--conversation=" + detectedConvID
			}
		}
	}

	return profileName, agyArgs, nil
}
