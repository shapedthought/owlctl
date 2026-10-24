package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/99designs/keyring"
	"github.com/shapedthought/owlctl/models"
	"golang.org/x/term"
)

const (
	KeyringService = "owlctl"
	TokenEnvVar    = "OWLCTL_TOKEN"

	// accessTokenSkew is how much lifetime a stored access token must have left
	// to be used as is; below it the token is renewed first.
	accessTokenSkew = 60 * time.Second

	// expireAccessEnvVar is a test-only hook (see CLAUDE.md): when "1", a stored
	// access token is treated as expired so the refresh path runs. It does not
	// affect OWLCTL_TOKEN.
	expireAccessEnvVar = "OWLCTL_TEST_EXPIRE_ACCESS_TOKEN"
)

// TokenManager handles token storage and retrieval
type TokenManager struct {
	keyring  keyring.Keyring // nil when storage is disabled
	debug    bool
	backend  string
	timeout  time.Duration // 0 = no timeout
	timedOut bool
	hintOnce sync.Once
	// degraded is a short reason when "auto" had to give up on storage (empty
	// when storage works or was disabled explicitly with OWLCTL_KEYRING=none).
	degraded string
}

// Backend returns the OWLCTL_KEYRING-style name of the active backend.
func (tm *TokenManager) Backend() string { return tm.backend }

// Degraded returns why token storage was silently disabled, or "" when it was
// not (storage works, or the user chose OWLCTL_KEYRING=none).
func (tm *TokenManager) Degraded() string { return tm.degraded }

// TokenInfo holds the stored session: the access token, the single-use refresh
// token and the server they were issued by. The password is never stored.
type TokenInfo struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	IssuedAt  time.Time `json:"issued_at"`
	Profile   string    `json:"profile"`

	// RefreshToken is empty for Enterprise Manager sessions. RefreshExpiresAt is
	// zero when the refresh token's expiry is unknown.
	RefreshToken     string    `json:"refresh_token,omitempty"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`

	// URL (hostname) and Port bind the item to the server that issued it; it is
	// never used for a different server. Items without a URL predate binding.
	URL  string `json:"url,omitempty"`
	Port int    `json:"port,omitempty"`
}

// usableRefresh reports whether the stored refresh token may still be tried.
func (i *TokenInfo) usableRefresh(now time.Time) bool {
	return i.RefreshToken != "" && (i.RefreshExpiresAt.IsZero() || now.Before(i.RefreshExpiresAt))
}

