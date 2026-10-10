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
