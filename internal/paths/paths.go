// Package paths resolves owlctl's config directory. It has no dependencies on
// other owlctl packages so that both utils and state can share one resolver.
package paths

import (
	"os"
	"path/filepath"
)

// ConfigDir returns the directory holding settings.json, profiles.json,
// headers.json and state.json. It is OWLCTL_SETTINGS_PATH (cleaned) when set,
// otherwise the current directory (".").
func ConfigDir() string {
	if p := os.Getenv("OWLCTL_SETTINGS_PATH"); p != "" {
		return filepath.Clean(p)
	}
	return "."
}

// File returns the path of the named file inside ConfigDir.
func File(name string) string {
	return filepath.Join(ConfigDir(), name)
}

// LegacyStatePath returns the pre-#156 default state location,
// <home>/.owlctl/state.json. It returns "" if the home directory is unknown.
func LegacyStatePath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".owlctl", "state.json")
}
