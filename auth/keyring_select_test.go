package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/99designs/keyring"
	"github.com/shapedthought/owlctl/models"
)

// blockingKeyring never returns from Get/Set/Remove, like a locked keychain
// waiting on a dialog nobody can answer.
type blockingKeyring struct{ release chan struct{} }

func (b *blockingKeyring) Get(string) (keyring.Item, error) {
	<-b.release
	return keyring.Item{}, keyring.ErrKeyNotFound
}
func (b *blockingKeyring) GetMetadata(string) (keyring.Metadata, error) {
	<-b.release
	return keyring.Metadata{}, nil
}
func (b *blockingKeyring) Set(keyring.Item) error { <-b.release; return nil }
func (b *blockingKeyring) Remove(string) error    { <-b.release; return nil }
func (b *blockingKeyring) Keys() ([]string, error) {
	<-b.release
	return nil, nil
}

// captureStderr runs fn and returns what it wrote to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	fn()
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

// ---- chooseKeyring -------------------------------------------------------------

func TestChooseKeyring(t *testing.T) {
	tests := []struct {
		name    string
		env     keyringEnv
		want    string
		wantErr string
	}{
		{"none explicit, interactive", keyringEnv{setting: "none", interactive: true, goos: "darwin"}, "none", ""},
		{"default non-interactive acts as none", keyringEnv{interactive: false, goos: "darwin"}, "none", ""},
		{"auto non-interactive acts as none", keyringEnv{setting: "auto", interactive: false, goos: "linux", hasDBus: true}, "none", ""},
		{"auto darwin interactive", keyringEnv{interactive: true, goos: "darwin"}, "keychain", ""},
		{"auto windows interactive", keyringEnv{interactive: true, goos: "windows"}, "wincred", ""},
		{"auto linux with dbus", keyringEnv{interactive: true, goos: "linux", hasDBus: true}, "secret-service", ""},
		{"auto linux without dbus", keyringEnv{interactive: true, goos: "linux"}, "file", ""},
		{"explicit keychain non-interactive honoured", keyringEnv{setting: "keychain", goos: "darwin"}, "keychain", ""},
		{"explicit secret-service", keyringEnv{setting: "secret-service", goos: "linux"}, "secret-service", ""},
		{"explicit wincred", keyringEnv{setting: "wincred", goos: "windows"}, "wincred", ""},
		{"file with key, no TTY", keyringEnv{setting: "file", hasFileKey: true}, "file", ""},
		{"file with TTY, no key", keyringEnv{setting: "file", stdinTTY: true}, "file", ""},
		{"file with no TTY and no key errors", keyringEnv{setting: "file"}, "", "OWLCTL_FILE_KEY"},
		{"unknown value errors", keyringEnv{setting: "bogus"}, "", "invalid OWLCTL_KEYRING"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := chooseKeyring(tt.env)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.name != tt.want {
				t.Errorf("name = %q, want %q", got.name, tt.want)
			}
			if (tt.want == "none") != (got.backend == "") {
				t.Errorf("backend %q inconsistent with name %q", got.backend, got.name)
			}
		})
	}
}

// ---- NewTokenManager -----------------------------------------------------------

func TestNewTokenManager_NoneNeverOpensKeyring(t *testing.T) {
	t.Setenv(KeyringEnvVar, "none")
	t.Setenv("HOME", t.TempDir())
	tm, err := NewTokenManager(false)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	if tm.keyring != nil || tm.StorageEnabled() {
		t.Fatal("OWLCTL_KEYRING=none must not open a keyring")
	}
	if err := tm.StoreToken("vbr", "tok", 900); err != nil {
		t.Errorf("StoreToken with storage disabled should be a no-op, got %v", err)
	}
	if err := tm.DeleteToken("vbr"); err != nil {
		t.Errorf("DeleteToken with storage disabled should be a no-op, got %v", err)
	}
	if _, err := tm.getTokenFromKeychain("vbr"); err == nil {
		t.Error("expected a miss with storage disabled")
	}
}

func TestNewTokenManager_NonInteractiveDefaultIsNone(t *testing.T) {
	t.Setenv(KeyringEnvVar, "")
	t.Setenv("CI", "1")
	tm, err := NewTokenManager(false)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	if tm.keyring != nil {
		t.Fatal("CI session must not open a keyring by default")
	}
}

func TestNewTokenManager_FileWithoutKeyOrTTYErrorsImmediately(t *testing.T) {
	t.Setenv(KeyringEnvVar, "file")
	t.Setenv(FileKeyEnvVar, "")
	t.Setenv("HOME", t.TempDir())
	// `go test` runs with stdin not attached to a terminal.
	start := time.Now()
	_, err := NewTokenManager(false)
	if err == nil || !strings.Contains(err.Error(), FileKeyEnvVar) {
		t.Fatalf("err = %v, want a message naming %s", err, FileKeyEnvVar)
	}
	if time.Since(start) > time.Second {
		t.Error("error should be immediate, not after a prompt/timeout")
	}
}

