package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

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

	storeErr := tm.StoreSession(keychainStorageKey, result, owlctlUrl, profile.Port)
	if storeErr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to store token in %s: %v\n", storeFailureTarget(tm.Backend()), storeErr)
		fmt.Fprintln(os.Stderr, storeFailureHint(tm.Backend()))
		fmt.Fprintln(os.Stderr, "Token will need to be re-generated on next command")
	} else if debugAuth {
		fmt.Fprintf(os.Stderr, "DEBUG: Token stored for key '%s'\n", keychainStorageKey)
	}
	fmt.Println(loginStatusLine(instanceFlag, tm.StorageEnabled(), tm.Degraded(), storeErr))
}

// storeFailureTarget names the store in the stderr warning.
func storeFailureTarget(backend string) string {
	if backend == "keychain" {
		return "keychain"
	}
	return "keyring"
}

// storeFailureHint gives backend-specific advice; OWLCTL_FILE_KEY is only
// relevant to the file backend.
func storeFailureHint(backend string) string {
	switch backend {
	case "file":
		return "Fix: set OWLCTL_FILE_KEY (file keyring), or set OWLCTL_KEYRING=none to skip storage"
	case "keychain":
		return "Fix: choose \"Always Allow\" in the Keychain prompt, run 'security unlock-keychain' if the login keychain is locked, or set OWLCTL_KEYRING=none to skip storage"
	}
	return "Fix: make sure the keyring is running and unlocked, set OWLCTL_KEYRING=file with OWLCTL_FILE_KEY, or set OWLCTL_KEYRING=none to skip storage"
}

// loginStatusLine builds the stdout line. A plain "Login OK" is returned only
// when the session was actually stored; the error is reduced to a short fixed
// reason so no raw error text reaches stdout. degraded is non-empty when "auto"
// had to give up on storage, as opposed to the user choosing OWLCTL_KEYRING=none.
func loginStatusLine(instance string, storageEnabled bool, degraded string, storeErr error) string {
	prefix := "Login OK"
	if instance != "" {
		prefix = fmt.Sprintf("Login OK (instance: %s)", instance)
	}
	switch {
	case !storageEnabled && degraded != "":
		return fmt.Sprintf("%s — token NOT stored (keyring unavailable: %s)", prefix, degraded)
	case !storageEnabled:
		if instance != "" {
			return fmt.Sprintf("Login OK (instance: %s; token not stored: keyring disabled; set OWLCTL_KEYRING to store it)", instance)
		}
		return "Login OK (token not stored: keyring disabled; set OWLCTL_KEYRING to store it)"
	case storeErr != nil:
		return fmt.Sprintf("%s — token NOT stored (%s)", prefix, shortStoreReason(storeErr))
	}
	return prefix
}

func shortStoreReason(err error) string {
	if strings.Contains(err.Error(), "OWLCTL_FILE_KEY") {
		return "OWLCTL_FILE_KEY not set; set it or use OWLCTL_KEYRING=none"
	}
	if errors.Is(err, auth.ErrKeyringTimeout) {
		return "keyring timed out; set OWLCTL_KEYRING=none to skip storage"
	}
	return "keyring write failed; set OWLCTL_KEYRING=none to skip storage"
}
