package auth

import (
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const (
	defaultLockWait = 10 * time.Second
	lockPoll        = 50 * time.Millisecond
)

var unsafeLockChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// lockPath is the advisory lock file for a keyring item, next to the file
// keyring's directory (~/.owlctl/<key>.lock).
func lockPath(key string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".owlctl", unsafeLockChars.ReplaceAllString(key, "_")+".lock")
}

// lockRefresh serialises token refresh across owlctl processes sharing the same
// stored session. The wait is bounded (the keyring timeout when set, else 10s);
// if the lock cannot be taken it returns a no-op release and the caller carries
// on unserialised. It does nothing when token storage is disabled.
func (tm *TokenManager) lockRefresh(key string) (release func()) {
	noop := func() {}
	if tm.keyring == nil {
		return noop
	}
	path := lockPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return noop
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return noop
	}
	wait := defaultLockWait
	if tm.timeout > 0 {
		wait = tm.timeout
	}
	deadline := time.Now().Add(wait)
	for !tryLockFile(f) {
		if time.Now().After(deadline) {
			f.Close()
			return noop
		}
		time.Sleep(lockPoll)
	}
	return func() {
		unlockFile(f)
		f.Close()
	}
}
