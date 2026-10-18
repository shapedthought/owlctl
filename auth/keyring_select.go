package auth

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/99designs/keyring"
	"golang.org/x/term"
)

const (
	// KeyringEnvVar selects the token storage backend.
	KeyringEnvVar = "OWLCTL_KEYRING"
	// KeyringTimeoutEnvVar overrides the keyring operation timeout.
	KeyringTimeoutEnvVar = "OWLCTL_KEYRING_TIMEOUT"
	// FileKeyEnvVar holds the file keyring passphrase.
	FileKeyEnvVar = "OWLCTL_FILE_KEY"

	defaultKeyringTimeout = 5 * time.Second
)

// Valid OWLCTL_KEYRING values.
const (
	keyringAuto          = "auto"
	keyringKeychain      = "keychain"
	keyringSecretService = "secret-service"
	keyringWinCred       = "wincred"
	keyringFile          = "file"
	keyringNone          = "none"
)

// keyringChoice is the resolved storage decision.
type keyringChoice struct {
	// backend is the keyring backend to open; empty means storage is disabled.
	backend keyring.BackendType
	// name is the OWLCTL_KEYRING-style name of the backend ("none" when disabled).
	name string
}

// keyringEnv carries the inputs to backend selection so it can be tested.
type keyringEnv struct {
	setting     string // raw OWLCTL_KEYRING value
	goos        string
	interactive bool // stdin is a TTY and not in CI
	stdinTTY    bool
	hasFileKey  bool
	hasDBus     bool
}

func currentKeyringEnv() keyringEnv {
	return keyringEnv{
		setting:     strings.ToLower(strings.TrimSpace(os.Getenv(KeyringEnvVar))),
		goos:        runtime.GOOS,
		interactive: isInteractiveSession(),
		stdinTTY:    term.IsTerminal(int(os.Stdin.Fd())),
		hasFileKey:  os.Getenv(FileKeyEnvVar) != "",
		hasDBus:     os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "",
	}
}

// chooseKeyring decides which backend (if any) to open. It never opens anything
// that could prompt in a non-interactive session unless the user asked for it.
func chooseKeyring(e keyringEnv) (keyringChoice, error) {
	none := keyringChoice{name: keyringNone}

	switch e.setting {
	case keyringNone:
		return none, nil
	case "", keyringAuto:
		if !e.interactive {
			// stdin not a TTY or CI: every backend might prompt, so act as "none".
			return none, nil
		}
		switch e.goos {
		case "darwin":
			return keyringChoice{keyring.KeychainBackend, keyringKeychain}, nil
		case "windows":
			return keyringChoice{keyring.WinCredBackend, keyringWinCred}, nil
		case "linux":
			if e.hasDBus {
				return keyringChoice{keyring.SecretServiceBackend, keyringSecretService}, nil
			}
		}
		return keyringChoice{keyring.FileBackend, keyringFile}, nil
	case keyringKeychain:
		return keyringChoice{keyring.KeychainBackend, keyringKeychain}, nil
	case keyringSecretService:
		return keyringChoice{keyring.SecretServiceBackend, keyringSecretService}, nil
	case keyringWinCred:
		return keyringChoice{keyring.WinCredBackend, keyringWinCred}, nil
	case keyringFile:
		if !e.hasFileKey && !e.stdinTTY {
			return none, fmt.Errorf("%s=file needs %s when stdin is not a terminal (it cannot prompt for a passphrase); set %s or use %s=none",
				KeyringEnvVar, FileKeyEnvVar, FileKeyEnvVar, KeyringEnvVar)
		}
		return keyringChoice{keyring.FileBackend, keyringFile}, nil
	default:
		return none, fmt.Errorf("invalid %s value %q: must be one of auto, keychain, secret-service, wincred, file, none",
			KeyringEnvVar, os.Getenv(KeyringEnvVar))
	}
}

// keyringTimeout returns the keyring operation timeout. The value is a Go
// duration ("10s") or a bare number of seconds. Invalid or non-positive values
// fall back to the default.
func keyringTimeout() time.Duration {
	v := strings.TrimSpace(os.Getenv(KeyringTimeoutEnvVar))
	if v == "" {
		return defaultKeyringTimeout
	}
	if d, err := time.ParseDuration(v); err == nil && d > 0 {
		return d
	}
	if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
		return time.Duration(n * float64(time.Second))
	}
	return defaultKeyringTimeout
}

var errKeyringDisabled = errors.New("token storage disabled (OWLCTL_KEYRING=none or non-interactive session)")
var errKeyringTimeout = errors.New("keyring operation timed out")
