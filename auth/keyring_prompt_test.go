package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/99designs/keyring"
)

func TestWaitsOnPrompt(t *testing.T) {
	if !waitsOnPrompt(keyring.KeychainBackend) {
		t.Error("macOS Keychain can show a GUI prompt and must get the long timeout")
	}
	for _, b := range []keyring.BackendType{keyring.FileBackend, keyring.SecretServiceBackend, keyring.WinCredBackend} {
		if waitsOnPrompt(b) {
			t.Errorf("%s should not use the prompt timeout", b)
		}
	}
}

func TestTimeoutHint_KeychainMentionsPrompt(t *testing.T) {
	bk := &blockingKeyring{release: make(chan struct{})}
	t.Cleanup(func() { close(bk.release) })
	tm := &TokenManager{keyring: bk, backend: keyringKeychain, timeout: 50 * time.Millisecond}
	stderr := captureStderr(t, func() { _, _ = tm.keyringGet("k") })
	for _, want := range []string{"Keychain prompt may be waiting", "Always Allow", "security unlock-keychain", "OWLCTL_KEYRING_TIMEOUT"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("keychain hint missing %q: %q", want, stderr)
		}
	}
}
