package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/99designs/keyring"
	"github.com/shapedthought/owlctl/models"
)

const (
	testHost     = "127.0.0.1"
	testUser     = "admin"
	testPassword = "test-only-pw-xyz" // fake; the mock server accepts exactly this
)

// getTokenFromKeychain returns the stored access token if it has not expired,
// deleting an expired item, mirroring the pre-refresh lookup older tests cover.
func (tm *TokenManager) getTokenFromKeychain(key string) (string, error) {
	info, err := tm.readStored(key)
	if err != nil {
		return "", err
	}
	if time.Now().After(info.ExpiresAt) {
		_ = tm.DeleteToken(key)
		return "", errors.New("token expired")
	}
	return info.Token, nil
}

// fakeJWT builds an unsigned JWT-shaped string with the given exp at test time.
func fakeJWT(exp time.Time, id int) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." +
		enc([]byte(fmt.Sprintf(`{"exp":%d,"jti":%d}`, exp.Unix(), id))) + ".sig"
}

// mockVBR simulates VBR's token and logout endpoints over HTTPS. Refresh
// tokens are single-use and the server records what it is sent.
type mockVBR struct {
	mu            sync.Mutex
	seq           int
	passwordCalls int
	refreshCalls  int
	refreshSeen   []string
	validRefresh  map[string]bool
	logoutCalls   int
	logoutBearer  string
	profile       models.Profile
}

func newMockVBR(t *testing.T) *mockVBR {
	t.Helper()
	m := &mockVBR{validRefresh: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", m.token)
	mux.HandleFunc("/api/oauth2/logout", m.logout)
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(ClearProcessTokenCache)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	m.profile = vbrProfile()
	m.profile.Port = port
	m.profile.Endpoints.Auth = "/token"
	return m
}

// issue returns a fresh access/refresh pair; the caller holds m.mu.
func (m *mockVBR) issue() (access, refresh string) {
	m.seq++
	access = fmt.Sprintf("access-token-%d", m.seq)
	refresh = fakeJWT(time.Now().Add(24*time.Hour), m.seq)
	m.validRefresh[refresh] = true
	return access, refresh
}

func (m *mockVBR) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	m.mu.Lock()
	defer m.mu.Unlock()
	switch r.FormValue("grant_type") {
	case "password":
		if r.FormValue("username") != testUser || r.FormValue("password") != testPassword {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		m.passwordCalls++
	case "refresh_token":
		m.refreshCalls++
		rt := r.FormValue("refresh_token")
		m.refreshSeen = append(m.refreshSeen, rt)
		if !m.validRefresh[rt] {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		delete(m.validRefresh, rt) // single-use
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	access, refresh := m.issue()
	_ = json.NewEncoder(w).Encode(models.SendHeader{
		AccessToken: access, TokenType: "bearer", RefreshToken: refresh, ExpiresIn: 900,
	})
}

func (m *mockVBR) logout(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logoutCalls++
	m.logoutBearer = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	w.WriteHeader(http.StatusNoContent)
}

// calls returns the password and refresh grant counts.
func (m *mockVBR) calls() (password, refresh int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.passwordCalls, m.refreshCalls
}

// newRefreshableRefresh registers a refresh token the server will accept.
func (m *mockVBR) newRefreshToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, rt := m.issue()
	return rt
}

func seedInfo(t *testing.T, kr *mockKeyring, key string, info TokenInfo) {
	t.Helper()
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	kr.store[key] = keyring.Item{Key: key, Data: data}
}

func storedInfo(t *testing.T, kr *mockKeyring, key string) TokenInfo {
	t.Helper()
	item, ok := kr.store[key]
	if !ok {
		t.Fatalf("nothing stored under %q", key)
	}
	var info TokenInfo
	if err := json.Unmarshal(item.Data, &info); err != nil {
		t.Fatal(err)
	}
	return info
}

// expiredSession is a stored session whose access token has expired.
func expiredSession(m *mockVBR, refresh string) TokenInfo {
	return TokenInfo{
		Token:        "stale-access-token",
		IssuedAt:     time.Now().Add(-2 * time.Hour),
		ExpiresAt:    time.Now().Add(-time.Hour),
		RefreshToken: refresh,
		URL:          testHost,
		Port:         m.profile.Port,
	}
}

func TestJWTExpiry(t *testing.T) {
	want := time.Now().Add(time.Hour).Truncate(time.Second)
	got, ok := jwtExpiry(fakeJWT(want, 1))
	if !ok || !got.Equal(want) {
		t.Errorf("jwtExpiry = %v, %v; want %v", got, ok, want)
	}
	for _, bad := range []string{"", "opaque-refresh-token", "a.b.c", "a." + base64.RawURLEncoding.EncodeToString([]byte(`{}`)) + ".c"} {
		if _, ok := jwtExpiry(bad); ok {
			t.Errorf("jwtExpiry(%q) should not yield an expiry", bad)
		}
	}
}

func TestPasswordLoginCarriesRefreshToken(t *testing.T) {
	m := newMockVBR(t)
	res, err := NewAuthenticator(true, false).Authenticate(m.profile, testUser, testPassword, testHost)
	if err != nil {
		t.Fatal(err)
	}
	if res.RefreshToken == "" || res.RefreshExpiresAt.Before(time.Now().Add(23*time.Hour)) {
		t.Errorf("refresh token and expiry not carried: %+v", res.RefreshExpiresAt)
	}
}

// [P2] expired access + valid refresh: exactly one refresh_token grant, the new
// pair is stored, and no password is needed.
func TestGetToken_ExpiredAccessUsesRefreshToken(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)
	old := m.newRefreshToken()
	seedInfo(t, kr, "vbr", expiredSession(m, old))

	token, err := tm.GetToken("vbr", m.profile, "", "", testHost, true)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if pw, rf := m.calls(); pw != 0 || rf != 1 {
		t.Fatalf("password calls = %d, refresh calls = %d; want 0 and 1", pw, rf)
	}
	info := storedInfo(t, kr, "vbr")
	if info.Token != token || info.RefreshToken == "" || info.RefreshToken == old {
		t.Error("the new access/refresh pair was not stored")
	}
	if info.URL != testHost || info.Port != m.profile.Port {
		t.Error("stored item is not bound to the server")
	}
}

// [P2] rotation: the second refresh sends the refresh token returned by the first.
func TestGetToken_SecondRefreshUsesRotatedToken(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)
	first := m.newRefreshToken()
	seedInfo(t, kr, "vbr", expiredSession(m, first))

	if _, err := tm.GetToken("vbr", m.profile, "", "", testHost, true); err != nil {
		t.Fatal(err)
	}
	rotated := storedInfo(t, kr, "vbr").RefreshToken

	// Let the new access token expire and renew again.
	info := storedInfo(t, kr, "vbr")
	info.ExpiresAt = time.Now().Add(-time.Minute)
	seedInfo(t, kr, "vbr", info)
	if _, err := tm.GetToken("vbr", m.profile, "", "", testHost, true); err != nil {
		t.Fatalf("second refresh: %v", err)
	}

	if len(m.refreshSeen) != 2 || m.refreshSeen[0] != first || m.refreshSeen[1] != rotated {
		t.Error("second refresh did not use the rotated refresh token")
	}
	if storedInfo(t, kr, "vbr").RefreshToken == rotated {
		t.Error("second rotation was not stored")
	}
}

