package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUsesOverlayShorthand(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"job", "apply", "x.yaml", "-o", "o.yaml"}, true},
		{[]string{"job", "apply", "x.yaml", "-o=o.yaml"}, true},
		{[]string{"job", "apply", "x.yaml", "-oo.yaml"}, true},
		{[]string{"job", "apply", "x.yaml", "--overlay", "o.yaml"}, false},
		{[]string{"job", "apply", "x.yaml", "--overlay=o.yaml"}, false},
		{[]string{"job", "apply", "x.yaml"}, false},
		{[]string{"job", "apply", "--", "-o"}, false},
	}
	for _, c := range cases {
		if got := usesOverlayShorthand(c.args); got != c.want {
			t.Errorf("usesOverlayShorthand(%v) = %v, want %v", c.args, got, c.want)
		}
	}
}

func TestWriteOverlayShorthandWarning(t *testing.T) {
	var buf bytes.Buffer
	writeOverlayShorthandWarning(&buf, []string{"-o", "o.yaml"})
	if got, want := buf.String(), "Warning: -o for --overlay is deprecated; use --overlay.\n"; got != want {
		t.Errorf("warning = %q, want %q", got, want)
	}
	buf.Reset()
	writeOverlayShorthandWarning(&buf, []string{"--overlay", "o.yaml"})
	if buf.Len() != 0 {
		t.Errorf("unexpected warning for --overlay: %q", buf.String())
	}
}

func TestDeprecatedExportWarningText(t *testing.T) {
	want := "Warning: 'owlctl export <id>' is deprecated and will be removed in a future release; use 'owlctl job export <id>'."
	if deprecatedExportWarning != want {
		t.Errorf("deprecatedExportWarning = %q, want %q", deprecatedExportWarning, want)
	}
}

// aliasCLI builds the binary, runs `init` in a scratch dir, and returns a runner
// that executes owlctl there with no OWLCTL_* env (so no VBR is ever contacted).
func aliasCLI(t *testing.T) (work string, run func(args ...string) (stdout, stderr string, code int)) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "owlctl")
	if out, err := exec.Command("go", "build", "-o", bin, "..").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	work = t.TempDir()
	home := t.TempDir()
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "OWLCTL_") || strings.HasPrefix(e, "HOME=") || strings.HasPrefix(e, "USERPROFILE=") {
			continue
		}
		env = append(env, e)
	}
	env = append(env, "HOME="+home, "USERPROFILE="+home)

	run = func(args ...string) (string, string, int) {
		c := exec.Command(bin, args...)
		c.Dir = work
		c.Env = env
		var o, e bytes.Buffer
		c.Stdout, c.Stderr = &o, &e
		err := c.Run()
		code := 0
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("owlctl %v: %v", args, err)
		}
		return o.String(), e.String(), code
	}
	if _, _, code := run("init"); code != 0 {
		t.Fatalf("init failed with exit code %d", code)
	}
	return work, run
}

// The alias and 'job export' share a Run function, so with no VBR reachable
// both fail at the same point: same stdout and exit code, and the only
// difference is the one-line warning on the alias's stderr.
func TestCLI_ExportAliasMatchesJobExport(t *testing.T) {
	_, run := aliasCLI(t)

	newOut, newErr, newCode := run("job", "export", "abc", "-o", "out.yaml")
	oldOut, oldErr, oldCode := run("export", "abc", "-o", "out.yaml")

	if oldOut != newOut {
		t.Errorf("stdout differs:\nalias: %q\njob export: %q", oldOut, newOut)
	}
	if oldCode != newCode {
		t.Errorf("exit code differs: alias %d, job export %d", oldCode, newCode)
	}
	if strings.Contains(newOut, "deprecated") || strings.Contains(newErr, "deprecated") {
		t.Errorf("job export must not warn; stdout=%q stderr=%q", newOut, newErr)
	}
	if strings.Contains(oldOut, "deprecated") {
		t.Errorf("warning leaked to stdout: %q", oldOut)
	}
	if !strings.HasPrefix(oldErr, deprecatedExportWarning+"\n") {
		t.Errorf("stderr should start with the warning, got %q", oldErr)
	}
	if got := strings.Count(oldErr, "deprecated"); got != 1 {
		t.Errorf("expected exactly one deprecation line, got %d: %q", got, oldErr)
	}
	if strings.TrimPrefix(oldErr, deprecatedExportWarning+"\n") != newErr {
		t.Errorf("rest of stderr differs:\nalias: %q\njob export: %q", oldErr, newErr)
	}
}

// 'export --all' keeps its new all-resource-types meaning and does not warn.
func TestCLI_ExportAllDoesNotWarn(t *testing.T) {
	_, run := aliasCLI(t)
	_, stderr, _ := run("export", "--all", "--from-state", "-d", "exports")
	if strings.Contains(stderr, "deprecated") {
		t.Errorf("export --all must not warn: %q", stderr)
	}
}

// 'job apply x.yaml -o o.yaml' merges the overlay and warns on stderr only.
func TestCLI_ApplyOverlayShorthand(t *testing.T) {
	work, run := aliasCLI(t)
	write := func(name, desc string) {
		spec := "apiVersion: owlctl.io/v1\nkind: VBRJob\nmetadata:\n  name: j1\nspec:\n  description: " + desc + "\n"
		if err := os.WriteFile(filepath.Join(work, name), []byte(spec), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("x.yaml", "base")
	write("o.yaml", "overlay")

	// Dry run still needs VBR to fetch the current job, so it exits non-zero
	// after the overlay has been merged; the overlay line proves the merge ran.
	longOut, longErr, _ := run("job", "apply", "x.yaml", "--overlay", "o.yaml", "--dry-run")
	out, stderr, _ := run("job", "apply", "x.yaml", "-o", "o.yaml", "--dry-run")

	if !strings.Contains(out, "Applying overlay: o.yaml") {
		t.Fatalf("overlay was not applied via -o; stdout=%q stderr=%q", out, stderr)
	}
	if strings.Contains(out, "deprecated") {
		t.Errorf("warning leaked to stdout: %q", out)
	}
	if !strings.Contains(stderr, deprecatedOverlayShorthandWarning+"\n") {
		t.Errorf("stderr missing shorthand warning: %q", stderr)
	}
	if out != longOut {
		t.Errorf("stdout differs between -o and --overlay:\n-o: %q\n--overlay: %q", out, longOut)
	}
	if strings.Contains(longErr, "deprecated") {
		t.Errorf("--overlay must not warn: %q", longErr)
	}
}
