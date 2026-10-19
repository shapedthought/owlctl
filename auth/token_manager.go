package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/99designs/keyring"
	"github.com/shapedthought/owlctl/models"
	"golang.org/x/term"
)

const (
	KeyringService = "owlctl"
	TokenEnvVar    = "OWLCTL_TOKEN"
)

// TokenManager handles token storage and retrieval
type TokenManager struct {
	keyring  keyring.Keyring // nil when storage is disabled
	debug    bool
	backend  string
	timeout  time.Duration // 0 = no timeout
	timedOut bool
	hintOnce sync.Once
}

// TokenInfo holds token metadata
type TokenInfo struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	IssuedAt  time.Time `json:"issued_at"`
	Profile   string    `json:"profile"`
}

// StorageEnabled reports whether tokens can be persisted (a keyring is open).
func (tm *TokenManager) StorageEnabled() bool {
	return tm.keyring != nil
}

// keychainKey returns the effective keychain key for token storage/retrieval.
// If OWLCTL_KEYCHAIN_KEY is set (by instance activation), it is used instead of profileName.
// This ensures separate keychain entries per instance (e.g., "instance:vbr-prod" vs "instance:vbr-dr").
func keychainKey(profileName string) string {
	if override := os.Getenv("OWLCTL_KEYCHAIN_KEY"); override != "" {
		return override
	}
	return profileName
}

// GetToken retrieves token using hybrid approach
// Priority: 1) OWLCTL_TOKEN env var, 2) keychain, 3) auto-authenticate
// profileName is used for keychain storage/retrieval
func (tm *TokenManager) GetToken(profileName string, profile models.Profile, username, password, apiURL string, insecure bool) (string, error) {
	if tm.debug {
		fmt.Fprintln(os.Stderr, "DEBUG: Token resolution started")
	}

	kcKey := keychainKey(profileName)

	// 1. Check explicit token env var (highest priority)
	if token := os.Getenv(TokenEnvVar); token != "" {
		if tm.debug {
			fmt.Fprintln(os.Stderr, "DEBUG: Using token from OWLCTL_TOKEN env var")
		}
		// Validate token format and expiry if possible
		if isValidTokenFormat(token) {
			return token, nil
		}
		if tm.debug {
			fmt.Fprintln(os.Stderr, "DEBUG: OWLCTL_TOKEN token invalid, trying next method")
		}
	}

	// 2. Check system keychain
	if token, err := tm.getTokenFromKeychain(kcKey); err == nil {
		if tm.debug {
			fmt.Fprintln(os.Stderr, "DEBUG: Using token from system keychain")
		}
		return token, nil
	} else if tm.debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Keychain lookup failed: %v\n", err)
	}

	// 3. Auto-authenticate if credentials available
	if username != "" && password != "" && apiURL != "" {
		if tm.debug {
			fmt.Fprintln(os.Stderr, "DEBUG: Auto-authenticating with credentials")
		}
		token, expiresIn, err := tm.AuthenticateWithSettings(profile, username, password, apiURL, insecure)
		if err != nil {
			return "", fmt.Errorf("auto-authentication failed: %w", err)
		}

		// Store only when a keyring is active (never in non-interactive sessions
		// unless the user chose a backend via OWLCTL_KEYRING)
		if tm.StorageEnabled() {
			if tm.debug {
				fmt.Fprintln(os.Stderr, "DEBUG: Storing token in keychain (interactive session)")
			}
			if err := tm.StoreToken(kcKey, token, expiresIn); err != nil {
				// Non-fatal: just warn
				if tm.debug {
					fmt.Fprintf(os.Stderr, "DEBUG: Failed to store token in keychain: %v\n", err)
				}
			}
		} else if tm.debug {
			fmt.Fprintln(os.Stderr, "DEBUG: Skipping keychain storage (CI/CD environment)")
		}

		return token, nil
	}

	return "", errors.New("no authentication method available: set OWLCTL_TOKEN, store token in keychain, or provide OWLCTL_USERNAME/OWLCTL_PASSWORD/OWLCTL_URL")
}

// StoreToken stores a token in the system keychain
func (tm *TokenManager) StoreToken(profileName, token string, expiresIn int) error {
	if tm.keyring == nil {
		return nil // storage disabled: nothing to persist
	}
	info := TokenInfo{
		Token:     token,
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(expiresIn) * time.Second),
		Profile:   profileName,
	}

	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal token info: %w", err)
	}

	item := keyring.Item{
		Key:  profileName,
		Data: data,
	}

	if err := tm.keyringCall(func(kr keyring.Keyring) error { return kr.Set(item) }); err != nil {
		return fmt.Errorf("failed to store token in keychain: %w", err)
	}

	return nil
}

// getTokenFromKeychain retrieves token from system keychain
func (tm *TokenManager) getTokenFromKeychain(profileName string) (string, error) {
	item, err := tm.keyringGet(profileName)
	if err != nil {
		return "", fmt.Errorf("token not found in keychain: %w", err)
	}

	var info TokenInfo
	if err := json.Unmarshal(item.Data, &info); err != nil {
		return "", fmt.Errorf("failed to unmarshal token info: %w", err)
	}

	// Check if token is expired
	if time.Now().After(info.ExpiresAt) {
		if tm.debug {
			fmt.Fprintln(os.Stderr, "DEBUG: Keychain token expired")
		}
		// Remove expired token
		_ = tm.keyringCall(func(kr keyring.Keyring) error { return kr.Remove(profileName) })
		return "", errors.New("token expired")
	}

	return info.Token, nil
}

