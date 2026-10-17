package vhttp

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/utils"
)

// setupGetDataTest points GetDataWithError at a TLS test server with a static token.
func setupGetDataTest(t *testing.T, handler http.HandlerFunc) models.Profile {
	t.Helper()

	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)

	host, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split host/port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	t.Setenv("OWLCTL_URL", host)
	t.Setenv("OWLCTL_TOKEN", "eyJ-test-token-for-get-data")
	utils.OverrideSettings(models.Settings{SelectedProfile: "vbr", ApiNotSecure: true})
	t.Cleanup(utils.ClearSettingsOverride)

	return models.Profile{Port: port}
}

func TestGetDataWithError_Success(t *testing.T) {
	profile := setupGetDataTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jobs/abc" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"abc","isDisabled":true}`))
	})

	raw, err := GetDataWithError[json.RawMessage]("jobs/abc", profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var job map[string]interface{}
	if err := json.Unmarshal(raw, &job); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if job["id"] != "abc" {
		t.Errorf("id = %v, want abc", job["id"])
	}
}

func TestGetDataWithError_HTTPErrorReturnsError(t *testing.T) {
	profile := setupGetDataTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"job not found"}`))
	})

	_, err := GetDataWithError[json.RawMessage]("jobs/missing", profile)
	if err == nil {
		t.Fatal("expected error for HTTP 404, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 404") || !strings.Contains(err.Error(), "job not found") {
		t.Errorf("error should carry status and body, got: %v", err)
	}
}

func TestGetDataWithError_InvalidJSONReturnsError(t *testing.T) {
	profile := setupGetDataTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	})

	if _, err := GetDataWithError[map[string]interface{}]("jobs/abc", profile); err == nil {
		t.Fatal("expected unmarshal error, got nil")
	}
}

func TestGetDataWithError_MissingURLReturnsError(t *testing.T) {
	utils.OverrideSettings(models.Settings{SelectedProfile: "vbr"})
	t.Cleanup(utils.ClearSettingsOverride)
	t.Setenv("OWLCTL_URL", "")

	if _, err := GetDataWithError[json.RawMessage]("jobs/abc", models.Profile{}); err == nil {
		t.Fatal("expected error when OWLCTL_URL is unset, got nil")
	}
}
