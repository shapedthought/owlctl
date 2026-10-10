package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLI_InitThenStatePath builds the binary and runs `init` then `state path`
// with OWLCTL_SETTINGS_PATH unset: state.json must resolve next to the
// settings.json that init wrote, as an absolute path.
func TestCLI_InitThenStatePath(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "owlctl")
	build := exec.Command("go", "build", "-o", bin, "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	work := t.TempDir()
	home := t.TempDir()
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "OWLCTL_") || strings.HasPrefix(e, "HOME=") || strings.HasPrefix(e, "USERPROFILE=") {
			continue
		}
		env = append(env, e)
	}
	env = append(env, "HOME="+home, "USERPROFILE="+home)

	run := func(args ...string) string {
		c := exec.Command(bin, args...)
		c.Dir = work
		c.Env = env
		out, err := c.Output()
		if err != nil {
			t.Fatalf("owlctl %v failed: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}

	run("init")
	if _, err := os.Stat(filepath.Join(work, "settings.json")); err != nil {
		t.Fatalf("init did not write settings.json in cwd: %v", err)
	}

	got := run("state", "path")
	wantDir, _ := filepath.EvalSymlinks(work)
	gotDir, _ := filepath.EvalSymlinks(filepath.Dir(got))
	if !filepath.IsAbs(got) || filepath.Base(got) != "state.json" || gotDir != wantDir {
		t.Errorf("state path = %q, want %q", got, filepath.Join(work, "state.json"))
	}
}

// TestCLI_StatePathLegacyNoteOnStderr: with only legacy state present, stdout
// must be just the absolute path (safe for $(owlctl state path)); the legacy
// note goes to stderr.
func TestCLI_StatePathLegacyNoteOnStderr(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "owlctl")
	if out, err := exec.Command("go", "build", "-o", bin, "..").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	work := t.TempDir()
	home := t.TempDir()
	legacyDir := filepath.Join(home, ".owlctl")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyJSON := "{\"version\":4,\"instances\":{}}"
	if err := os.WriteFile(filepath.Join(legacyDir, "state.json"), []byte(legacyJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "OWLCTL_") || strings.HasPrefix(e, "HOME=") || strings.HasPrefix(e, "USERPROFILE=") {
			continue
		}
		env = append(env, e)
	}
	env = append(env, "HOME="+home, "USERPROFILE="+home)

	c := exec.Command(bin, "state", "path")
	c.Dir = work
	c.Env = env
	var stdout, stderr strings.Builder
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		t.Fatalf("state path failed: %v\n%s", err, stderr.String())
	}

	got := strings.TrimSuffix(stdout.String(), "\n")
	if strings.ContainsAny(got, "\n ()") || !filepath.IsAbs(got) || filepath.Base(got) != "state.json" {
		t.Errorf("stdout = %q, want only an absolute state.json path", stdout.String())
	}
	if !strings.Contains(stderr.String(), "legacy") || !strings.Contains(stderr.String(), "state migrate") {
		t.Errorf("stderr = %q, want the legacy note", stderr.String())
	}
}
