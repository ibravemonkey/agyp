package runner

import (
	"fmt"
	"io"
	"os"
	"strings"

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
		// Auto profile mode: automatically preserve and use the owning profile
		if profileName != detectedProfile {
			fmt.Fprintf(errOut, "[agys] Resumed conversation detected. Auto-switching profile %q -> %q\n", profileName, detectedProfile)
			profileName = detectedProfile
		}
		return profileName, agyArgs, nil
	}

	if profileName != detectedProfile {
		// Explicit profile specified, different from detected owner:
		// Migrate the conversation brain to the specified profile so the user can continue with the new profile.
		fmt.Fprintf(errOut, "[agys] Resuming conversation %s on specified profile %q (migrating brain from %q)...\n", detectedConvID, profileName, detectedProfile)
		if err := profile.MigrateConversation(detectedConvID, detectedProfile, profileName); err != nil {
			return profileName, agyArgs, fmt.Errorf("failed to migrate conversation %s from %s to %s: %w", detectedConvID, detectedProfile, profileName, err)
		}
		// Replace shorthand resume flags in agyArgs with explicit --conversation=<detectedConvID>
		for i := 0; i < len(agyArgs); i++ {
			if agyArgs[i] == "-c" || agyArgs[i] == "--continue" || agyArgs[i] == "-r" || agyArgs[i] == "--resume" {
				agyArgs[i] = "--conversation=" + detectedConvID
			}
		}
	}

	return profileName, agyArgs, nil
}
