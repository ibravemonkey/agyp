package runner

import (
	"strings"

	"github.com/ibravemonkey/agyp/pkg/profile"
)

var agySubcommands = map[string]bool{
	"agent":          true,
	"agents":         true,
	"auth":           true,
	"changelog":      true,
	"config":         true,
	"help":           true,
	"install":        true,
	"mcp":            true,
	"mic-serve":      true,
	"models":         true,
	"plugin":         true,
	"plugins":        true,
	"remote-control": true,
	"update":         true,
	"version":        true,
}

var agyBoolFlags = map[string]bool{
	"-c":                             true,
	"--continue":                     true,
	"--dangerously-skip-permissions": true,
	"--disable-slash-commands":       true,
	"--new-project":                  true,
	"--sandbox":                      true,
	"-h":                             true,
	"--help":                         true,
	"-v":                             true,
	"--version":                      true,
}

var agySubcommandVerbs = map[string]map[string]bool{
	"auth":           {"login": true, "logout": true, "status": true, "token": true, "refresh": true},
	"config":         {"get": true, "set": true, "list": true, "path": true},
	"mcp":            {"list": true, "add": true, "remove": true, "enable": true, "disable": true, "status": true},
	"plugin":         {"list": true, "install": true, "uninstall": true, "enable": true, "disable": true},
	"plugins":        {"list": true, "install": true, "uninstall": true, "enable": true, "disable": true},
	"agent":          {"list": true, "create": true, "delete": true, "show": true},
	"agents":         {"list": true},
	"remote-control": {"status": true, "start": true, "stop": true},
}

func isAgySubcommand(arg string) bool {
	return agySubcommands[arg]
}

func isAgySubcommandInvocation(args []string, idx int) bool {
	cmd := args[idx]
	if !isAgySubcommand(cmd) {
		return false
	}
	if idx == len(args)-1 {
		return true
	}
	next := args[idx+1]
	if strings.HasPrefix(next, "-") {
		return true
	}
	if verbs, ok := agySubcommandVerbs[cmd]; ok && verbs[next] {
		return true
	}
	return false
}

func findFirstPositionalIndex(args []string) int {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return -1
		}
		if strings.HasPrefix(arg, "-") {
			if strings.Contains(arg, "=") || agyBoolFlags[arg] {
				continue
			}
			i++ // skip flag argument
			continue
		}
		return i
	}
	return -1
}

// IsAgySubcommandCall checks if the given arguments represent a direct agy subcommand call.
func IsAgySubcommandCall(args []string) bool {
	idx := findFirstPositionalIndex(args)
	if idx < 0 {
		return false
	}
	return isAgySubcommandInvocation(args, idx)
}

// IsInteractiveSession returns true if the agy command starts an interactive chat/session.
func IsInteractiveSession(agyArgs []string) bool {
	for _, arg := range agyArgs {
		if arg == "-p" || arg == "--print" || arg == "--prompt" ||
			strings.HasPrefix(arg, "-p=") || strings.HasPrefix(arg, "--prompt=") || strings.HasPrefix(arg, "--print=") ||
			strings.HasPrefix(arg, "--input-format") || strings.HasPrefix(arg, "--output-format") ||
			arg == "-h" || arg == "--help" || arg == "-v" || arg == "--version" {
			return false
		}
	}

	return !IsAgySubcommandCall(agyArgs)
}

// EnsureDefaultModelAndEffort ensures agyArgs has a default model and reasoning effort.
func EnsureDefaultModelAndEffort(args []string) []string {
	return EnsureDefaultModelAndEffortWithModel(args, profile.GetLatestGeminiModel())
}

// EnsureDefaultModelAndEffortWithModel ensures agyArgs has the specified model and effort.
func EnsureDefaultModelAndEffortWithModel(args []string, defaultModel string) []string {
	if defaultModel == "" {
		defaultModel = profile.GetLatestGeminiModel()
	}

	if IsAgySubcommandCall(args) {
		return args
	}

	hasModel := false
	hasEffort := false
	modelValue := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-m" || arg == "--model" {
			hasModel = true
			if i+1 < len(args) {
				modelValue = args[i+1]
			}
		} else if strings.HasPrefix(arg, "--model=") {
			hasModel = true
			modelValue = strings.TrimPrefix(arg, "--model=")
		} else if strings.HasPrefix(arg, "-m=") {
			hasModel = true
			modelValue = strings.TrimPrefix(arg, "-m=")
		} else if arg == "--effort" || strings.HasPrefix(arg, "--effort=") {
			hasEffort = true
		}
	}

	finalArgs := make([]string, len(args), len(args)+4)
	copy(finalArgs, args)

	if !hasModel {
		finalArgs = append(finalArgs, "--model", defaultModel)
		modelValue = defaultModel
	} else if modelValue == "auto" || modelValue == "latest" {
		targetModel := profile.GetLatestGeminiModel()
		for i := 0; i < len(finalArgs); i++ {
			if (finalArgs[i] == "-m" || finalArgs[i] == "--model") && i+1 < len(finalArgs) {
				finalArgs[i] = "--model"
				finalArgs[i+1] = targetModel
				modelValue = targetModel
				break
			} else if strings.HasPrefix(finalArgs[i], "--model=") {
				finalArgs[i] = "--model=" + targetModel
				modelValue = targetModel
				break
			} else if strings.HasPrefix(finalArgs[i], "-m=") {
				finalArgs[i] = "--model=" + targetModel
				modelValue = targetModel
				break
			}
		}
	}

	if !hasEffort {
		if profile.ModelSupportsEffort(modelValue) {
			finalArgs = append(finalArgs, "--effort", "high")
		}
	}

	// Normalize shorthand -m and -m= to canonical --model
	for i := 0; i < len(finalArgs); i++ {
		if finalArgs[i] == "-m" && i+1 < len(finalArgs) {
			finalArgs[i] = "--model"
		} else if strings.HasPrefix(finalArgs[i], "-m=") {
			finalArgs[i] = "--model=" + strings.TrimPrefix(finalArgs[i], "-m=")
		}
	}

	return finalArgs
}