func TestGetToken_ValidAccessTokenMakesNoCalls(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)
	info := expiredSession(m, m.newRefreshToken())
	info.Token, info.ExpiresAt = "still-good-token", time.Now().Add(10*time.Minute)
	seedInfo(t, kr, "vbr", info)

	token, err := tm.GetToken("vbr", m.profile, "", "", testHost, true)
	if err != nil || token != "still-good-token" {
		t.Fatalf("got %q, %v", token, err)
	}
	if pw, rf := m.calls(); pw+rf != 0 {
		t.Errorf("unexpected server calls: %d password, %d refresh", pw, rf)
	}
}

func TestGetToken_AccessTokenNearExpiryIsRenewed(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)
	info := expiredSession(m, m.newRefreshToken())
	info.ExpiresAt = time.Now().Add(30 * time.Second) // under the 60s margin
	seedInfo(t, kr, "vbr", info)

	if _, err := tm.GetToken("vbr", m.profile, "", "", testHost, true); err != nil {
		t.Fatal(err)
	}
	if _, rf := m.calls(); rf != 1 {
		t.Errorf("refresh calls = %d, want 1", rf)
	}
}

// [P2] a rejected refresh token falls back to the password when env credentials are set.
func TestGetToken_RejectedRefreshFallsBackToPassword(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)
	seedInfo(t, kr, "vbr", expiredSession(m, fakeJWT(time.Now().Add(time.Hour), 999))) // server never issued it

	token, err := tm.GetToken("vbr", m.profile, testUser, testPassword, testHost, true)
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if pw, rf := m.calls(); pw != 1 || rf != 1 {
		t.Errorf("password calls = %d, refresh calls = %d; want 1 and 1", pw, rf)
	}
	if storedInfo(t, kr, "vbr").Token != token {
		t.Error("password-login session was not stored")
	}
}

