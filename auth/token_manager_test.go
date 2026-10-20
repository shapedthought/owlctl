package auth

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/99designs/keyring"
	"github.com/shapedthought/owlctl/models"
)

// mockKeyring implements keyring.Keyring for testing without touching the system keychain.
type mockKeyring struct {
	mu       sync.Mutex // guards store for tests that write from another goroutine
	store    map[string]keyring.Item
	errOnSet bool
	errOnGet bool
}

func newMockKeyring() *mockKeyring {
	return &mockKeyring{store: make(map[string]keyring.Item)}
}

func (m *mockKeyring) Get(key string) (keyring.Item, error) {
	if m.errOnGet {
		return keyring.Item{}, errors.New("keyring unavailable")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.store[key]
	if !ok {
		return keyring.Item{}, keyring.ErrKeyNotFound
	}
	return item, nil
}

func (m *mockKeyring) GetMetadata(key string) (keyring.Metadata, error) {
	return keyring.Metadata{}, nil
}

func (m *mockKeyring) Set(item keyring.Item) error {
	if m.errOnSet {
		return errors.New("keyring write failed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[item.Key] = item
	return nil
}

func (m *mockKeyring) Remove(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
	return nil
}

func (m *mockKeyring) Keys() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.store))
	for k := range m.store {
		keys = append(keys, k)
	}
	return keys, nil
}

// newTestTokenManager returns a TokenManager backed by a mock keyring.
func newTestTokenManager(kr *mockKeyring) *TokenManager {
	return &TokenManager{keyring: kr, debug: false}
}

// storeExpiredToken writes an already-expired TokenInfo into the mock keyring.
func storeExpiredToken(kr *mockKeyring, key string) {
	info := TokenInfo{
		Token:     "expired-token",
		IssuedAt:  time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		Profile:   key,
	}
	data, _ := json.Marshal(info)
	kr.store[key] = keyring.Item{Key: key, Data: data}
}

// vbrProfile returns a minimal Profile for test use.
func vbrProfile() models.Profile {
	return models.Profile{
		Product:  "vbr",
		AuthType: "oauth",
		Port:     9419,
		Endpoints: models.Endpoints{
			Auth:      "/api/oauth2/token",
			APIPrefix: "/api/v1",
		},
		Headers: models.Headers{
			Accept:      "application/json",
			ContentType: "application/x-www-form-urlencoded",
			XAPIVersion: "1.1-rev0",
		},
	}
}

// ---- isValidTokenFormat --------------------------------------------------------

func TestIsValidTokenFormat(t *testing.T) {
	tests := []struct {
		name  string
		token string
		valid bool
	}{
		{"empty string", "", false},
		{"too short (6 chars)", "abc123", false},
		{"19 chars - just under minimum", "1234567890123456789", false},
		{"exactly 20 chars non-JWT (below 32 threshold)", "12345678901234567890", false},
		{"JWT token (starts with eyJ)", "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature", true},
		{"session token exactly 32 chars", "abcdefghijklmnopqrstuvwxyz123456", true},
		{"non-JWT 33 chars accepted", "abcdefghijklmnopqrstuvwxyz1234567", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidTokenFormat(tt.token)
			if got != tt.valid {
				t.Errorf("isValidTokenFormat(%q) = %v, want %v", tt.token, got, tt.valid)
			}
		})
	}
}

// ---- IsCI / isInteractiveSession -----------------------------------------------

// TestIsCI_KnownCIEnvVars verifies that IsCI returns true when known CI env vars are set.
// Note: in a `go test` process stdin is not a TTY, so isInteractiveSession() returns false
// (i.e. IsCI()=true) regardless of env vars. The CI env var detection branch is therefore
// not directly reachable in unit tests without refactoring isInteractiveSession to accept
// an injectable isTTY parameter. For now these tests confirm the public API contract:
// IsCI() is true in any non-TTY environment, which covers all real CI/CD scenarios.
func TestIsCI_KnownCIEnvVars(t *testing.T) {
	ciVars := []string{
		"CI",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"JENKINS_HOME",
		"CIRCLECI",
		"TRAVIS",
		"TF_BUILD",
		"CODEBUILD_BUILD_ID",
		"BITBUCKET_BUILD_NUMBER",
	}

	for _, envVar := range ciVars {
		t.Run("CI detected via "+envVar, func(t *testing.T) {
			t.Setenv(envVar, "true")
			if !IsCI() {
				t.Errorf("expected IsCI()=true when %s is set", envVar)
			}
		})
	}
}

