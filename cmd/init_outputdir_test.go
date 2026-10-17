package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func withOutputDir(t *testing.T, dir string) {
	t.Helper()
	old := outputDir
	outputDir = dir
	t.Cleanup(func() { outputDir = old })
}

func TestInitCreatesMissingOutputDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", ".owlctl")
	withOutputDir(t, dir)

	var stdout, stderr bytes.Buffer
	if err := runInitNonInteractive(&stdout, &stderr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range []string{"settings.json", "profiles.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("expected %s to exist: %v", f, err)
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0700 {
			t.Errorf("expected dir mode 0700, got %o", perm)
		}
	}
	if !strings.Contains(stderr.String(), "Initialized successfully") {
		t.Errorf("expected success message, got %q", stderr.String())
	}
}

func TestInitSubcommandsCreateMissingOutputDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "x", "y")
	withOutputDir(t, dir)

	var out, errOut bytes.Buffer
	if err := runInitSettings(&out, &errOut); err != nil {
		t.Fatalf("settings: %v", err)
	}
	if err := runInitProfiles(&out, &errOut); err != nil {
		t.Fatalf("profiles: %v", err)
	}
	for _, f := range []string{"settings.json", "profiles.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("expected %s to exist: %v", f, err)
		}
	}
}

func TestInitUnwritableOutputDirFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod permissions not enforced on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}

	parent := t.TempDir()
	if err := os.Chmod(parent, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0700) })
	dir := filepath.Join(parent, "sub")
	withOutputDir(t, dir)

	var stdout, stderr bytes.Buffer
	err := runInitNonInteractive(&stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for unwritable output dir")
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("error should name the path %q, got %v", dir, err)
	}
	if strings.Contains(stderr.String(), "Initialized successfully") || stdout.Len() != 0 {
		t.Errorf("no success output expected; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestInitUnwritableExistingDirFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod permissions not enforced on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}

	dir := t.TempDir()
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
	withOutputDir(t, dir)

	var stdout, stderr bytes.Buffer
	err := runInitNonInteractive(&stdout, &stderr)
	if err == nil {
		t.Fatal("expected write error")
	}
	if !strings.Contains(err.Error(), "profiles.json") {
		t.Errorf("error should name the file, got %v", err)
	}
	if strings.Contains(stderr.String(), "Initialized successfully") {
		t.Error("success message must not be printed on failure")
	}
}
