package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/99designs/keyring"
	"golang.org/x/term"
)

// NewTokenManager creates a new token manager. The storage backend is chosen by
// OWLCTL_KEYRING (see chooseKeyring). When storage is disabled (OWLCTL_KEYRING=none)
// no keyring is opened and
// the manager authenticates from the environment on every process.
//
// If the backend picked by "auto" cannot be opened (not compiled into this build,
// or hung past the timeout) storage is disabled instead of failing; a backend
// named explicitly fails with an error.
func NewTokenManager(debug bool) (*TokenManager, error) {
	kenv := keyringEnvFunc()
	choice, err := chooseKeyring(kenv)
	if err != nil {
		return nil, err
	}
	if choice.backend == "" {
		if debug {
			fmt.Fprintln(os.Stderr, "DEBUG: Token storage disabled (keyring backend: none)")
		}
		return &TokenManager{debug: debug, backend: keyringNone}, nil
	}

	// Get home directory for file backend
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "." // Fallback to current directory if home not available
	}

	// KeychainName is deliberately unset: the user's login keychain is used so
	// there is no separate auto-locking keychain. KeychainTrustApplication puts
	// the creating binary on the item's access list; the backend's Set updates an
	// existing item in place (no delete and re-create, access list untouched so
	// an update does not prompt again). macOS identifies the binary by its code
	// signature, so an ad-hoc rebuild is a new, untrusted binary for items an
	// earlier build created.
	cfg := keyring.Config{
		ServiceName:              KeyringService,
		KeychainTrustApplication: true,
		FileDir:                  homeDir + "/.owlctl",
		FilePasswordFunc: func(prompt string) (string, error) {
			// Check environment variable first (allows non-interactive configuration)
			if pw := os.Getenv(FileKeyEnvVar); pw != "" {
				return pw, nil
			}

			// Prompt for password in interactive mode
			if term.IsTerminal(int(os.Stdin.Fd())) {
				if prompt == "" {
					prompt = "Enter password for owlctl file keyring: "
				}
				fmt.Fprint(os.Stderr, prompt)
				pwBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(os.Stderr)
				if err != nil {
					return "", fmt.Errorf("failed to read password: %w", err)
				}
				if len(pwBytes) == 0 {
					return "", errors.New("empty password not allowed")
				}
				return string(pwBytes), nil
			}

			return "", errors.New("OWLCTL_FILE_KEY environment variable required for file keyring in non-interactive mode")
		},
		AllowedBackends: []keyring.BackendType{choice.backend},
	}

	tm := &TokenManager{debug: debug, backend: choice.name, timeout: keyringTimeout(kenv.interactive || waitsOnPrompt(choice.backend))}
	if choice.backend == keyring.FileBackend {
		// The file backend may legitimately wait on the user typing a passphrase.
		tm.timeout = 0
	}

	kr, err := tm.openKeyring(cfg)
	if err != nil {
		if choice.auto {
			if debug {
				fmt.Fprintf(os.Stderr, "DEBUG: %s keyring unavailable (%v); token storage disabled\n", choice.name, err)
			}
			return &TokenManager{debug: debug, backend: keyringNone}, nil
		}
		return nil, fmt.Errorf("failed to open %s keyring: %w", choice.name, err)
	}
	tm.keyring = kr
	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Token storage backend: %s\n", choice.name)
	}
	return tm, nil
}

// openKeyring opens the backend, bounded by the keyring timeout so a hung
// D-Bus session or Keychain cannot block forever.
func (tm *TokenManager) openKeyring(cfg keyring.Config) (keyring.Keyring, error) {
	if tm.timeout <= 0 {
		return keyring.Open(cfg)
	}
	type result struct {
		kr  keyring.Keyring
		err error
	}
	done := make(chan result, 1)
	go func() {
		kr, err := keyring.Open(cfg)
		done <- result{kr, err}
	}()
	select {
	case r := <-done:
		return r.kr, r.err
	case <-time.After(tm.timeout):
		tm.printTimeoutHint()
		return nil, errKeyringTimeout
	}
}

// waitsOnPrompt reports whether the backend can show a GUI prompt that a person
// must answer regardless of stdin (the macOS Keychain access and unlock
// dialogs). Redirecting stdin does not suppress it, so the short non-interactive
// timeout would give up before the user can click.
func waitsOnPrompt(b keyring.BackendType) bool {
	return b == keyring.KeychainBackend
}

func (tm *TokenManager) printTimeoutHint() {
	tm.hintOnce.Do(func() {
		if tm.backend == keyringKeychain {
			fmt.Fprintf(os.Stderr, "owlctl: keyring did not respond within %s; a macOS Keychain prompt may be waiting for your answer (look for a dialog, possibly behind other windows). Continuing without stored tokens. Choose \"Always Allow\" in the prompt, run 'security unlock-keychain' if the login keychain is locked, raise OWLCTL_KEYRING_TIMEOUT, or set OWLCTL_TOKEN or OWLCTL_KEYRING=none to skip the keyring.\n", tm.timeout)
			return
		}
		fmt.Fprintf(os.Stderr, "owlctl: keyring did not respond within %s; continuing without stored tokens. Set OWLCTL_TOKEN or OWLCTL_KEYRING=none to skip the keyring (OWLCTL_KEYRING_TIMEOUT adjusts the wait).\n", tm.timeout)
	})
}

// keyringDo runs fn with the configured timeout. A blocked backend (for
// example a locked keychain with no GUI to show the unlock dialog) returns
// errKeyringTimeout after printing one hint; later calls then skip the keyring.
// The result travels through the channel so nothing is shared after a timeout.
func keyringDo[T any](tm *TokenManager, fn func(keyring.Keyring) (T, error)) (T, error) {
	var zero T
	if tm.keyring == nil || tm.timedOut {
		return zero, errKeyringDisabled
	}
	if tm.timeout <= 0 {
		return fn(tm.keyring)
	}
	type result struct {
		v   T
		err error
	}
	done := make(chan result, 1)
	kr := tm.keyring
	go func() {
		v, err := fn(kr)
		done <- result{v, err}
	}()
	select {
	case r := <-done:
		return r.v, r.err
	case <-time.After(tm.timeout):
		tm.timedOut = true
		tm.printTimeoutHint()
		return zero, errKeyringTimeout
	}
}

func (tm *TokenManager) keyringCall(fn func(keyring.Keyring) error) error {
	_, err := keyringDo(tm, func(kr keyring.Keyring) (struct{}, error) { return struct{}{}, fn(kr) })
	return err
}

func (tm *TokenManager) keyringGet(key string) (keyring.Item, error) {
	return keyringDo(tm, func(kr keyring.Keyring) (keyring.Item, error) { return kr.Get(key) })
}
