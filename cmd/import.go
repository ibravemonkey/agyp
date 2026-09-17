package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ibravemonkey/agyp/pkg/profile"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:               "import <archive_path> [target_profile_name]",
	Short:             "Import a profile (or all profiles) from an archive",
	Long: `Restores a profile directory from a compressed .tar.gz archive or an encrypted .agyp.enc archive.
If target_profile_name is omitted, it is inferred from the archive filename. Use --all to import all profiles.`,
	ValidArgsFunction: CompleteImportArgs,
	Args:              cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		archivePath := args[0]
		importAll, _ := cmd.Flags().GetBool("all")
		forceImport, _ := cmd.Flags().GetBool("force")
		passFlag, _ := cmd.Flags().GetString("password")

		file, err := os.Open(archivePath)
		if err != nil {
			return fmt.Errorf("failed to open archive: %w", err)
		}
		defer file.Close()

		isEnc, reader, err := profile.IsEncryptedArchive(file)
		if err != nil {
			return fmt.Errorf("failed to inspect archive format: %w", err)
		}

		if isEnc {
			password, err := getCryptoPassword(cmd, passFlag, "Enter decryption password: ", false)
			if err != nil {
				return err
			}

			cmd.Println("Decrypting archive with AES-256-GCM...")
			encBytes, err := io.ReadAll(reader)
			if err != nil {
				return fmt.Errorf("failed reading encrypted archive: %w", err)
			}

			decrypted, err := profile.DecryptData(encBytes, password)
			if err != nil {
				return fmt.Errorf("decryption failed: %w", err)
			}

			reader = bytes.NewReader(decrypted)
		}

		if importAll {
			if len(args) > 1 {
				return fmt.Errorf("cannot specify a target profile name when importing all profiles (--all)")
			}

			if err := profile.ImportAll(reader, forceImport); err != nil {
				return err
			}

			cmd.Println("Successfully imported all profiles.")
			return nil
		}

		var targetName string
		if len(args) == 2 {
			targetName = args[1]
		} else {
			base := filepath.Base(archivePath)
			baseLower := strings.ToLower(base)
			if strings.HasSuffix(baseLower, ".agyp.enc") {
				targetName = base[:len(base)-9]
			} else if strings.HasSuffix(baseLower, ".tar.gz.enc") {
				targetName = base[:len(base)-11]
			} else if strings.HasSuffix(baseLower, ".tgz.enc") {
				targetName = base[:len(base)-8]
			} else if strings.HasSuffix(baseLower, ".tar.gz") {
				targetName = base[:len(base)-7]
			} else if strings.HasSuffix(baseLower, ".tgz") {
				targetName = base[:len(base)-4]
			} else {
				ext := filepath.Ext(base)
				targetName = base[:len(base)-len(ext)]
			}
		}

		if err := profile.ImportProfile(reader, targetName, forceImport); err != nil {
			return err
		}

		cmd.Printf("Successfully imported profile as %q.\n", targetName)
		return nil
	},
}

func init() {
	importCmd.Flags().BoolP("all", "a", false, "Import all profiles from the archive")
	importCmd.Flags().BoolP("force", "f", false, "Overwrite existing profiles during import")
	importCmd.Flags().String("password", "", "Decryption password (or set AGYP_ENCRYPTION_KEY)")
	rootCmd.AddCommand(importCmd)
}
