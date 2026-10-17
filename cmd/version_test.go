package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func runRoot(t *testing.T, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
	return buf.String()
}

func setBuildInfo(t *testing.T, v, c, d string) {
	t.Helper()
	ov, oc, od, orv := version, commit, date, rootCmd.Version
	version, commit, date = v, c, d
	rootCmd.Version = v
	t.Cleanup(func() {
		version, commit, date = ov, oc, od
		rootCmd.Version = orv
	})
}

func TestVersionFlagDefault(t *testing.T) {
	setBuildInfo(t, "dev", "none", "unknown")
	if out := runRoot(t, "--version"); !strings.Contains(out, "dev") {
		t.Errorf("expected dev in %q", out)
	}
}

func TestVersionFlagInjected(t *testing.T) {
	setBuildInfo(t, "v9.9.9", "abc123", "2026-01-02T03:04:05Z")
	if out := runRoot(t, "--version"); !strings.Contains(out, "v9.9.9") {
		t.Errorf("expected v9.9.9 in %q", out)
	}
}

func TestVersionSubcommand(t *testing.T) {
	setBuildInfo(t, "v9.9.9", "abc123", "2026-01-02T03:04:05Z")
	out := runRoot(t, "version")
	for _, want := range []string{"v9.9.9", "abc123", "2026-01-02T03:04:05Z", "go"} {
		if !strings.Contains(out, want) {
			t.Errorf("version output %q missing %q", out, want)
		}
	}
}

func TestToggleFlagRemoved(t *testing.T) {
	if rootCmd.Flags().Lookup("toggle") != nil {
		t.Error("--toggle should not exist on the root command")
	}
	if rootCmd.Flags().ShorthandLookup("t") != nil {
		t.Error("-t should not exist on the root command")
	}
}

func TestCompareVersion(t *testing.T) {
	tests := []struct {
		current, latest, want string
	}{
		{"dev", "v1.3.0", "development build"},
		{"v1.3.0", "v1.3.0", "latest version."},
		{"v1.2.0", "v1.3.0", "You are running v1.2.0, the latest version is v1.3.0"},
	}
	for _, tt := range tests {
		if got := compareVersion(tt.current, tt.latest); !strings.Contains(got, tt.want) {
			t.Errorf("compareVersion(%q,%q)=%q, want it to contain %q", tt.current, tt.latest, got, tt.want)
		}
	}
	if got := compareVersion("dev", "v1.3.0"); !strings.Contains(got, "v1.3.0") {
		t.Errorf("dev case should show latest release, got %q", got)
	}
}

func TestLatestTag(t *testing.T) {
	if tag, err := latestTag(map[string]interface{}{"tag_name": "v1.3.0"}); err != nil || tag != "v1.3.0" {
		t.Errorf("got %q, %v", tag, err)
	}
	_, err := latestTag(map[string]interface{}{"message": "API rate limit exceeded"})
	if err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected rate limit error, got %v", err)
	}
	if _, err := latestTag(map[string]interface{}{"tag_name": 5}); err == nil {
		t.Error("expected error for non-string tag_name")
	}
	if _, err := latestTag(nil); err == nil {
		t.Error("expected error for nil result")
	}
}
