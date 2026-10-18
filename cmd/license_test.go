package cmd

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const licenseFixture = "\xEF\xBB\xBF<?xml version=\"1.0\"?><License>SECRET-LICENSE-CONTENT</License>"

func TestBuildLicenseInstallBody_StripsBOM(t *testing.T) {
	body, hadBOM := buildLicenseInstallBody([]byte(licenseFixture))
	if !hadBOM {
		t.Error("expected BOM to be reported")
	}
	dec, err := base64.StdEncoding.DecodeString(body.License)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec) == 0 || dec[0] != '<' {
		t.Errorf("decoded licence should start with '<', got %q", dec[:1])
	}
}

func TestRunLicenseInstall_DryRunMakesNoHTTPCallAndHidesContent(t *testing.T) {
	var hits int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	t.Setenv("OWLCTL_URL", strings.TrimPrefix(srv.URL, "https://"))

	path := filepath.Join(t.TempDir(), "nfr.lic")
	if err := os.WriteFile(path, []byte(licenseFixture), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runLicenseInstall(&out, path, true); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Errorf("server saw %d requests, want 0", n)
	}
	s := out.String()
	if strings.Contains(s, "SECRET-LICENSE-CONTENT") || strings.Contains(s, "<License>") {
		t.Errorf("dry-run printed licence content:\n%s", s)
	}
	if !strings.Contains(s, "UTF-8 BOM:     yes") {
		t.Errorf("summary missing BOM line:\n%s", s)
	}
}

func TestRunLicenseInstall_DryRunRejectsNonXML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.lic")
	if err := os.WriteFile(path, []byte("not xml"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runLicenseInstall(&bytes.Buffer{}, path, true); err == nil {
		t.Error("expected error for non-XML file")
	}
}
