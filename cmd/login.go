package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/shapedthought/owlctl/auth"
	"github.com/shapedthought/owlctl/models"
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
	Long: `Authenticate with the Veeam API using credentials from environment variables or profiles.json.

The login command now uses secure token storage:
- Tokens are stored in your system keychain (not plaintext files)
- In CI/CD environments, tokens are generated but not stored
- Use --output-token to print the token for CI/CD workflows

Authentication Methods (in priority order):
1. OWLCTL_TOKEN environment variable (if set)
2. System keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)
3. Auto-authenticate using OWLCTL_USERNAME/OWLCTL_PASSWORD/OWLCTL_URL

Examples:
  # Interactive login (stores in keychain)
  owlctl login

  # Output token for CI/CD (doesn't store in keychain)
  export OWLCTL_TOKEN=$(owlctl login --output-token)

  # Debug authentication
  owlctl login --debug-auth
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

	// Create token manager
	tm, err := auth.NewTokenManager(debugAuth)
	if err != nil {
		if debugAuth {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to initialize token manager: %v\n", err)
			fmt.Fprintln(os.Stderr, "Falling back to direct authentication")
		}
		// Fallback to direct authentication
		authenticateDirect(profile, username, password, owlctlUrl, settings.ApiNotSecure)
		return
	}

	// Authenticate
	token, expiresIn, err := tm.AuthenticateWithSettings(profile, username, password, owlctlUrl, settings.ApiNotSecure)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	if outputToken {
		// Just print the token for scripting
		fmt.Println(token)
		return
	}

	// Determine keychain storage key (instance-aware)
	keychainStorageKey := settings.SelectedProfile
	if kcOverride := os.Getenv("OWLCTL_KEYCHAIN_KEY"); kcOverride != "" {
		keychainStorageKey = kcOverride
	}

	// Store token in keychain if interactive (not CI/CD)
	if auth.IsCI() && !tm.StorageEnabled() {
		if debugAuth {
			fmt.Fprintln(os.Stderr, "DEBUG: CI/CD environment detected, skipping keychain storage")
		}
		fmt.Println("Login OK (CI/CD mode - token not stored)")
		fmt.Fprintln(os.Stderr, "Note: In CI/CD, tokens are auto-generated on each command")
		fmt.Fprintln(os.Stderr, "Use 'owlctl login --output-token' to capture token for reuse")
	} else {
		// Store in keychain for interactive sessions
		if err := tm.StoreToken(keychainStorageKey, token, expiresIn); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to store token in keychain: %v\n", err)
			fmt.Fprintln(os.Stderr, "Token will need to be re-generated on next command")
		} else {
			if debugAuth {
				fmt.Fprintf(os.Stderr, "DEBUG: Token stored in system keychain for key '%s'\n", keychainStorageKey)
			}
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

	// For backward compatibility, also write to headers.json (will be removed in future version)
	if !outputToken {
		if err := writeLegacyHeadersFile(token, expiresIn, profile.AuthType == "basic"); err != nil {
			// Non-fatal warning
			if debugAuth {
				fmt.Fprintf(os.Stderr, "WARNING: Failed to write legacy headers.json: %v\n", err)
			}
		} else if debugAuth {
			fmt.Fprintln(os.Stderr, "DEBUG: Legacy headers.json written for backward compatibility")
		}
	}
}

// authenticateDirect is fallback when keyring fails
func authenticateDirect(profile models.Profile, username, password, apiURL string, insecure bool) {
	authenticator := auth.NewAuthenticator(insecure, debugAuth)

	result, err := authenticator.Authenticate(profile, username, password, apiURL)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	if outputToken {
		fmt.Println(result.Token)
		return
	}

	// Write to headers.json (legacy)
	if err := writeLegacyHeadersFile(result.Token, result.ExpiresIn, result.IsBasicAuth); err != nil {
		log.Fatalf("Failed to write token: %v", err)
	}

	fmt.Println("Login OK")
}

// writeLegacyHeadersFile writes headers.json for backward compatibility
// This will be removed in a future version once all code uses TokenManager
func writeLegacyHeadersFile(token string, expiresIn int, isBasicAuth bool) error {
	settingsPath := utils.SettingPath()
	headersFile := settingsPath + "headers.json"

	var data []byte
	if isBasicAuth {
		authModel := models.BasicAuthModel{
			Token:       token,
			ContentType: "application/json",
		}
		var err error
		data, err = json.Marshal(authModel)
		if err != nil {
			return fmt.Errorf("failed to marshal basic auth: %w", err)
		}
	} else {
		header := models.SendHeader{
			AccessToken: token,
			TokenType:   "bearer",
			ExpiresIn:   expiresIn,
		}
		var err error
		data, err = json.Marshal(header)
		if err != nil {
			return fmt.Errorf("failed to marshal OAuth header: %w", err)
		}
	}

	// Use restrictive permissions (0600) to protect tokens from other users
	if err := os.WriteFile(headersFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write headers file: %w", err)
	}

	return nil
}