// boundTo reports whether the item was issued by the given server. Items that
// predate binding (no URL) only match a lookup with no URL.
func (i *TokenInfo) boundTo(apiURL string, port int) bool {
	if i.URL == "" {
		return apiURL == ""
	}
	return strings.EqualFold(i.URL, apiURL) && i.Port == port
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

// errNoSession is the final error when no step of the resolution order yields a token.
var errNoSession = errors.New("not logged in: run 'owlctl login' (or set OWLCTL_TOKEN, or OWLCTL_USERNAME/OWLCTL_PASSWORD/OWLCTL_URL)")

// GetToken retrieves a token. Order: 1) OWLCTL_TOKEN, 2) stored access token
// with more than 60s left, 3) stored refresh token (grant_type=refresh_token),
// 4) password login with the supplied credentials, 5) an error.
// profileName is used for keychain storage/retrieval.
func (tm *TokenManager) GetToken(profileName string, profile models.Profile, username, password, apiURL string, insecure bool) (string, error) {
	token, _, err := tm.getToken(profileName, profile, username, password, apiURL, insecure, "")
	return token, err
}

// getToken is GetToken plus the token's expiry (zero for OWLCTL_TOKEN). A
// non-empty rejected is an access token the server answered 401 to: it is not
// reused even if it looks valid.
func (tm *TokenManager) getToken(profileName string, profile models.Profile, username, password, apiURL string, insecure bool, rejected string) (string, time.Time, error) {
	if tm.debug {
		fmt.Fprintln(os.Stderr, "DEBUG: Token resolution started")
	}

	// 1. Explicit token env var (highest priority)
	if token := os.Getenv(TokenEnvVar); token != "" {
		if isValidTokenFormat(token) {
			if token == rejected {
				return "", time.Time{}, errors.New("OWLCTL_TOKEN was rejected by the server (401); it cannot be renewed")
			}
			if tm.debug {
				fmt.Fprintln(os.Stderr, "DEBUG: Using token from OWLCTL_TOKEN env var")
			}
			return token, time.Time{}, nil
		}
		if tm.debug {
			fmt.Fprintln(os.Stderr, "DEBUG: OWLCTL_TOKEN token invalid, trying next method")
		}
	}

	return tm.resolveSession(keychainKey(profileName), profile, username, password, apiURL, insecure, rejected)
}

// resolveSession runs steps 2-5 of the resolution order.
func (tm *TokenManager) resolveSession(kcKey string, profile models.Profile, username, password, apiURL string, insecure bool, rejected string) (string, time.Time, error) {
	var refreshErr error

	info := tm.loadInfo(kcKey, apiURL, profile.Port)
	if info != nil {
		now := time.Now()

		// 2. Stored access token
		forced := os.Getenv(expireAccessEnvVar) == "1" // test-only hook, see CLAUDE.md
		if !forced && info.Token != "" && info.Token != rejected && info.ExpiresAt.Sub(now) > accessTokenSkew {
			if tm.debug {
				fmt.Fprintf(os.Stderr, "DEBUG: Using stored access token (expires %s)\n", info.ExpiresAt.Format(time.RFC3339))
			}
			return info.Token, info.ExpiresAt, nil
		}

		// 3. Stored refresh token (not for Enterprise Manager sessions)
		if profile.AuthType != "basic" && apiURL != "" && info.usableRefresh(now) {
			tok, exp, err := tm.refreshLocked(kcKey, profile, info, rejected, apiURL, insecure)
			if err == nil {
				return tok, exp, nil
			}
			refreshErr = err
			if tm.debug {
				fmt.Fprintf(os.Stderr, "DEBUG: Token refresh failed: %v\n", err)
			}
		} else if !forced && !info.usableRefresh(now) {
			// Expired (or rejected) and nothing to renew it with.
			tm.discard(kcKey)
		}
	}

	// 4. Password login
	if username != "" && password != "" && apiURL != "" {
		if tm.debug {
			fmt.Fprintln(os.Stderr, "DEBUG: Auto-authenticating with credentials")
		}
		res, err := tm.AuthenticateWithSettings(profile, username, password, apiURL, insecure)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("auto-authentication failed: %w", err)
		}
		// StoreSession is a no-op when no keyring is active (only with
		// OWLCTL_KEYRING=none).
		exp := tm.storeSession(kcKey, res, apiURL, profile.Port)
		return res.Token, exp, nil
	}

	// 5. Nothing left to try
	if refreshErr != nil {
		return "", time.Time{}, fmt.Errorf("stored session could not be renewed (%v): run 'owlctl login', or set OWLCTL_USERNAME/OWLCTL_PASSWORD", refreshErr)
	}
	return "", time.Time{}, errNoSession
}

// Recheck window after a rejected refresh: the process that rotated the pair
// may not have stored it yet, so the item is polled before the session is
// declared dead. Vars so tests can shorten them.
var (
	rejectRecheckTotal = 1500 * time.Millisecond
	rejectRecheckEvery = 150 * time.Millisecond
)

// sessionChanged reports whether a is a different stored session than b.
func sessionChanged(a, b *TokenInfo) bool {
	return a.RefreshToken != b.RefreshToken || a.Token != b.Token
}