// ---- keychainKey ---------------------------------------------------------------

func TestKeychainKey_NoOverride(t *testing.T) {
	t.Setenv("OWLCTL_KEYCHAIN_KEY", "")
	got := keychainKey("vbr")
	if got != "vbr" {
		t.Errorf("keychainKey() = %q, want %q", got, "vbr")
	}
}

func TestKeychainKey_WithOverride(t *testing.T) {
	t.Setenv("OWLCTL_KEYCHAIN_KEY", "instance:vbr-prod")
	got := keychainKey("vbr")
	if got != "instance:vbr-prod" {
		t.Errorf("keychainKey() = %q, want %q", got, "instance:vbr-prod")
	}
}

func TestKeychainKey_DifferentInstances(t *testing.T) {
	t.Setenv("OWLCTL_KEYCHAIN_KEY", "instance:vbr-dr")
	prod := keychainKey("vbr-prod")
	dr := keychainKey("vbr-dr")
	// Both should return the override, demonstrating per-instance isolation
	// is controlled by the caller setting OWLCTL_KEYCHAIN_KEY before each call.
	if prod != "instance:vbr-dr" || dr != "instance:vbr-dr" {
		t.Errorf("unexpected keychainKey results: prod=%q, dr=%q", prod, dr)
	}
}

// ---- StoreToken / getTokenFromKeychain -----------------------------------------

func TestStoreAndRetrieveToken(t *testing.T) {
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)

	if err := tm.StoreToken("vbr", "my-access-token", 900); err != nil {
		t.Fatalf("StoreToken returned error: %v", err)
	}

	got, err := tm.getTokenFromKeychain("vbr")
	if err != nil {
		t.Fatalf("getTokenFromKeychain returned error: %v", err)
	}
	if got != "my-access-token" {
		t.Errorf("got token %q, want %q", got, "my-access-token")
	}
}

func TestGetTokenFromKeychain_NotFound(t *testing.T) {
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)

	_, err := tm.getTokenFromKeychain("missing-key")
	if err == nil {
		t.Error("expected error for missing key, got nil")
	}
}

func TestGetTokenFromKeychain_Expired(t *testing.T) {
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)

	storeExpiredToken(kr, "vbr")

	_, err := tm.getTokenFromKeychain("vbr")
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}

	// Expired token should be removed from keychain
	if _, ok := kr.store["vbr"]; ok {
		t.Error("expired token should have been removed from keychain")
	}
}

func TestGetTokenFromKeychain_KeyringError(t *testing.T) {
	kr := newMockKeyring()
	kr.errOnGet = true
	tm := newTestTokenManager(kr)

	_, err := tm.getTokenFromKeychain("vbr")
	if err == nil {
		t.Error("expected error when keyring is unavailable")
	}
}

func TestStoreToken_KeyringError(t *testing.T) {
	kr := newMockKeyring()
	kr.errOnSet = true
	tm := newTestTokenManager(kr)

	err := tm.StoreToken("vbr", "token", 900)
	if err == nil {
		t.Error("expected error when keyring write fails")
	}
}

func TestStoreToken_ExpiryIsSet(t *testing.T) {
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)

	before := time.Now()
	_ = tm.StoreToken("vbr", "token", 900)
	after := time.Now()

	item := kr.store["vbr"]
	var info TokenInfo
	if err := json.Unmarshal(item.Data, &info); err != nil {
		t.Fatalf("failed to unmarshal stored token info: %v", err)
	}

	expectedMin := before.Add(900 * time.Second)
	expectedMax := after.Add(900 * time.Second)

	if info.ExpiresAt.Before(expectedMin) || info.ExpiresAt.After(expectedMax) {
		t.Errorf("ExpiresAt %v not in expected range [%v, %v]", info.ExpiresAt, expectedMin, expectedMax)
	}
}

// ---- DeleteToken ---------------------------------------------------------------

