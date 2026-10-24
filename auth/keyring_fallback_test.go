package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func withKeyringEnv(t *testing.T, e keyringEnv) {
	t.Helper()
	orig := keyringEnvFunc
	keyringEnvFunc = func() keyringEnv { return e }
	t.Cleanup(func() { keyringEnvFunc = orig })
	t.Setenv("HOME", t.TempDir())
}

var errNoBus = errors.New("secret service unavailable: session bus not reachable")

func TestFileFallback(t *testing.T) {
	if _, ok := fileFallback(keyringEnv{hasFileKey: true}); !ok {
		t.Error("file key set: want file fallback")
	}
	if _, ok := fileFallback(keyringEnv{stdinTTY: true}); !ok {
		t.Error("TTY: want file fallback (passphrase prompt)")
	}
	if _, ok := fileFallback(keyringEnv{}); ok {
		t.Error("no key and no TTY: must not pick file")
	}
}

func TestNewTokenManager_LinuxAutoNoSecretServiceFallsBackToFile(t *testing.T) {
	t.Setenv(FileKeyEnvVar, "test-passphrase")
	withKeyringEnv(t, keyringEnv{goos: "linux", hasDBus: true, hasFileKey: true,
		secretServiceProbe: func() error { return errNoBus }})
	var tm *TokenManager
	var err error
	stderr := captureStderr(t, func() { tm, err = NewTokenManager(false) })
	if err != nil {
		t.Fatal(err)
	}
	if tm.Backend() != keyringFile || !tm.StorageEnabled() || tm.Degraded() != "" {
		t.Fatalf("backend=%q enabled=%v degraded=%q, want file storage", tm.Backend(), tm.StorageEnabled(), tm.Degraded())
	}
	if stderr != "" {
		t.Errorf("unexpected stderr: %q", stderr)
	}
	// Tokens stored now are reusable by a later process.
	if err := tm.StoreToken("vbr", "tok-1", 900); err != nil {
		t.Fatal(err)
	}
	tm2, err := NewTokenManager(false)
	if err != nil {
		t.Fatal(err)
	}
	if tm2.Backend() != keyringFile {
		t.Fatalf("second manager backend = %q", tm2.Backend())
	}
	if got, err := tm2.getTokenFromKeychain("vbr"); err != nil || got != "tok-1" {
		t.Errorf("reuse: token=%q err=%v", got, err)
	}
}

func TestNewTokenManager_LinuxAutoNoSecretServiceNoFileKeyDegrades(t *testing.T) {
	withKeyringEnv(t, keyringEnv{goos: "linux", hasDBus: true,
		secretServiceProbe: func() error { return errNoBus }})
	var tm *TokenManager
	var err error
	start := time.Now()
	stderr := captureStderr(t, func() { tm, err = NewTokenManager(false) })
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 2*time.Second {
		t.Error("decision should be immediate")
	}
	if tm.StorageEnabled() || tm.Backend() != keyringNone || tm.Degraded() == "" {
		t.Fatalf("want degraded none, got backend=%q enabled=%v degraded=%q", tm.Backend(), tm.StorageEnabled(), tm.Degraded())
	}
	if n := strings.Count(stderr, "OWLCTL_FILE_KEY"); n != 1 || strings.Count(stderr, "\n") != 1 {
		t.Errorf("want exactly one message naming OWLCTL_FILE_KEY, got %q", stderr)
	}
}

func TestNewTokenManager_ExplicitSecretServiceErrorsWhenUnavailable(t *testing.T) {
	withKeyringEnv(t, keyringEnv{setting: "secret-service", goos: "linux", hasFileKey: true,
		secretServiceProbe: func() error { return errNoBus }})
	if _, err := NewTokenManager(false); err == nil || !strings.Contains(err.Error(), "secret-service") {
		t.Fatalf("explicit secret-service must error, got %v", err)
	}
}

func TestNewTokenManager_ExplicitNoneNotDegraded(t *testing.T) {
	withKeyringEnv(t, keyringEnv{setting: "none", goos: "linux"})
	tm, err := NewTokenManager(false)
	if err != nil {
		t.Fatal(err)
	}
	if tm.Degraded() != "" {
		t.Errorf("explicit none must not be reported as degraded: %q", tm.Degraded())
	}
}

func TestNewTokenManager_ProbeNotUsedOffLinux(t *testing.T) {
	probed := false
	withKeyringEnv(t, keyringEnv{setting: "file", goos: "darwin", hasFileKey: true,
		secretServiceProbe: func() error { probed = true; return errNoBus }})
	t.Setenv(FileKeyEnvVar, "pw")
	if _, err := NewTokenManager(false); err != nil {
		t.Fatal(err)
	}
	if probed {
		t.Error("probe must only run for the Secret Service backend")
	}
}

func TestTimeoutHint_NamesBackendAndFixes(t *testing.T) {
	bk := &blockingKeyring{release: make(chan struct{})}
	t.Cleanup(func() { close(bk.release) })
	tm := &TokenManager{keyring: bk, backend: keyringSecretService, timeout: 50 * time.Millisecond}
	stderr := captureStderr(t, func() { _, _ = tm.keyringGet("k") })
	for _, want := range []string{"Secret Service (D-Bus) did not respond within", "OWLCTL_KEYRING=file", "OWLCTL_FILE_KEY", "OWLCTL_KEYRING=none", "OWLCTL_KEYRING_TIMEOUT"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("hint missing %q: %q", want, stderr)
		}
	}
}

func TestUnixSocketOf(t *testing.T) {
	tests := map[string]string{
		"unix:path=/run/user/1000/bus":       "/run/user/1000/bus",
		"unix:abstract=/tmp/dbus-x,guid=abc": "@/tmp/dbus-x",
		"tcp:host=localhost,port=1234":       "",
		"unix:path=/tmp/a;tcp:host=x":        "/tmp/a",
		"":                                   "",
	}
	for in, want := range tests {
		if got := unixSocketOf(in); got != want {
			t.Errorf("unixSocketOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDefaultProbe_StaleAddressFailsFast(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+t.TempDir()+"/nope")
	start := time.Now()
	if err := defaultProbeSecretService(); !errors.Is(err, errSecretServiceUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Error("stale address should fail fast")
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	if err := defaultProbeSecretService(); !errors.Is(err, errSecretServiceUnavailable) {
		t.Fatalf("unset address: err = %v", err)
	}
}