// [P2] without credentials a rejected refresh token is a clear "run owlctl login" error.
func TestGetToken_RejectedRefreshWithoutCredentialsErrors(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)
	seedInfo(t, kr, "vbr", expiredSession(m, fakeJWT(time.Now().Add(time.Hour), 999)))

	_, err := tm.GetToken("vbr", m.profile, "", "", testHost, true)
	if err == nil || !strings.Contains(err.Error(), "owlctl login") {
		t.Fatalf("err = %v, want a message telling the user to run owlctl login", err)
	}
	if _, ok := kr.store["vbr"]; ok {
		t.Error("a rejected session should be removed")
	}
	if _, rf := m.calls(); rf != 1 {
		t.Errorf("refresh calls = %d, want 1", rf)
	}
}

// [P2] an expired refresh token is not even sent.
func TestGetToken_ExpiredRefreshTokenNotSent(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)
	info := expiredSession(m, m.newRefreshToken())
	info.RefreshExpiresAt = time.Now().Add(-time.Minute)
	seedInfo(t, kr, "vbr", info)

	_, err := tm.GetToken("vbr", m.profile, "", "", testHost, true)
	if err == nil || !strings.Contains(err.Error(), "owlctl login") {
		t.Fatalf("err = %v, want run-owlctl-login error", err)
	}
	if _, rf := m.calls(); rf != 0 {
		t.Errorf("an expired refresh token must not be sent (refresh calls = %d)", rf)
	}

	if _, err := tm.GetToken("vbr", m.profile, testUser, testPassword, testHost, true); err != nil {
		t.Fatalf("password fallback: %v", err)
	}
}

// [P2] an item stored for server A is never sent to server B.
func TestGetToken_StoredItemNeverUsedForOtherServer(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)

	cases := map[string]TokenInfo{}
	otherHost := expiredSession(m, m.newRefreshToken())
	otherHost.URL = "vbr-a.example.com"
	cases["other host"] = otherHost
	otherPort := expiredSession(m, m.newRefreshToken())
	otherPort.Port = m.profile.Port + 1
	cases["other port"] = otherPort
	legacy := expiredSession(m, m.newRefreshToken()) // predates binding
	legacy.URL, legacy.Port = "", 0
	cases["unbound legacy item"] = legacy
	validAccess := expiredSession(m, m.newRefreshToken())
	validAccess.Token, validAccess.ExpiresAt = "server-a-access-token", time.Now().Add(10*time.Minute)
	validAccess.URL = "vbr-a.example.com"
	cases["other host with a valid access token"] = validAccess

	for name, info := range cases {
		t.Run(name, func(t *testing.T) {
			kr := newMockKeyring()
			tm := newTestTokenManager(kr)
			seedInfo(t, kr, "vbr", info)

			if _, err := tm.GetToken("vbr", m.profile, "", "", testHost, true); err == nil {
				t.Fatal("expected an error: nothing stored belongs to this server")
			}
			token, err := tm.GetToken("vbr", m.profile, testUser, testPassword, testHost, true)
			if err != nil || token == "server-a-access-token" {
				t.Fatalf("got %q, %v; must log in to this server instead", token, err)
			}
			for _, seen := range m.refreshSeen {
				if seen == info.RefreshToken {
					t.Fatal("server A's refresh token was sent to server B")
				}
			}
		})
	}
	if _, rf := m.calls(); rf != 0 {
		t.Errorf("refresh calls = %d, want 0", rf)
	}
}

// Two instances keep separate items under their own keys and servers.
func TestGetToken_TwoInstancesDoNotMix(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)

	a := expiredSession(m, m.newRefreshToken())
	a.Token, a.ExpiresAt, a.URL = "token-for-server-a", time.Now().Add(10*time.Minute), "vbr-a.example.com"
	b := expiredSession(m, m.newRefreshToken())
	b.Token, b.ExpiresAt = "token-for-server-b", time.Now().Add(10*time.Minute)
	seedInfo(t, kr, "instance:a", a)
	seedInfo(t, kr, "instance:b", b)

	t.Setenv("OWLCTL_KEYCHAIN_KEY", "instance:b")
	if got, err := tm.GetToken("vbr", m.profile, "", "", testHost, true); err != nil || got != "token-for-server-b" {
		t.Errorf("instance b: got %q, %v", got, err)
	}
	t.Setenv("OWLCTL_KEYCHAIN_KEY", "instance:a")
	if _, err := tm.GetToken("vbr", m.profile, "", "", testHost, true); err == nil {
		t.Error("instance a's item must not be used against server B")
	}
}

