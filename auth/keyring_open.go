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
// OWLCTL_KEYRING (see chooseKeyring). When storage is disabled (OWLCTL_KEYRING=none,
// or a non-interactive session with the default setting) no keyring is opened and
// the manager authenticates from the environment on every process.
func NewTokenManager(debug bool) (*TokenManager, error) {
	choice, err := chooseKeyring(currentKeyringEnv())
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
	// there is no separate auto-locking keychain, and KeychainTrustApplication
	// avoids a confirmation prompt on every read.
	kr, err := keyring.Open(keyring.Config{
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
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open %s keyring: %w", choice.name, err)
	}

	tm := &TokenManager{keyring: kr, debug: debug, backend: choice.name, timeout: keyringTimeout()}
	if choice.backend == keyring.FileBackend {
		// The file backend may legitimately wait on the user typing a passphrase.
		tm.timeout = 0
	}
	if debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Token storage backend: %s\n", choice.name)
	}
	return tm, nil
}

// keyringCall runs fn with the configured timeout. A blocked backend (for
// example a locked keychain with no GUI to show the unlock dialog) returns
// errKeyringTimeout after printing one hint; later calls then skip the keyring.
func (tm *TokenManager) keyringCall(fn func(keyring.Keyring) error) error {
	if tm.keyring == nil || tm.timedOut {
		return errKeyringDisabled
	}
	if tm.timeout <= 0 {
		return fn(tm.keyring)
	}
	done := make(chan error, 1)
	go func() { done <- fn(tm.keyring) }()
	select {
	case err := <-done:
		return err
	case <-time.After(tm.timeout):
		tm.timedOut = true
		tm.hintOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "owlctl: keyring did not respond within %s; continuing without stored tokens. Set OWLCTL_TOKEN or OWLCTL_KEYRING=none to skip the keyring (OWLCTL_KEYRING_TIMEOUT adjusts the wait).\n", tm.timeout)
		})
		return errKeyringTimeout
	}
}

func (tm *TokenManager) keyringGet(key string) (keyring.Item, error) {
	var item keyring.Item
	err := tm.keyringCall(func(kr keyring.Keyring) error {
		var e error
		item, e = kr.Get(key)
		return e
	})
	return item, err
}