func TestNewTokenManager_FileWithKeyOpens(t *testing.T) {
	t.Setenv(KeyringEnvVar, "file")
	t.Setenv(FileKeyEnvVar, "unit-test-passphrase")
	t.Setenv("HOME", t.TempDir())
	tm, err := NewTokenManager(false)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	if tm.keyring == nil {
		t.Fatal("expected file keyring to be open")
	}
	if err := tm.StoreToken("vbr", "stored-token-value", 900); err != nil {
		t.Fatalf("StoreToken: %v", err)
	}
	got, err := tm.getTokenFromKeychain("vbr")
	if err != nil || got != "stored-token-value" {
		t.Errorf("round trip failed: %q, %v", got, err)
	}
}

func TestNewTokenManager_InvalidValue(t *testing.T) {
	t.Setenv(KeyringEnvVar, "bogus")
	_, err := NewTokenManager(false)
	if err == nil || !strings.Contains(err.Error(), "invalid OWLCTL_KEYRING") {
		t.Fatalf("err = %v, want invalid OWLCTL_KEYRING message", err)
	}
}

// ---- timeout -------------------------------------------------------------------

func TestKeyringTimeout_Parsing(t *testing.T) {
	tests := []struct {
		val  string
		want time.Duration
	}{
		{"", 5 * time.Second},
		{"250ms", 250 * time.Millisecond},
		{"10", 10 * time.Second},
		{"1.5", 1500 * time.Millisecond},
		{"0", 5 * time.Second},
		{"-3s", 5 * time.Second},
		{"junk", 5 * time.Second},
	}
	for _, tt := range tests {
		t.Setenv(KeyringTimeoutEnvVar, tt.val)
		if got := keyringTimeout(); got != tt.want {
			t.Errorf("keyringTimeout(%q) = %v, want %v", tt.val, got, tt.want)
		}
	}
}

// newTLSOAuthServer simulates VBR's token endpoint over HTTPS and returns the
// server plus a profile pointing at its port. Credentials are admin/pass.
func newTLSOAuthServer(t *testing.T) (*httptest.Server, models.Profile) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("username") != "admin" || r.FormValue("password") != "pass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(models.SendHeader{
			AccessToken: "mock-oauth-token-for-timeout-test",
			TokenType:   "Bearer",
			ExpiresIn:   900,
		})
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	p := vbrProfile()
	p.Port = port
	p.Endpoints.Auth = "/token"
	return srv, p
}

// Covers #209: a keyring that blocks forever must time out, print one hint
// naming OWLCTL_TOKEN and OWLCTL_KEYRING=none, and still authenticate from env.
func TestGetToken_BlockingKeyringTimesOutAndAuthenticates(t *testing.T) {
	_, profile := newTLSOAuthServer(t)
	t.Setenv(TokenEnvVar, "")

	bk := &blockingKeyring{release: make(chan struct{})}
	t.Cleanup(func() { close(bk.release) })
	tm := &TokenManager{keyring: bk, backend: "keychain", timeout: 100 * time.Millisecond}

	var token string
	var err error
	start := time.Now()
	stderr := captureStderr(t, func() {
		token, err = tm.GetToken("vbr", profile, "admin", "pass", "127.0.0.1", true)
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("GetToken should authenticate from env after timeout, got %v", err)
	}
	if token != "mock-oauth-token-for-timeout-test" {
		t.Errorf("unexpected token result")
	}
	if elapsed > 3*time.Second {
		t.Errorf("took %v; keyring timeout not honoured", elapsed)
	}
	if n := strings.Count(stderr, "keyring did not respond"); n != 1 {
		t.Errorf("expected exactly one hint line, got %d: %q", n, stderr)
	}
	if !strings.Contains(stderr, "OWLCTL_TOKEN") || !strings.Contains(stderr, "OWLCTL_KEYRING=none") {
		t.Errorf("hint must name OWLCTL_TOKEN and OWLCTL_KEYRING=none: %q", stderr)
	}
	if strings.Contains(stderr, token) {
		t.Error("stderr must not contain secrets")
	}
}

func TestKeyringCall_AllOpsTimeOutWithSingleHint(t *testing.T) {
	bk := &blockingKeyring{release: make(chan struct{})}
	t.Cleanup(func() { close(bk.release) })
	tm := &TokenManager{keyring: bk, timeout: 50 * time.Millisecond}

	stderr := captureStderr(t, func() {
		if _, err := tm.keyringGet("k"); err != errKeyringTimeout {
			t.Errorf("Get err = %v, want timeout", err)
		}
		if err := tm.StoreToken("k", "t", 900); err == nil {
			t.Error("Set should fail after timeout")
		}
		if err := tm.DeleteToken("k"); err == nil {
			t.Error("Remove should fail after timeout")
		}
	})
	if n := strings.Count(stderr, "keyring did not respond"); n != 1 {
		t.Errorf("expected one hint across Get/Set/Remove, got %d", n)
	}
}

func TestKeyringCall_FastBackendUnaffected(t *testing.T) {
	kr := newMockKeyring()
	tm := &TokenManager{keyring: kr, timeout: time.Second}
	if err := tm.StoreToken("vbr", "value-abc", 900); err != nil {
		t.Fatal(err)
	}
	if got, err := tm.getTokenFromKeychain("vbr"); err != nil || got != "value-abc" {
		t.Errorf("got %q, %v", got, err)
	}
}