// Nothing the manager writes contains the password.
func TestStoreSession_NeverStoresPassword(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)

	if _, err := tm.GetToken("vbr", m.profile, testUser, testPassword, testHost, true); err != nil {
		t.Fatal(err)
	}
	for k, item := range kr.store {
		if strings.Contains(string(item.Data), testPassword) {
			t.Errorf("item %q contains the password", k)
		}
	}
}

func TestDeleteToken_RemovesRefreshTokenToo(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)
	if _, err := tm.GetToken("vbr", m.profile, testUser, testPassword, testHost, true); err != nil {
		t.Fatal(err)
	}
	if storedInfo(t, kr, "vbr").RefreshToken == "" {
		t.Fatal("setup: no refresh token stored")
	}

	if err := tm.DeleteToken("vbr"); err != nil {
		t.Fatal(err)
	}
	if len(kr.store) != 0 {
		t.Error("stored item (access and refresh token) should be gone")
	}
	if getProcessInfo("vbr") != nil {
		t.Error("in-process copy of the refresh token should be gone")
	}
}

// [P2] logout calls the logout endpoint with the access token and removes the stored item.
func TestLogout_CallsEndpointAndRemovesItem(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)
	m.profile.Product = "vbr"
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)
	token, err := tm.GetToken("vbr", m.profile, testUser, testPassword, testHost, true)
	if err != nil {
		t.Fatal(err)
	}

	removed, err := tm.Logout("vbr", m.profile, testHost, true)
	if err != nil || !removed {
		t.Fatalf("Logout = %v, %v", removed, err)
	}
	if m.logoutCalls != 1 || m.logoutBearer != token {
		t.Errorf("logout calls = %d, bearer matches = %v", m.logoutCalls, m.logoutBearer == token)
	}
	if len(kr.store) != 0 {
		t.Error("stored item should be removed")
	}
}

// An expired access token is renewed first so the logout call revokes the live session.
func TestLogout_RenewsExpiredAccessTokenFirst(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)
	m.profile.Product = "vbr"
	kr := newMockKeyring()
	tm := newTestTokenManager(kr)
	seedInfo(t, kr, "vbr", expiredSession(m, m.newRefreshToken()))

	removed, err := tm.Logout("vbr", m.profile, testHost, true)
	if err != nil || !removed {
		t.Fatalf("Logout = %v, %v", removed, err)
	}
	if _, rf := m.calls(); rf != 1 || m.logoutCalls != 1 {
		t.Errorf("refresh calls = %d, logout calls = %d; want 1 and 1", rf, m.logoutCalls)
	}
	if m.logoutBearer == "stale-access-token" {
		t.Error("logout used the expired token")
	}
	if len(kr.store) != 0 {
		t.Error("stored item should be removed")
	}
}

func TestLogout_NothingStored(t *testing.T) {
	m := newMockVBR(t)
	m.profile.Product = "vbr"
	tm := newTestTokenManager(newMockKeyring())
	removed, err := tm.Logout("vbr", m.profile, testHost, true)
	if err != nil || removed {
		t.Errorf("Logout = %v, %v; want false, nil", removed, err)
	}
	if m.logoutCalls != 0 {
		t.Error("no server call expected when nothing is stored")
	}
}

// ---- GetTokenForRequest / RenewTokenAfter401 -----------------------------------

func requestEnv(t *testing.T, m *mockVBR, password string) models.Settings {
	t.Helper()
	t.Setenv(TokenEnvVar, "")
	t.Setenv("OWLCTL_KEYCHAIN_KEY", "")
	t.Setenv("OWLCTL_URL", testHost)
	t.Setenv("OWLCTL_USERNAME", testUser)
	t.Setenv("OWLCTL_PASSWORD", password)
	t.Cleanup(ClearProcessTokenCache)
	ClearProcessTokenCache()
	return models.Settings{SelectedProfile: "vbr", ApiNotSecure: true}
}

