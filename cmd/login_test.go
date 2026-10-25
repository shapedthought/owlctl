package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/shapedthought/owlctl/auth"
)

func TestLoginStatusLine(t *testing.T) {
	fileKeyErr := errors.New("OWLCTL_FILE_KEY environment variable required to use the file keyring")
	otherErr := errors.New("keyring: write failed")
	const fixFile = "OWLCTL_FILE_KEY not set; set it or use OWLCTL_KEYRING=none"
	tests := []struct {
		name     string
		instance string
		enabled  bool
		degraded string
		err      error
		want     string
	}{
		{"stored", "", true, "", nil, "Login OK"},
		{"stored instance", "p", true, "", nil, "Login OK (instance: p)"},
		{"explicit none", "", false, "", nil, "Login OK (token not stored: keyring disabled; set OWLCTL_KEYRING to store it)"},
		{"explicit none instance", "p", false, "", nil, "Login OK (instance: p; token not stored: keyring disabled; set OWLCTL_KEYRING to store it)"},
		{"degraded", "", false, "Secret Service unavailable", nil, "Login OK — token NOT stored (keyring unavailable: Secret Service unavailable)"},
		{"degraded instance", "p", false, "Secret Service did not respond", nil, "Login OK (instance: p) — token NOT stored (keyring unavailable: Secret Service did not respond)"},
		{"store fails file key", "", true, "", fileKeyErr, "Login OK — token NOT stored (" + fixFile + ")"},
		{"store fails instance", "p", true, "", fileKeyErr, "Login OK (instance: p) — token NOT stored (" + fixFile + ")"},
		{"store fails other", "", true, "", otherErr, "Login OK — token NOT stored (keyring write failed; set OWLCTL_KEYRING=none to skip storage)"},
		{"store times out", "", true, "", auth.ErrKeyringTimeout, "Login OK — token NOT stored (keyring timed out; set OWLCTL_KEYRING=none to skip storage)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loginStatusLine(tt.instance, tt.enabled, tt.degraded, tt.err)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if tt.err != nil && got == "Login OK" {
				t.Error("plain Login OK printed although the store failed")
			}
			if tt.err != nil && strings.Contains(got, tt.err.Error()) {
				t.Error("raw error text leaked into stdout")
			}
		})
	}
}

func TestStoreFailureHint(t *testing.T) {
	if h := storeFailureHint("file"); !strings.Contains(h, "OWLCTL_FILE_KEY") {
		t.Errorf("file hint should mention OWLCTL_FILE_KEY: %q", h)
	}
	kc := storeFailureHint("keychain")
	for _, want := range []string{"Always Allow", "security unlock-keychain", "OWLCTL_KEYRING=none"} {
		if !strings.Contains(kc, want) {
			t.Errorf("keychain hint missing %q: %q", want, kc)
		}
	}
	if strings.Contains(kc, "OWLCTL_FILE_KEY") {
		t.Errorf("keychain hint must not mention OWLCTL_FILE_KEY: %q", kc)
	}
	if h := storeFailureHint("secret-service"); !strings.Contains(h, "OWLCTL_KEYRING=file") {
		t.Errorf("secret-service hint should offer the file keyring: %q", h)
	}
	if storeFailureTarget("keychain") != "keychain" || storeFailureTarget("secret-service") != "keyring" || storeFailureTarget("file") != "keyring" {
		t.Error("stderr target should say keychain only for the Keychain backend")
	}
}