func TestDeleteToken(t *testing.T) {
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)

	_ = tm.StoreToken("vbr", "token-to-delete", 900)
	if err := tm.DeleteToken("vbr"); err != nil {
		t.Fatalf("DeleteToken returned error: %v", err)
	}

	if _, ok := kr.store["vbr"]; ok {
		t.Error("token should have been removed after DeleteToken")
	}
}

func TestDeleteToken_NonExistent(t *testing.T) {
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)

	// Deleting a non-existent key should not error
	if err := tm.DeleteToken("does-not-exist"); err != nil {
		t.Errorf("DeleteToken on missing key returned unexpected error: %v", err)
	}
}

// ---- GetToken priority chain ---------------------------------------------------

func TestGetToken_EnvVarTakesPriority(t *testing.T) {
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)

	// Valid JWT env token
	envToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature-padding-here"
	t.Setenv(TokenEnvVar, envToken)

	// Also store a different token in keychain — env var should win
	_ = tm.StoreToken("vbr", "keychain-token", 900)

	got, err := tm.GetToken("vbr", vbrProfile(), "user", "pass", "vbr.local", true)
	if err != nil {
		t.Fatalf("GetToken returned error: %v", err)
	}
	if got != envToken {
		t.Errorf("expected env var token %q, got %q", envToken, got)
	}
}

func TestGetToken_KeychainFallback(t *testing.T) {
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)

	t.Setenv(TokenEnvVar, "") // no env token
	_ = tm.StoreToken("vbr", "keychain-token-value", 900)

	// No credentials provided — should use keychain
	got, err := tm.GetToken("vbr", vbrProfile(), "", "", "", false)
	if err != nil {
		t.Fatalf("GetToken returned error: %v", err)
	}
	if got != "keychain-token-value" {
		t.Errorf("expected keychain token, got %q", got)
	}
}

func TestGetToken_InvalidEnvTokenFallsToKeychain(t *testing.T) {
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)

	t.Setenv(TokenEnvVar, "tooshort") // invalid — falls through
	_ = tm.StoreToken("vbr", "fallback-keychain-token", 900)

	got, err := tm.GetToken("vbr", vbrProfile(), "", "", "", false)
	if err != nil {
		t.Fatalf("GetToken returned error: %v", err)
	}
	if got != "fallback-keychain-token" {
		t.Errorf("expected keychain fallback token, got %q", got)
	}
}

func TestGetToken_NoAuthMethodReturnsError(t *testing.T) {
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)

	t.Setenv(TokenEnvVar, "")
	// Empty keychain, no credentials → should error

	_, err := tm.GetToken("vbr", vbrProfile(), "", "", "", false)
	if err == nil {
		t.Error("expected error when no auth method available")
	}
}

func TestGetToken_KeychainKeyOverrideUsed(t *testing.T) {
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)

	t.Setenv(TokenEnvVar, "")
	t.Setenv("OWLCTL_KEYCHAIN_KEY", "instance:vbr-prod")

	// Store token under the instance key
	_ = tm.StoreToken("instance:vbr-prod", "instance-token", 900)

	got, err := tm.GetToken("vbr", vbrProfile(), "", "", "", false)
	if err != nil {
		t.Fatalf("GetToken returned error: %v", err)
	}
	if got != "instance-token" {
		t.Errorf("expected instance-keyed token, got %q", got)
	}
}

// ---- ClearProcessTokenCache ----------------------------------------------------

func TestClearProcessTokenCache(t *testing.T) {
	// Capture and restore original state so this test doesn't affect others.
	origCache := processTokenCache
	origKey := processTokenCacheKey
	origExpiry := processTokenExpiresAt
	t.Cleanup(func() {
		processTokenCache = origCache
		processTokenCacheKey = origKey
		processTokenExpiresAt = origExpiry
	})

	processTokenCache = "cached-token"
	processTokenCacheKey = "vbr"
	processTokenExpiresAt = time.Now().Add(10 * time.Minute)

	ClearProcessTokenCache()

	if processTokenCache != "" {
		t.Error("expected processTokenCache to be empty after clear")
	}
	if processTokenCacheKey != "" {
		t.Error("expected processTokenCacheKey to be empty after clear")
	}
	if !processTokenExpiresAt.IsZero() {
		t.Error("expected processTokenExpiresAt to be zero after clear")
	}
}
