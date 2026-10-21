package vhttp

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/shapedthought/owlctl/auth"
	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/utils"
)

// vbrStub is a TLS server with VBR-like token and API endpoints. Fake token
// values are generated at test time.
type vbrStub struct {
	mu            sync.Mutex
	passwordCalls int
	refreshCalls  int
	apiCalls      int
	apiBodies     []string
	// apiAccepts decides whether the API accepts a bearer token.
	apiAccepts func(token string) bool
	profile    models.Profile
}

func newVBRStub(t *testing.T) *vbrStub {
	t.Helper()
	s := &vbrStub{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		s.mu.Lock()
		defer s.mu.Unlock()
		access, refresh := "access-from-password", "refresh-1"
		switch r.FormValue("grant_type") {
		case "password":
			s.passwordCalls++
		case "refresh_token":
			s.refreshCalls++
			if r.FormValue("refresh_token") != "refresh-1" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			access, refresh = "access-from-refresh", "refresh-2"
		}
		_ = json.NewEncoder(w).Encode(models.SendHeader{AccessToken: access, RefreshToken: refresh, TokenType: "bearer", ExpiresIn: 900})
	})
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.apiCalls++
		s.apiBodies = append(s.apiBodies, string(body))
		if !s.apiAccepts(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	host, portStr, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	s.profile = models.Profile{
		Product:   "vbr",
		AuthType:  "oauth",
		Port:      port,
		Endpoints: models.Endpoints{Auth: "/api/oauth2/token", APIPrefix: "/api/v1"},
		Headers:   models.Headers{Accept: "application/json", ContentType: "application/x-www-form-urlencoded", XAPIVersion: "1.1-rev0"},
	}

	t.Setenv("OWLCTL_URL", host)
	t.Setenv("OWLCTL_USERNAME", "admin")
	t.Setenv("OWLCTL_PASSWORD", "test-only-pw")
	t.Setenv("OWLCTL_TOKEN", "")
	t.Setenv("OWLCTL_KEYCHAIN_KEY", "")
	t.Setenv("OWLCTL_KEYRING", "none")
	utils.OverrideSettings(models.Settings{SelectedProfile: "vbr", ApiNotSecure: true})
	t.Cleanup(utils.ClearSettingsOverride)
	auth.ClearProcessTokenCache()
	t.Cleanup(auth.ClearProcessTokenCache)
	return s
}

// [P2] a 401 triggers exactly one refresh and one retry.
func TestDoAuthed_401RefreshesOnceAndRetriesOnce(t *testing.T) {
	s := newVBRStub(t)
	s.apiAccepts = func(token string) bool { return token == "access-from-refresh" }

	raw, err := GetDataWithError[json.RawMessage]("jobs", s.profile)
	if err != nil {
		t.Fatalf("GetDataWithError: %v", err)
	}
	if string(raw) != `{"ok":true}` {
		t.Errorf("body = %s", raw)
	}
	if s.passwordCalls != 1 || s.refreshCalls != 1 || s.apiCalls != 2 {
		t.Errorf("password=%d refresh=%d api=%d; want 1, 1, 2", s.passwordCalls, s.refreshCalls, s.apiCalls)
	}
}

// [P2] a 401 that survives the retry is returned as an error, never looped.
func TestDoAuthed_NeverLoops(t *testing.T) {
	s := newVBRStub(t)
	s.apiAccepts = func(string) bool { return false }

	_, err := GetDataWithError[json.RawMessage]("jobs", s.profile)
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("err = %v, want HTTP 401", err)
	}
	if s.apiCalls != 2 || s.refreshCalls != 1 {
		t.Errorf("api=%d refresh=%d; want exactly one retry after one refresh", s.apiCalls, s.refreshCalls)
	}
}

// The retry resends the request body.
func TestDoAuthed_RetryResendsBody(t *testing.T) {
	s := newVBRStub(t)
	s.apiAccepts = func(token string) bool { return token == "access-from-refresh" }

	if _, err := PostDataWithError("jobs", map[string]string{"name": "j1"}, s.profile); err != nil {
		t.Fatalf("PostDataWithError: %v", err)
	}
	if len(s.apiBodies) != 2 || s.apiBodies[0] != s.apiBodies[1] || !strings.Contains(s.apiBodies[1], `"j1"`) {
		t.Errorf("bodies = %q; the retry must resend the body", s.apiBodies)
	}
}

// An OWLCTL_TOKEN rejected with 401 is not renewed or retried.
func TestDoAuthed_EnvTokenNotRetried(t *testing.T) {
	s := newVBRStub(t)
	s.apiAccepts = func(string) bool { return false }
	t.Setenv("OWLCTL_TOKEN", "eyJ-fake-env-token-built-at-test-time")

	_, err := GetDataWithError[json.RawMessage]("jobs", s.profile)
	if err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("err = %v, want an authentication error", err)
	}
	if s.apiCalls != 1 || s.passwordCalls+s.refreshCalls != 0 {
		t.Errorf("api=%d password=%d refresh=%d; want one call and no renewal", s.apiCalls, s.passwordCalls, s.refreshCalls)
	}
}