// [P2] a 401 triggers a renewal through the refresh token, in-process even with no keyring.
func TestRenewTokenAfter401_UsesRefreshToken(t *testing.T) {
	m := newMockVBR(t)
	settings := requestEnv(t, m, testPassword)
	t.Setenv(KeyringEnvVar, "none")

	first, err := GetTokenForRequest("vbr", m.profile, settings)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OWLCTL_PASSWORD", "") // the refresh token must be enough

	second, err := RenewTokenAfter401("vbr", m.profile, settings, first)
	if err != nil {
		t.Fatalf("RenewTokenAfter401: %v", err)
	}
	if second == first {
		t.Error("the rejected token was returned again")
	}
	if pw, rf := m.calls(); pw != 1 || rf != 1 {
		t.Errorf("password calls = %d, refresh calls = %d; want 1 and 1", pw, rf)
	}
	// The renewed token is now the cached one.
	if again, _ := GetTokenForRequest("vbr", m.profile, settings); again != second {
		t.Error("renewed token was not cached")
	}
}

func TestRenewTokenAfter401_EnvTokenCannotBeRenewed(t *testing.T) {
	m := newMockVBR(t)
	settings := requestEnv(t, m, testPassword)
	envToken := "eyJ-fake-env-token-built-at-test-time"
	t.Setenv(TokenEnvVar, envToken)

	if _, err := RenewTokenAfter401("vbr", m.profile, settings, envToken); err == nil {
		t.Fatal("an OWLCTL_TOKEN rejected with 401 cannot be renewed")
	}
	if pw, rf := m.calls(); pw+rf != 0 {
		t.Error("no auth call expected")
	}
}

// [P2] after one login, requests keep working in later processes with
// OWLCTL_PASSWORD unset, past the access token's 15 minutes.
func TestGetTokenForRequest_WorksWithoutPasswordAfterAccessTokenExpires(t *testing.T) {
	m := newMockVBR(t)
	settings := requestEnv(t, m, testPassword)
	t.Setenv("HOME", t.TempDir())
	t.Setenv(KeyringEnvVar, "file")
	t.Setenv(FileKeyEnvVar, "unit-test-passphrase")

	// "owlctl login"
	tm, err := NewTokenManager(false)
	if err != nil {
		t.Fatal(err)
	}
	res, err := tm.AuthenticateWithSettings(m.profile, testUser, testPassword, testHost, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := tm.StoreSession("vbr", res, testHost, m.profile.Port); err != nil {
		t.Fatal(err)
	}

	// 20 minutes later, in a new process, with no password in the environment.
	ClearProcessTokenCache()
	t.Setenv("OWLCTL_PASSWORD", "")
	stored, err := tm.readStored("vbr")
	if err != nil {
		t.Fatal(err)
	}
	stored.ExpiresAt = time.Now().Add(-5 * time.Minute)
	data, _ := json.Marshal(stored)
	if err := tm.keyringCall(func(kr keyring.Keyring) error { return kr.Set(keyring.Item{Key: "vbr", Data: data}) }); err != nil {
		t.Fatal(err)
	}

	token, err := GetTokenForRequest("vbr", m.profile, settings)
	if err != nil {
		t.Fatalf("GetTokenForRequest without a password: %v", err)
	}
	if token == "" || token == res.Token {
		t.Error("expected a renewed access token")
	}
	if pw, rf := m.calls(); pw != 1 || rf != 1 {
		t.Errorf("password calls = %d, refresh calls = %d; want 1 and 1", pw, rf)
	}
}

func TestGetTokenForRequest_NoSessionNoPasswordSaysRunLogin(t *testing.T) {
	m := newMockVBR(t)
	settings := requestEnv(t, m, "")
	t.Setenv(KeyringEnvVar, "none")

	_, err := GetTokenForRequest("vbr", m.profile, settings)
	if err == nil || !strings.Contains(err.Error(), "owlctl login") {
		t.Fatalf("err = %v, want a message telling the user to run owlctl login", err)
	}
}

// A stored refresh token must never reach output, including debug output.
func TestRefresh_DebugOutputHasNoSecrets(t *testing.T) {
	t.Setenv(TokenEnvVar, "")
	m := newMockVBR(t)
	kr := newMockKeyring()
	tm := &TokenManager{keyring: kr, debug: true}
	rt := m.newRefreshToken()
	seedInfo(t, kr, "vbr", expiredSession(m, rt))

	var token string
	stderr := captureStderr(t, func() {
		var err error
		token, err = tm.GetToken("vbr", m.profile, "", "", testHost, true)
		if err != nil {
			t.Error(err)
		}
	})
	stored := storedInfo(t, kr, "vbr")
	for _, secret := range []string{rt, token, stored.RefreshToken, "stale-access-token"} {
		if secret != "" && strings.Contains(stderr, secret) {
			t.Error("debug output contains a token")
		}
	}
}
