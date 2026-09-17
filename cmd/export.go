package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ibravemonkey/agyp/pkg/profile"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var exportCmd = &cobra.Command{
	Use:               "export [profile_name]",
	Short:             "Export a profile (or all profiles) to an archive",
	Long: `Packages profile configuration and credentials into a compressed .tar.gz archive or an encrypted .agyp.enc archive (AES-256-GCM).
Use the --all flag to export all profiles. Use --encrypt (-e) to password-protect the archive.`,
	ValidArgsFunction: CompleteProfileNames,
	Args:              cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		exportAll, _ := cmd.Flags().GetBool("all")
		outputFile, _ := cmd.Flags().GetString("out")
		encrypt, _ := cmd.Flags().GetBool("encrypt")
		passFlag, _ := cmd.Flags().GetString("password")

		var password string
		if encrypt {
			var err error
			password, err = getCryptoPassword(cmd, passFlag, "Enter encryption password: ", true)
			if err != nil {
				return err
			}
		}

		if exportAll {
			if len(args) > 0 {
				return fmt.Errorf("cannot specify a profile name when exporting all profiles (--all)")
			}

			outPath := outputFile
			if outPath == "" {
				if encrypt {
					outPath = "agys_profiles_backup.agyp.enc"
				} else {
					outPath = "agys_profiles_backup.tar.gz"
				}
			}

			parentDir := filepath.Dir(outPath)
			if parentDir != "" && parentDir != "." {
				if err := os.MkdirAll(parentDir, 0755); err != nil {
					return fmt.Errorf("failed to create output directory: %w", err)
				}
			}

			file, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				return fmt.Errorf("failed to create export file: %w", err)
			}
			defer file.Close()

			if encrypt {
				cmd.Println("Exporting all profiles with AES-256-GCM encryption...")
				if err := profile.ExportAllEncrypted(file, password); err != nil {
					_ = os.Remove(outPath)
					return err
				}
			} else {
				cmd.Println("Exporting all profiles...")
				if err := profile.ExportAll(file); err != nil {
					_ = os.Remove(outPath)
					return err
				}
			}

			absPath, err := filepath.Abs(outPath)
			if err != nil {
				absPath = outPath
			}
			cmd.Printf("Successfully exported all profiles to %s\n", absPath)
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("must specify a profile name to export, or use --all")
		}

		profileName := args[0]

		exists, _, err := profile.Exists(profileName)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("profile %q does not exist", profileName)
		}

		outPath := outputFile
		if outPath == "" {
			if encrypt {
				outPath = profileName + ".agyp.enc"
			} else {
				outPath = profileName + ".tar.gz"
			}
		}

		parentDir := filepath.Dir(outPath)
		if parentDir != "" && parentDir != "." {
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}
		}

		file, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("failed to create export file: %w", err)
		}
		defer file.Close()

		if encrypt {
			cmd.Printf("Exporting profile %q with AES-256-GCM encryption...\n", profileName)
			if err := profile.ExportProfileEncrypted(profileName, file, password); err != nil {
				_ = os.Remove(outPath)
				return err
			}
		} else {
			cmd.Printf("Exporting profile %q...\n", profileName)
			if err := profile.ExportProfile(profileName, file); err != nil {
				_ = os.Remove(outPath)
				return err
			}
		}

		absPath, err := filepath.Abs(outPath)
		if err != nil {
			absPath = outPath
		}
		cmd.Printf("Successfully exported profile %q to %s\n", profileName, absPath)
		return nil
	},
}

func getCryptoPassword(cmd *cobra.Command, flagVal string, prompt string, confirm bool) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if env := os.Getenv("AGYP_ENCRYPTION_KEY"); env != "" {
		return env, nil
	}
	if env := os.Getenv("AGYP_PASSWORD"); env != "" {
		return env, nil
	}

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		cmd.Print(prompt)
		passBytes, err := term.ReadPassword(fd)
		cmd.Println()
		if err != nil {
			return "", fmt.Errorf("failed reading password: %w", err)
		}
		password := strings.TrimSpace(string(passBytes))
		if password == "" {
			return "", fmt.Errorf("password cannot be empty")
		}

		if confirm {
			cmd.Print("Confirm password: ")
			confBytes, err := term.ReadPassword(fd)
			cmd.Println()
			if err != nil {
				return "", fmt.Errorf("failed reading password confirmation: %w", err)
			}
			conf := strings.TrimSpace(string(confBytes))
			if password != conf {
				return "", fmt.Errorf("passwords do not match")
			}
		}
		return password, nil
	}

	// Non-terminal fallback (pipes, automated scripts, unit tests)
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err == nil {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}

	return "", fmt.Errorf("password required for encryption/decryption (pass via --password or set AGYP_ENCRYPTION_KEY)")
}

func init() {
	exportCmd.Flags().BoolP("all", "a", false, "Export all profiles to a single archive")
	exportCmd.Flags().StringP("out", "o", "", "Output file path for the exported archive")
	exportCmd.Flags().BoolP("encrypt", "e", false, "Encrypt the archive using AES-256-GCM")
	exportCmd.Flags().String("password", "", "Encryption password (or set AGYP_ENCRYPTION_KEY)")

	_ = exportCmd.RegisterFlagCompletionFunc("out", cobra.FixedCompletions(nil, cobra.ShellCompDirectiveDefault))

	rootCmd.AddCommand(exportCmd)
}
