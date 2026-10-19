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

	// defaultKeyringTimeout applies when nobody can respond to a prompt.
	defaultKeyringTimeout = 5 * time.Second
	// interactiveKeyringTimeout applies on an interactive terminal, leaving time
	// to approve a macOS Keychain dialog.
	interactiveKeyringTimeout = 60 * time.Second
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
	// auto is true when the backend was picked by "auto" rather than named
	// explicitly; a failure to open it degrades to "none" instead of erroring.
	auto bool
}

// keyringEnv carries the inputs to backend selection so it can be tested.
type keyringEnv struct {
	setting     string // raw OWLCTL_KEYRING value
	goos        string
	interactive bool // stdin is a TTY and not in CI
	stdinTTY    bool
	hasFileKey  bool
	hasDBus     bool
	// available lists the backends compiled into this build
	// (keyring.AvailableBackends()). nil means "all of them" (used by tests).
	available []keyring.BackendType
}

func (e keyringEnv) has(b keyring.BackendType) bool {
	if e.available == nil {
		return true
	}
	for _, a := range e.available {
		if a == b {
			return true
		}
	}
	return false
}

// keyringEnvFunc is swapped in tests.
var keyringEnvFunc = currentKeyringEnv

func currentKeyringEnv() keyringEnv {
	return keyringEnv{
		setting:     strings.ToLower(strings.TrimSpace(os.Getenv(KeyringEnvVar))),
		goos:        runtime.GOOS,
		interactive: isInteractiveSession(),
		stdinTTY:    term.IsTerminal(int(os.Stdin.Fd())),
		hasFileKey:  os.Getenv(FileKeyEnvVar) != "",
		hasDBus:     os.Getenv("DBUS_SESSION_BUS_ADDRESS") != "",
		available:   keyring.AvailableBackends(),
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
		// Only pick a backend compiled into this build (the cgo-free darwin
		// release binaries have no Keychain); otherwise use the file keyring.
		preferred := keyringChoice{keyring.FileBackend, keyringFile, true}
		switch e.goos {
		case "darwin":
			preferred = keyringChoice{keyring.KeychainBackend, keyringKeychain, true}
		case "windows":
			preferred = keyringChoice{keyring.WinCredBackend, keyringWinCred, true}
		case "linux":
			if e.hasDBus {
				preferred = keyringChoice{keyring.SecretServiceBackend, keyringSecretService, true}
			}
		}
		if !e.has(preferred.backend) {
			preferred = keyringChoice{keyring.FileBackend, keyringFile, true}
		}
		return preferred, nil
	case keyringKeychain:
		return explicitChoice(e, keyring.KeychainBackend, keyringKeychain)
	case keyringSecretService:
		return explicitChoice(e, keyring.SecretServiceBackend, keyringSecretService)
	case keyringWinCred:
		return explicitChoice(e, keyring.WinCredBackend, keyringWinCred)
	case keyringFile:
		if !e.hasFileKey && !e.stdinTTY {
			return none, fmt.Errorf("%s=file needs %s when stdin is not a terminal (it cannot prompt for a passphrase); set %s or use %s=none",
				KeyringEnvVar, FileKeyEnvVar, FileKeyEnvVar, KeyringEnvVar)
		}
		return keyringChoice{backend: keyring.FileBackend, name: keyringFile}, nil
	default:
		return none, fmt.Errorf("invalid %s value %q: must be one of auto, keychain, secret-service, wincred, file, none",
			KeyringEnvVar, os.Getenv(KeyringEnvVar))
	}
}

// explicitChoice validates a backend named by the user against this build.
func explicitChoice(e keyringEnv, b keyring.BackendType, name string) (keyringChoice, error) {
	if !e.has(b) {
		return keyringChoice{name: keyringNone}, fmt.Errorf("%s=%s: the %s backend is not available in this build of owlctl (release macOS binaries, for example, are built without Keychain); use %s=file or %s=none",
			KeyringEnvVar, name, name, KeyringEnvVar, KeyringEnvVar)
	}
	return keyringChoice{backend: b, name: name}, nil
}

// keyringTimeout returns the keyring operation timeout. The value is a Go
// duration ("10s") or a bare number of seconds. Invalid or non-positive values
// fall back to the default: 60s when a user can respond (interactive), else 5s.
func keyringTimeout(interactive bool) time.Duration {
	def := defaultKeyringTimeout
	if interactive {
		def = interactiveKeyringTimeout
	}
	v := strings.TrimSpace(os.Getenv(KeyringTimeoutEnvVar))
	if v == "" {
		return def
	}
	if d, err := time.ParseDuration(v); err == nil && d > 0 {
		return d
	}
	if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
		return time.Duration(n * float64(time.Second))
	}
	return def
}

var errKeyringDisabled = errors.New("token storage disabled (OWLCTL_KEYRING=none or non-interactive session)")
var errKeyringTimeout = errors.New("keyring operation timed out")
