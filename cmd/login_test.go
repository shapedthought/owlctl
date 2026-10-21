package cmd

import (
	"errors"
	"strings"
	"testing"
)

func TestLoginStatusLine(t *testing.T) {
	fileKeyErr := errors.New("OWLCTL_FILE_KEY environment variable required to use the file keyring")
	otherErr := errors.New("keyring: write failed")
	const fixFile = "token not stored: OWLCTL_FILE_KEY not set; set it or use OWLCTL_KEYRING=none"
	tests := []struct {
		name     string
		instance string
		enabled  bool
		err      error
		want     string
	}{
		{"stored", "", true, nil, "Login OK"},
		{"stored instance", "p", true, nil, "Login OK (instance: p)"},
		{"disabled", "", false, nil, "Login OK (token not stored: keyring disabled; set OWLCTL_KEYRING to store it)"},
		{"store fails file key", "", true, fileKeyErr, "Login OK (" + fixFile + ")"},
		{"store fails instance", "p", true, fileKeyErr, "Login OK (instance: p; " + fixFile + ")"},
		{"store fails other", "", true, otherErr, "Login OK (token not stored: keyring write failed; set OWLCTL_KEYRING=none to skip storage)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loginStatusLine(tt.instance, tt.enabled, tt.err)
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
