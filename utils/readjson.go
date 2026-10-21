package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shapedthought/owlctl/internal/paths"
	"github.com/shapedthought/owlctl/models"
	"gopkg.in/yaml.v2"
)

func ReadCurrentProfile() {
	settings := ReadSettings()
	profile := GetProfile(settings.SelectedProfile)

	d, err := json.MarshalIndent(profile, "", "    ")
	IsErr(err)
	fmt.Println(string(d))
}

// GetProfile returns the profile with the given name
func GetProfile(profileName string) models.Profile {
	profilesFile := ReadProfilesFile()

	profile, exists := profilesFile.Profiles[profileName]
	if !exists {
		fmt.Fprintf(os.Stderr, "Profile '%s' not found in profiles.json\n", profileName)
		os.Exit(1)
	}

	// Apply instance port override if set
	if profilePortOverride != nil {
		profile.Port = *profilePortOverride
	}

	return profile
}

// GetCurrentProfile returns the currently selected profile from settings
func GetCurrentProfile() models.Profile {
	settings := ReadSettings()
	return GetProfile(settings.SelectedProfile)
}

// SettingPath returns the config directory with a trailing path separator, so
// callers can append a file name. With OWLCTL_SETTINGS_PATH unset it resolves
// to the current directory ("./").
func SettingPath() string {
	return paths.ConfigDir() + string(filepath.Separator)
}

// MissingConfigError is returned by readConfigFile when settings.json or
// profiles.json does not exist in the config directory.
type MissingConfigError struct {
	File string // file name, e.g. "settings.json"
	Path string // absolute path that was checked
	// EnvPath is the value of OWLCTL_SETTINGS_PATH ("" when unset).
	EnvPath string
}

func (e *MissingConfigError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "no owlctl %s found at %s\n", e.File, e.Path)
	if e.EnvPath != "" {
		fmt.Fprintf(&b, "OWLCTL_SETTINGS_PATH is set to %q, but %s is not there.\n", e.EnvPath, e.File)
		b.WriteString("Run 'owlctl init' (add --insecure for a self-signed VBR certificate) with OWLCTL_SETTINGS_PATH set to that directory,\n")
		b.WriteString("or point OWLCTL_SETTINGS_PATH at the directory where you ran 'owlctl init'.")
	} else {
		b.WriteString("OWLCTL_SETTINGS_PATH is not set, so the current directory is used.\n")
		b.WriteString("Run 'owlctl init' (add --insecure for a self-signed VBR certificate) in this directory,\n")
		b.WriteString("or set OWLCTL_SETTINGS_PATH to the directory where you ran it (e.g. export OWLCTL_SETTINGS_PATH=\"$HOME/.owlctl/\").")
	}
	return b.String()
}

// readConfigFile reads name from the config directory. A missing file is
// returned as *MissingConfigError; other errors are returned unchanged.
func readConfigFile(name string) ([]byte, error) {
	b, err := os.ReadFile(SettingPath() + name)
	if err == nil {
		return b, nil
	}
	if os.IsNotExist(err) {
		p := filepath.Join(paths.ConfigDir(), name)
		if abs, absErr := filepath.Abs(p); absErr == nil {
			p = abs
		}
		return nil, &MissingConfigError{File: name, Path: p, EnvPath: os.Getenv("OWLCTL_SETTINGS_PATH")}
	}
	return nil, err
}

// exitIfMissingConfig prints a missing-config error to stderr and exits 1.
// Other errors are left for the caller.
func exitIfMissingConfig(err error) {
	var me *MissingConfigError
	if errors.As(err, &me) {
		fmt.Fprintf(os.Stderr, "Error: %s\n", me.Error())
		os.Exit(1)
	}
}

// ReadProfilesFile reads the profiles.json file (new v1.0 format)
func ReadProfilesFile() models.ProfilesFile {
	var profilesFile models.ProfilesFile

	b, err := readConfigFile("profiles.json")
	exitIfMissingConfig(err)
	IsErr(err)

	err = json.Unmarshal(b, &profilesFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Invalid profiles.json format. Expected v1.0 format.\n")
		fmt.Fprintf(os.Stderr, "Please run 'owlctl init' to regenerate profiles.json\n")
		os.Exit(1)
	}

	// Validate version
	if profilesFile.Version != "1.0" {
		fmt.Fprintf(os.Stderr, "WARNING: Unsupported profiles.json version: %s (expected 1.0)\n", profilesFile.Version)
	}

	return profilesFile
}

// SaveProfilesFile writes the profiles file to disk
func SaveProfilesFile(profilesFile models.ProfilesFile) error {
	settingsPath := SettingPath()
	profileFile := settingsPath + "profiles.json"

	data, err := json.MarshalIndent(profilesFile, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal profiles: %w", err)
	}

	// Use restrictive permissions (0600) to protect infrastructure information
	if err := os.WriteFile(profileFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write profiles file: %w", err)
	}

	return nil
}

// settingsOverride allows ActivateInstance to change the effective profile/insecure
// without touching settings.json on disk.
var settingsOverride *models.Settings

// OverrideSettings sets a process-level settings override.
// Subsequent calls to ReadSettings() return this value instead of reading from disk.
func OverrideSettings(s models.Settings) { settingsOverride = &s }

// ClearSettingsOverride removes the settings override, reverting to disk reads.
func ClearSettingsOverride() { settingsOverride = nil }

// profilePortOverride allows ActivateInstance to override the product profile's default port.
var profilePortOverride *int

// OverrideProfilePort sets a process-level port override applied by GetProfile.
func OverrideProfilePort(port int) { profilePortOverride = &port }

// ClearProfilePortOverride removes the port override, reverting to the product default.
func ClearProfilePortOverride() { profilePortOverride = nil }

// TryReadSettings reads settings.json, returning a zero-value Settings if the
// file does not exist. Other errors are returned. Safe to call before init.
func TryReadSettings() (models.Settings, error) {
	if settingsOverride != nil {
		return *settingsOverride, nil
	}
	var settings models.Settings
	data, err := os.ReadFile(SettingPath() + "settings.json")
	if err != nil {
		if os.IsNotExist(err) {
			return settings, nil
		}
		return settings, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, err
	}
	return settings, nil
}

// WriteSettings persists s to settings.json at the configured path.
// Creates the settings directory if it does not already exist.
func WriteSettings(s models.Settings) error {
	data, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}
	settingsDir := SettingPath()
	settingsFile := settingsDir + "settings.json"
	if settingsDir != "" {
		dir := strings.TrimRight(settingsDir, "/\\")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create settings directory: %w", err)
		}
	}
	if err := os.WriteFile(settingsFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write settings.json: %w", err)
	}
	return nil
}

func ReadSettings() models.Settings {
	if settingsOverride != nil {
		return *settingsOverride
	}

	var settings models.Settings

	b, err := readConfigFile("settings.json")
	exitIfMissingConfig(err)
	IsErr(err)

	err = json.Unmarshal(b, &settings)
	IsErr(err)

	return settings
}

func ReadCreds() models.CredSpec {
	var creds models.CredSpec
	yml, err := os.Open("creds.yaml")
	IsErr(err)

	b, err := io.ReadAll(yml)
	IsErr(err)

	err = yaml.Unmarshal(b, &creds)
	IsErr(err)

	return creds
}