// refreshLocked renews the session with the stored refresh token. Refresh is
// serialised across processes with a lock file; after taking it the item is
// re-read and, if another process already rotated it, that session is used
// without refreshing. A rejected refresh is rechecked for a short while before
// the stored item is deleted, and the delete only happens if the item is still
// exactly the rejected one.
func (tm *TokenManager) refreshLocked(kcKey string, profile models.Profile, info *TokenInfo, rejected, apiURL string, insecure bool) (string, time.Time, error) {
	release := tm.lockRefresh(kcKey)
	defer release()

	cur := info
	if latest := tm.loadInfo(kcKey, apiURL, profile.Port); latest != nil && sessionChanged(latest, info) {
		cur = latest
	}
	deadline := time.Now().Add(rejectRecheckTotal)
	for {
		now := time.Now()
		if sessionChanged(cur, info) {
			if tm.debug {
				fmt.Fprintln(os.Stderr, "DEBUG: Stored session was replaced by another process; using it")
			}
			if cur.Token != "" && cur.Token != rejected && cur.ExpiresAt.Sub(now) > accessTokenSkew {
				return cur.Token, cur.ExpiresAt, nil
			}
		}
		if !cur.usableRefresh(now) {
			return "", time.Time{}, ErrRefreshRejected
		}
		if tm.debug {
			fmt.Fprintln(os.Stderr, "DEBUG: Refreshing access token with stored refresh token")
		}
		res, err := NewAuthenticator(insecure, tm.debug).Refresh(profile, cur.RefreshToken, apiURL)
		if err == nil {
			// Refresh tokens are single-use: persist the new pair straight away.
			return res.Token, tm.storeSession(kcKey, res, apiURL, profile.Port), nil
		}
		if !errors.Is(err, ErrRefreshRejected) {
			return "", time.Time{}, err
		}

		// Rejected: another process may have rotated the pair but not stored it
		// yet. Wait for a different item to appear.
		var next *TokenInfo
		for {
			if latest := tm.loadInfo(kcKey, apiURL, profile.Port); latest != nil && sessionChanged(latest, cur) {
				next = latest
				break
			}
			if !time.Now().Before(deadline) {
				break
			}
			time.Sleep(rejectRecheckEvery)
		}
		if next == nil {
			// Delete only if the item is still exactly the rejected one.
			// A failed read is not confirmation, so nothing is deleted then.
			if tm.keyring == nil {
				tm.discard(kcKey) // clears the in-process copy only
			} else if latest, rerr := tm.readStored(kcKey); rerr == nil && !sessionChanged(latest, cur) {
				tm.discard(kcKey)
			}
			return "", time.Time{}, err
		}
		cur = next
	}
}

// StoreToken stores an access token with no refresh token or server binding.
// Prefer StoreSession; this form remains for callers that hold only a token.
func (tm *TokenManager) StoreToken(profileName, token string, expiresIn int) error {
	return tm.StoreSession(profileName, &AuthResult{Token: token, ExpiresIn: expiresIn}, "", 0)
}

