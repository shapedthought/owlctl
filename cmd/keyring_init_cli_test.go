//go:build linux

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCLI_NoDBusInitDoesNotAutolaunchOrHang builds the binary and runs a command
// that never touches the keyring. Upstream keyring connected to the session bus
// from init(), which autolaunched a dbus-daemon when DBUS_SESSION_BUS_ADDRESS was
// unset and hung on an unreachable address. A fake dbus-launch on PATH records
// any autolaunch attempt.
func TestCLI_NoDBusInitDoesNotAutolaunchOrHang(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "owlctl")
	if out, err := exec.Command("go", "build", "-o", bin, "..").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	fakeBin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "autolaunched")
	script := "#!/bin/sh\necho launched > " + marker + "\nexit 1\n"
	if err := os.WriteFile(filepath.Join(fakeBin, "dbus-launch"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"unset address":         "",
		"unreachable tcp":       "tcp:host=10.255.255.1,port=9",
		"stale unix socket":     "unix:path=/nonexistent/owlctl-test-bus",
		"unreachable tcp (set)": "tcp:host=192.0.2.1,port=9",
	}
	for name, addr := range cases {
		t.Run(name, func(t *testing.T) {
			var env []string
			for _, e := range os.Environ() {
				if strings.HasPrefix(e, "OWLCTL_") || strings.HasPrefix(e, "DBUS_") || strings.HasPrefix(e, "PATH=") || strings.HasPrefix(e, "HOME=") {
					continue
				}
				env = append(env, e)
			}
			env = append(env, "PATH="+fakeBin+":"+os.Getenv("PATH"), "HOME="+t.TempDir())
			if addr != "" {
				env = append(env, "DBUS_SESSION_BUS_ADDRESS="+addr)
			}
			c := exec.Command(bin, "--help")
			c.Dir = t.TempDir()
			c.Env = env
			start := time.Now()
			done := make(chan error, 1)
			go func() { _, err := c.CombinedOutput(); done <- err }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("owlctl --help failed: %v", err)
				}
			case <-time.After(10 * time.Second):
				_ = c.Process.Kill()
				t.Fatal("owlctl --help hung (init-time D-Bus connection?)")
			}
			if d := time.Since(start); d > 2*time.Second {
				t.Errorf("took %s, want <= 2s", d)
			}
			if _, err := os.Stat(marker); err == nil {
				t.Error("dbus-launch was executed: a dbus-daemon would have leaked")
			}
		})
	}
}