// DeleteToken removes a token from keychain
func (tm *TokenManager) DeleteToken(profileName string) error {
	if tm.keyring == nil {
		return nil // nothing is stored when storage is disabled
	}
	return tm.keyringCall(func(kr keyring.Keyring) error { return kr.Remove(profileName) })
}

// AuthenticateWithSettings performs OAuth login with settings context
func (tm *TokenManager) AuthenticateWithSettings(profile models.Profile, username, password, apiURL string, insecure bool) (string, int, error) {
	auth := NewAuthenticator(insecure, tm.debug)

	result, err := auth.Authenticate(profile, username, password, apiURL)
	if err != nil {
		return "", 0, err
	}

	return result.Token, result.ExpiresIn, nil
}

// isValidTokenFormat performs basic token format validation
func isValidTokenFormat(token string) bool {
	// Minimum length check
	if len(token) < 20 {
		return false
	}

	// JWT tokens start with "eyJ" (base64 encoded header)
	if len(token) >= 3 && token[:3] == "eyJ" {
		return true
	}

	// Session tokens (e.g., Enterprise Manager) are typically 32-64 chars
	// Accept non-JWT tokens that are at least 32 chars
	return len(token) >= 32
}

// isInteractiveSession detects if running in an interactive terminal
func isInteractiveSession() bool {
	// Check if stdin is a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false
	}

	// Check for common CI/CD environment variables
	ciEnvVars := []string{
		"CI",                     // Generic CI indicator
		"GITHUB_ACTIONS",         // GitHub Actions
		"GITLAB_CI",              // GitLab CI
		"JENKINS_HOME",           // Jenkins
		"CIRCLECI",               // CircleCI
		"TRAVIS",                 // Travis CI
		"TF_BUILD",               // Azure DevOps
		"CODEBUILD_BUILD_ID",     // AWS CodeBuild
		"BITBUCKET_BUILD_NUMBER", // Bitbucket Pipelines
	}

	for _, envVar := range ciEnvVars {
		if os.Getenv(envVar) != "" {
			return false
		}
	}

	return true
}

// IsCI returns true if running in a CI/CD environment
func IsCI() bool {
	return !isInteractiveSession()
}

// In-process token cache to avoid repeated authentication within a single command.
// This is critical on Windows where the WinCred backend may reject large tokens
// (>2560 bytes), causing every API call to re-authenticate.
var (
	processTokenCache     string
	processTokenCacheKey  string
	processTokenExpiresAt time.Time
)

// ClearProcessTokenCache resets the in-process token cache.
// Used when switching instances mid-process (e.g., multi-instance group operations).
func ClearProcessTokenCache() {
	processTokenCache = ""
	processTokenCacheKey = ""
	processTokenExpiresAt = time.Time{}
}

// GetTokenForRequest is a convenience function for API requests that handles
// credential gathering from settings/env and token resolution.
// Returns the token string or an error.
func GetTokenForRequest(profileName string, profile models.Profile, settings models.Settings) (string, error) {
	// Check if explicit token is set (highest priority)
	if token := os.Getenv(TokenEnvVar); token != "" {
		if isValidTokenFormat(token) {
			return token, nil
		}
	}

	// Check in-process cache (avoids repeated keyring opens and re-authentication)
	kcKey := keychainKey(profileName)
	if processTokenCache != "" && processTokenCacheKey == kcKey && time.Now().Before(processTokenExpiresAt) {
		return processTokenCache, nil
	}

	// Get credentials from environment variables
	// Note: With v1.0 profiles, credentials are no longer stored in profiles.json
	username := os.Getenv("OWLCTL_USERNAME")
	apiURL := os.Getenv("OWLCTL_URL")
	password := os.Getenv("OWLCTL_PASSWORD")

	if username == "" || apiURL == "" {
		return "", errors.New("OWLCTL_USERNAME or OWLCTL_URL environment variable not set")
	}

	if password == "" {
		return "", errors.New("OWLCTL_PASSWORD environment variable not set")
	}

	// Create token manager and get token
	tm, err := NewTokenManager(false) // debug=false for API requests
	if err != nil {
		return "", fmt.Errorf("failed to initialize token manager: %w", err)
	}

	// Get token (tries keychain → auto-auth) using instance-aware key
	token, err := tm.GetToken(kcKey, profile, username, password, apiURL, settings.ApiNotSecure)
	if err != nil {
		return "", fmt.Errorf("failed to get authentication token: %w", err)
	}

	// Cache for subsequent calls in this process.
	// Use a conservative 10-minute TTL — VBR tokens expire in 15 minutes,
	// so this provides a safe margin while covering any realistic single-command duration.
	processTokenCache = token
	processTokenCacheKey = kcKey
	processTokenExpiresAt = time.Now().Add(10 * time.Minute)

	return token, nil
}
