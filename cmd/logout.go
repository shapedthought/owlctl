package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/shapedthought/owlctl/auth"
	"github.com/shapedthought/owlctl/utils"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "End the stored session and delete its tokens",
	Long: `Revoke the stored session on the server and delete the stored access and
refresh token for the active profile or instance.

For VBR this calls POST /api/oauth2/logout, which also invalidates the refresh
token. The stored item is removed even if the server call fails (the failure is
reported and the command exits non-zero). Nothing is contacted when no token is
stored for this server, for example with OWLCTL_KEYRING=none.

OWLCTL_TOKEN is not touched: it is not stored by owlctl.

Examples:
  owlctl logout
  owlctl --instance vbr-prod logout
`,
	Run: func(cmd *cobra.Command, args []string) {
		logoutWithTokenManager()
	},
}

func init() {
	rootCmd.AddCommand(logoutCmd)
	logoutCmd.Flags().BoolVar(&debugAuth, "debug-auth", false, "Enable authentication debug logging")
}

func logoutWithTokenManager() {
	settings := utils.ReadSettings()
	profile := utils.GetProfile(settings.SelectedProfile)

	owlctlUrl := os.Getenv("OWLCTL_URL")
	if owlctlUrl == "" {
		log.Fatal("OWLCTL_URL not set")
	}

	tm, err := auth.NewTokenManager(debugAuth)
	if err != nil {
		log.Fatalf("Failed to initialize token storage: %v", err)
	}
	if !tm.StorageEnabled() {
		fmt.Println("No stored token to remove (token storage is disabled; set OWLCTL_KEYRING to choose a backend)")
		return
	}

	removed, err := tm.Logout(settings.SelectedProfile, profile, owlctlUrl, settings.ApiNotSecure)
	if !removed {
		fmt.Println("No stored token for this server")
		return
	}
	if err != nil {
		fmt.Println("Stored token removed")
		log.Fatalf("Server-side logout did not complete: %v", err)
	}
	fmt.Println("Logout OK (session revoked, stored token removed)")
}