// StoreSession persists a login result (access and refresh token) bound to the
// server it came from. It also keeps the session in process memory so a renewal
// later in the same process works even when no keyring is active.
func (tm *TokenManager) StoreSession(profileName string, res *AuthResult, apiURL string, port int) error {
	now := time.Now()
	info := TokenInfo{
		Token:            res.Token,
		IssuedAt:         now,
		ExpiresAt:        now.Add(time.Duration(res.ExpiresIn) * time.Second),
		Profile:          profileName,
		RefreshToken:     res.RefreshToken,
		RefreshExpiresAt: res.RefreshExpiresAt,
		URL:              apiURL,
		Port:             port,
	}
	setProcessInfo(profileName, &info)

	if tm.keyring == nil {
		return nil // storage disabled: nothing to persist
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

// storeSession is StoreSession for the resolution path: a failure to persist is
// non-fatal (the caller still has a working token). It returns the access
// token's expiry.
func (tm *TokenManager) storeSession(key string, res *AuthResult, apiURL string, port int) time.Time {
	if err := tm.StoreSession(key, res, apiURL, port); err != nil && tm.debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Failed to store token in keychain: %v\n", err)
	}
	return time.Now().Add(time.Duration(res.ExpiresIn) * time.Second)
}

// readStored reads the stored item without any validity or server checks.
func (tm *TokenManager) readStored(key string) (*TokenInfo, error) {
	item, err := tm.keyringGet(key)
	if err != nil {
		return nil, fmt.Errorf("token not found in keychain: %w", err)
	}
	var info TokenInfo
	if err := json.Unmarshal(item.Data, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token info: %w", err)
	}
	return &info, nil
}

// loadInfo returns the stored session for key if it was issued by the given
// server, else nil. The keyring is authoritative (another process may have
// rotated the refresh token); the in-process copy covers sessions that were
// never persisted. An item for a different server is ignored and left alone.
func (tm *TokenManager) loadInfo(key, apiURL string, port int) *TokenInfo {
	info, err := tm.readStored(key)
	if err != nil {
		if tm.debug && tm.keyring != nil {
			fmt.Fprintf(os.Stderr, "DEBUG: Keychain lookup failed: %v\n", err)
		}
		// With a working keyring a miss is a miss; memory only stands in when
		// nothing could be persisted or read.
		if tm.keyring == nil || tm.timedOut {
			info = getProcessInfo(key)
		}
	}
	if info == nil {
		return nil
	}
	if !info.boundTo(apiURL, port) {
		if tm.debug {
			fmt.Fprintln(os.Stderr, "DEBUG: Ignoring stored token issued for a different server")
		}
		return nil
	}
	return info
}

// discard removes the stored session for key (best effort).
func (tm *TokenManager) discard(key string) {
	_ = tm.DeleteToken(key)
}

// DeleteToken removes the stored session (access and refresh token) for the key.
func (tm *TokenManager) DeleteToken(profileName string) error {
	clearProcessInfo(profileName)
	if tm.keyring == nil {
		return nil // nothing is stored when storage is disabled
	}
	return tm.keyringCall(func(kr keyring.Keyring) error { return kr.Remove(profileName) })
}

// Logout ends the session for profileName: it revokes it on the server (VBR
// only; renewing the access token first if needed so the refresh token is
// revoked too) and then deletes the stored item. The local item is removed
// even when the server call fails, in which case the error is returned.
// removed is false when nothing was stored for this server.
func (tm *TokenManager) Logout(profileName string, profile models.Profile, apiURL string, insecure bool) (removed bool, err error) {
	key := keychainKey(profileName)
	info := tm.loadInfo(key, apiURL, profile.Port)
	if info == nil {
		return false, nil
	}

	var remoteErr error
	if profile.Product == "vbr" {
		// No credentials are passed: a renewal here may only use the refresh token.
		token, _, rerr := tm.resolveSession(key, profile, "", "", apiURL, insecure, "")
		if rerr != nil {
			// The session is already dead server-side (or unreachable).
			remoteErr = fmt.Errorf("could not obtain an access token to revoke the session: %w", rerr)
		} else if lerr := NewAuthenticator(insecure, tm.debug).Logout(profile, token, apiURL); lerr != nil {
			remoteErr = lerr
		}
	}

	derr := tm.DeleteToken(key)
	removeLockFile(key)
	if derr != nil && remoteErr == nil {
		remoteErr = fmt.Errorf("failed to remove stored token: %w", derr)
	}
	return true, remoteErr
}

// AuthenticateWithSettings performs OAuth login with settings context
func (tm *TokenManager) AuthenticateWithSettings(profile models.Profile, username, password, apiURL string, insecure bool) (*AuthResult, error) {
	return NewAuthenticator(insecure, tm.debug).Authenticate(profile, username, password, apiURL)
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

// In-process session state. It avoids repeated authentication within a single
// command (critical on Windows where the WinCred backend may reject large
// tokens, >2560 bytes, so every call would re-authenticate) and lets a 401
// renewal use the refresh token even when no keyring is active. It lives only
// in memory.
var (
	processTokenCache     string
	processTokenCacheKey  string
	processTokenExpiresAt time.Time

	processInfoKey string
	processInfoVal *TokenInfo
)

func setProcessInfo(key string, info *TokenInfo) {
	processInfoKey = key
	cp := *info
	processInfoVal = &cp
}

func getProcessInfo(key string) *TokenInfo {
	if processInfoVal == nil || processInfoKey != key {
		return nil
	}
	cp := *processInfoVal
	return &cp
}

func clearProcessInfo(key string) {
	if processInfoKey == key {
		processInfoKey = ""
		processInfoVal = nil
	}
	if processTokenCacheKey == key {
		processTokenCache = ""
		processTokenCacheKey = ""
		processTokenExpiresAt = time.Time{}
	}
}

// ClearProcessTokenCache resets the in-process token cache and session.
// Used when switching instances mid-process (e.g., multi-instance group operations).
func ClearProcessTokenCache() {
	processTokenCache = ""
	processTokenCacheKey = ""
	processTokenExpiresAt = time.Time{}
	processInfoKey = ""
	processInfoVal = nil
}

// GetTokenForRequest is a convenience function for API requests that handles
// credential gathering from settings/env and token resolution.
// Returns the token string or an error. OWLCTL_PASSWORD is only needed when
// no stored session can supply or renew a token.
func GetTokenForRequest(profileName string, profile models.Profile, settings models.Settings) (string, error) {
	return tokenForRequest(profileName, profile, settings, "")
}

// RenewTokenAfter401 returns a fresh token after the server answered 401 to
// rejected: it renews through the refresh token, then the password. Callers
// retry the request once with the result and never again.
func RenewTokenAfter401(profileName string, profile models.Profile, settings models.Settings, rejected string) (string, error) {
	return tokenForRequest(profileName, profile, settings, rejected)
}

func tokenForRequest(profileName string, profile models.Profile, settings models.Settings, rejected string) (string, error) {
	// Check if explicit token is set (highest priority)
	if token := os.Getenv(TokenEnvVar); token != "" {
		if isValidTokenFormat(token) {
			if token == rejected {
				return "", errors.New("OWLCTL_TOKEN was rejected by the server (401); it cannot be renewed")
			}
			return token, nil
		}
	}

	// Check in-process cache (avoids repeated keyring opens and re-authentication)
	kcKey := keychainKey(profileName)
	if processTokenCache != "" && processTokenCache != rejected && processTokenCacheKey == kcKey && time.Now().Before(processTokenExpiresAt) {
		return processTokenCache, nil
	}

	// Credentials come from environment variables (profiles.json holds none).
	username := os.Getenv("OWLCTL_USERNAME")
	apiURL := os.Getenv("OWLCTL_URL")
	password := os.Getenv("OWLCTL_PASSWORD")

	if apiURL == "" {
		return "", errors.New("OWLCTL_URL environment variable not set")
	}

	tm, err := NewTokenManager(false) // debug=false for API requests
	if err != nil {
		return "", fmt.Errorf("failed to initialize token manager: %w", err)
	}

	// Resolves env token → stored token → refresh → password using the instance-aware key.
	token, expiresAt, err := tm.getToken(kcKey, profile, username, password, apiURL, settings.ApiNotSecure, rejected)
	if err != nil {
		return "", fmt.Errorf("failed to get authentication token: %w", err)
	}

	// Cache for subsequent calls in this process: at most 10 minutes, and never
	// past the point where the token would be renewed anyway.
	ttl := 10 * time.Minute
	if !expiresAt.IsZero() {
		if left := time.Until(expiresAt) - accessTokenSkew; left < ttl {
			ttl = left
		}
	}
	if ttl > 0 {
		processTokenCache = token
		processTokenCacheKey = kcKey
		processTokenExpiresAt = time.Now().Add(ttl)
	}

	return token, nil
}
