package cmd

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/shapedthought/owlctl/utils"
	"github.com/shapedthought/owlctl/vhttp"
	"github.com/spf13/cobra"
)

var (
	licenseFile   string
	licenseDryRun bool
)

var licenseCmd = &cobra.Command{
	Use:   "license",
	Short: "Manage the VBR license",
}

var licenseInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install a license file (.lic) on VBR",
	Long: `Install a Veeam license file via POST license/install.

The file is read, any leading UTF-8 byte order mark (EF BB BF) is stripped,
and the content is base64-encoded into {"license": "<base64>"}. VBR rejects
BOM-prefixed files with "Data at the root level is invalid".

--dry-run validates the file and prints a summary (size, BOM found, base64
length) without contacting VBR. The license content is never printed.

Examples:
  owlctl license install --file nfr.lic --dry-run
  owlctl license install --file nfr.lic`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLicenseInstall(cmd.OutOrStdout(), licenseFile, licenseDryRun)
	},
}

// licenseInstallBody holds the request body for license/install.
type licenseInstallBody struct {
	License string `json:"license"`
}

// buildLicenseInstallBody strips a UTF-8 BOM and base64-encodes the licence.
// It reports whether a BOM was found.
func buildLicenseInstallBody(raw []byte) (licenseInstallBody, bool) {
	stripped := utils.StripUTF8BOM(raw)
	return licenseInstallBody{License: base64.StdEncoding.EncodeToString(stripped)}, len(stripped) != len(raw)
}

func runLicenseInstall(out io.Writer, path string, dryRun bool) error {
	if path == "" {
		return fmt.Errorf("--file is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading license file: %w", err)
	}
	body, hadBOM := buildLicenseInstallBody(raw)
	stripped := utils.StripUTF8BOM(raw)
	if len(strings.TrimSpace(string(stripped))) == 0 {
		return fmt.Errorf("license file %s is empty", path)
	}
	looksLikeXML := strings.HasPrefix(strings.TrimSpace(string(stripped)), "<")

	if dryRun {
		fmt.Fprintf(out, "Dry run: no request sent to VBR.\n")
		fmt.Fprintf(out, "File:          %s\n", path)
		fmt.Fprintf(out, "Size:          %d bytes\n", len(raw))
		fmt.Fprintf(out, "UTF-8 BOM:     %s\n", yesNo(hadBOM))
		fmt.Fprintf(out, "Looks like XML: %s\n", yesNo(looksLikeXML))
		fmt.Fprintf(out, "Base64 length: %d\n", len(body.License))
		if !looksLikeXML {
			return fmt.Errorf("file does not look like license XML (expected content to start with '<')")
		}
		return nil
	}

	if !looksLikeXML {
		return fmt.Errorf("file does not look like license XML (expected content to start with '<'); use --dry-run to inspect")
	}

	profile := utils.GetCurrentProfile()
	resp, err := vhttp.PostDataWithError("license/install", body, profile)
	if err != nil {
		return err
	}
	if len(resp) > 0 {
		fmt.Fprintln(out, string(resp))
	} else {
		fmt.Fprintln(out, "License installed.")
	}
	return nil
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func init() {
	licenseInstallCmd.Flags().StringVarP(&licenseFile, "file", "f", "", "path to the .lic file")
	licenseInstallCmd.Flags().BoolVar(&licenseDryRun, "dry-run", false, "validate the file and print a summary without calling VBR")
	licenseCmd.AddCommand(licenseInstallCmd)
	rootCmd.AddCommand(licenseCmd)
}
