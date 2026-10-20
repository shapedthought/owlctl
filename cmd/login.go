package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/shapedthought/owlctl/auth"
	"github.com/shapedthought/owlctl/utils"
	"github.com/spf13/cobra"
)

var (
	outputToken bool
	debugAuth   bool
)

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the Veeam API",
	Long: `Authenticate with the Veeam API using credentials from environment variables.

Tokens are kept in a system keyring (OWLCTL_KEYRING selects the backend), never in
plaintext files. VBR sessions are stored as an access token plus a refresh token,
so later commands renew the 15-minute access token themselves and do not need
OWLCTL_PASSWORD until the refresh token expires. The password is never stored.
The backend does not depend on whether a terminal is attached; OWLCTL_KEYRING=none
disables storage. Use --output-token to capture an access token instead.

Token resolution for every command (in priority order):
1. OWLCTL_TOKEN environment variable
2. Stored access token (if it has more than 60s left)
3. Stored refresh token
4. OWLCTL_USERNAME/OWLCTL_PASSWORD/OWLCTL_URL
5. Error: run 'owlctl login'

Examples:
  # Interactive login (stores the session in the keyring)
  owlctl login

  # Output token for CI/CD (doesn't store anything)
  export OWLCTL_TOKEN=$(owlctl login --output-token)

  # Debug authentication (shows backend and expiry times, never tokens)
  owlctl login --debug-auth

  # End the session: revoke it on the server and delete the stored token
  owlctl logout
`,
	Run: func(cmd *cobra.Command, args []string) {
		loginWithTokenManager()
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.Flags().BoolVar(&outputToken, "output-token", false, "Output token to stdout (for CI/CD)")
	loginCmd.Flags().BoolVar(&debugAuth, "debug-auth", false, "Enable authentication debug logging")
}

func loginWithTokenManager() {
	settings := utils.ReadSettings()
	profile := utils.GetProfile(settings.SelectedProfile)

	// Get credentials from environment variables
	// Note: With v1.0 profiles, credentials are no longer stored in profiles.json
	username := os.Getenv("OWLCTL_USERNAME")
	owlctlUrl := os.Getenv("OWLCTL_URL")
	password := os.Getenv("OWLCTL_PASSWORD")

	// Validate credentials
	if username == "" {
		log.Fatal("OWLCTL_USERNAME not set")
	}
	if password == "" {
		log.Fatal("OWLCTL_PASSWORD not set")
	}
	if owlctlUrl == "" {
		log.Fatal("OWLCTL_URL not set")
	}

	if outputToken {
		// Just print the access token for scripting; nothing is stored, so no
		// keyring is opened.
		result, err := auth.NewAuthenticator(settings.ApiNotSecure, debugAuth).Authenticate(profile, username, password, owlctlUrl)
		if err != nil {
			log.Fatalf("Authentication failed: %v", err)
		}
		fmt.Println(result.Token)
		return
	}

	// Create token manager (fails on an invalid or unusable OWLCTL_KEYRING)
	tm, err := auth.NewTokenManager(debugAuth)
	if err != nil {
		log.Fatalf("Failed to initialize token storage: %v", err)
	}

	result, err := tm.AuthenticateWithSettings(profile, username, password, owlctlUrl, settings.ApiNotSecure)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	// Determine keychain storage key (instance-aware)
	keychainStorageKey := settings.SelectedProfile
	if kcOverride := os.Getenv("OWLCTL_KEYCHAIN_KEY"); kcOverride != "" {
		keychainStorageKey = kcOverride
	}

	// Store the session (access + refresh token, bound to this server) when a keyring is active
	if auth.IsCI() && !tm.StorageEnabled() {
		if debugAuth {
			fmt.Fprintln(os.Stderr, "DEBUG: CI/CD environment detected, skipping keychain storage")
		}
		fmt.Println("Login OK (CI/CD mode - token not stored)")
		fmt.Fprintln(os.Stderr, "Note: In CI/CD, tokens are auto-generated on each command")
		fmt.Fprintln(os.Stderr, "Use 'owlctl login --output-token' to capture token for reuse")
		return
	}

	if err := tm.StoreSession(keychainStorageKey, result, owlctlUrl, profile.Port); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to store token in keychain: %v\n", err)
		fmt.Fprintln(os.Stderr, "Token will need to be re-generated on next command")
	} else if debugAuth {
		fmt.Fprintf(os.Stderr, "DEBUG: Token stored for key '%s'\n", keychainStorageKey)
	}
	note := ""
	if !tm.StorageEnabled() {
		note = "token not stored: keyring disabled; set OWLCTL_KEYRING to store it"
	}
	switch {
	case instanceFlag != "" && note != "":
		fmt.Printf("Login OK (instance: %s; %s)\n", instanceFlag, note)
	case instanceFlag != "":
		fmt.Printf("Login OK (instance: %s)\n", instanceFlag)
	case note != "":
		fmt.Printf("Login OK (%s)\n", note)
	default:
		fmt.Println("Login OK")
	}
}
