package utils

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingConfigEnvPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OWLCTL_SETTINGS_PATH", dir)
	for _, name := range []string{"settings.json", "profiles.json"} {
		_, err := readConfigFile(name)
		var me *MissingConfigError
		if !errors.As(err, &me) {
			t.Fatalf("%s: expected MissingConfigError, got %v", name, err)
		}
		want := filepath.Join(dir, name)
		if me.Path != want {
			t.Errorf("path = %q, want %q", me.Path, want)
		}
		msg := me.Error()
		for _, s := range []string{want, "OWLCTL_SETTINGS_PATH is set to", "owlctl init", "--insecure"} {
			if !strings.Contains(msg, s) {
				t.Errorf("%s: message missing %q:\n%s", name, s, msg)
			}
		}
	}
}

func TestMissingConfigCwdDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OWLCTL_SETTINGS_PATH", "")
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	realDir, _ := filepath.EvalSymlinks(dir)
	for _, name := range []string{"settings.json", "profiles.json"} {
		_, err := readConfigFile(name)
		var me *MissingConfigError
		if !errors.As(err, &me) {
			t.Fatalf("%s: expected MissingConfigError, got %v", name, err)
		}
		if !filepath.IsAbs(me.Path) {
			t.Errorf("path %q is not absolute", me.Path)
		}
		gotDir, _ := filepath.EvalSymlinks(filepath.Dir(me.Path))
		if gotDir != realDir || filepath.Base(me.Path) != name {
			t.Errorf("path = %q, want %s in %q", me.Path, name, realDir)
		}
		msg := me.Error()
		for _, s := range []string{"current directory", "owlctl init", "--insecure", "OWLCTL_SETTINGS_PATH"} {
			if !strings.Contains(msg, s) {
				t.Errorf("%s: message missing %q:\n%s", name, s, msg)
			}
		}
	}
}

func TestOtherReadErrorsNotMasked(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OWLCTL_SETTINGS_PATH", dir)
	// A directory named settings.json gives a read error that is not "not exist".
	if err := os.Mkdir(filepath.Join(dir, "settings.json"), 0755); err != nil {
		t.Fatal(err)
	}
	_, err := readConfigFile("settings.json")
	var me *MissingConfigError
	if err == nil || errors.As(err, &me) {
		t.Fatalf("expected a non-missing error, got %v", err)
	}
}
