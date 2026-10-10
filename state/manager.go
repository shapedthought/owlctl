package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/shapedthought/owlctl/internal/paths"
)

// Manager handles state file operations
type Manager struct {
	statePath string
}

var legacyWarnOnce sync.Once

// NewManager creates a new state manager.
// The state file lives next to settings.json: OWLCTL_SETTINGS_PATH if set,
// otherwise the current directory. It creates no directories (Save does).
// If only a legacy ~/.owlctl/state.json exists, a warning is printed to stderr.
func NewManager() *Manager {
	m := NewManagerQuiet()
	if legacy := m.LegacyStatePath(); legacy != "" {
		legacyWarnOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "Warning: found legacy state at %s, but owlctl now uses %s. State is NOT moved automatically. Run 'owlctl state migrate' to move it, or set OWLCTL_SETTINGS_PATH=%s to keep using it.\n",
				legacy, absPath(m.statePath), filepath.Dir(legacy))
		})
	}
	return m
}

// NewManagerQuiet is NewManager without the legacy-state warning.
func NewManagerQuiet() *Manager {
	return &Manager{statePath: paths.File("state.json")}
}

// LegacyStatePath returns the absolute legacy ~/.owlctl/state.json path when it
// exists and the current state file does not; otherwise "".
func (m *Manager) LegacyStatePath() string {
	legacy := paths.LegacyStatePath()
	if legacy == "" || absPath(legacy) == absPath(m.statePath) {
		return ""
	}
	if _, err := os.Stat(m.statePath); err == nil {
		return ""
	}
	if _, err := os.Stat(legacy); err != nil {
		return ""
	}
	return legacy
}

func absPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// activeInstance returns the currently active instance name.
// Reads OWLCTL_ACTIVE_INSTANCE env var; falls back to "default".
func activeInstance() string {
	if inst := os.Getenv("OWLCTL_ACTIVE_INSTANCE"); inst != "" {
		return inst
	}
	return "default"
}

// activeProduct returns the currently active product name (e.g. "vbr", "azure").
// Reads OWLCTL_ACTIVE_PRODUCT env var; falls back to SelectedProfile from settings.json.
func activeProduct() string {
	if prod := os.Getenv("OWLCTL_ACTIVE_PRODUCT"); prod != "" {
		return prod
	}
	return selectedProfileFromSettings()
}

// selectedProfileFromSettings reads SelectedProfile from settings.json without
// importing the utils package (which would create a circular dependency).
// Returns empty string if the file cannot be found, read, or parsed.
func selectedProfileFromSettings() string {
	data, err := os.ReadFile(paths.File("settings.json"))
	if err != nil {
		return ""
	}
	var s struct {
		SelectedProfile string `json:"selectedProfile"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return ""
	}
	return s.SelectedProfile
}

// Load reads the state file from disk
// Returns a new empty state if file doesn't exist
func (m *Manager) Load() (*State, error) {
	readPath := m.statePath
	if _, err := os.Stat(readPath); os.IsNotExist(err) {
		// Read-only fallback to legacy state so drift checks never pass silently.
		// Save always writes to m.statePath; the legacy file is never modified.
		legacy := m.LegacyStatePath()
		if legacy == "" {
			// Return new empty state if file doesn't exist
			return NewState(), nil
		}
		readPath = legacy
	}

	data, err := os.ReadFile(readPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	// Initialize instances map if nil (defensive)
	if state.Instances == nil {
		state.Instances = make(map[string]*InstanceState)
	}

	// Migrate old state versions forward
	if state.Version < CurrentStateVersion {
		migrateState(&state)
	}

	return &state, nil
}

// migrateState upgrades an older state to CurrentStateVersion in-memory.
// The migrated state is persisted on the next Save() call.
func migrateState(s *State) {
	// v1 → v2: populate Origin field (operates on legacy Resources field)
	if s.Version < 2 {
		for _, r := range s.Resources {
			if r.Origin == "" {
				if r.Type == "VBRJob" {
					r.Origin = "applied"
				} else {
					r.Origin = "observed"
				}
			}
		}
	}

	// v2 → v3: History field added (no migration needed - omitempty handles it)

	// v3 → v4: merge flat Resources into Instances["default"]
	// Merges rather than overwrites to avoid discarding any existing default instance data.
	if s.Version < 4 && s.Resources != nil {
		if s.Instances == nil {
			s.Instances = make(map[string]*InstanceState)
		}
		defaultInst, ok := s.Instances["default"]
		if !ok || defaultInst == nil {
			s.Instances["default"] = &InstanceState{Resources: s.Resources}
		} else {
			if defaultInst.Resources == nil {
				defaultInst.Resources = s.Resources
			} else {
				for name, r := range s.Resources {
					defaultInst.Resources[name] = r
				}
			}
		}
		s.Resources = nil
	}

	s.Version = CurrentStateVersion
}

// Save writes the state to disk atomically using temp file + rename
func (m *Manager) Save(state *State) error {
	// Ensure directory exists
	dir := filepath.Dir(m.statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write atomically: write to temp file, then rename
	tmpFile, err := os.CreateTemp(dir, "state.json.tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp state file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure temp file is cleaned up on error
	defer func() {
		// Best-effort cleanup of temp file if it still exists
		if _, statErr := os.Stat(tmpPath); statErr == nil {
			_ = os.Remove(tmpPath)
		}
	}()

	// Write data to temp file
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temp state file: %w", err)
	}

	// Close temp file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp state file: %w", err)
	}

	// Atomically replace the old state file with the new one
	if err := os.Rename(tmpPath, m.statePath); err != nil {
		return fmt.Errorf("failed to rename temp state file: %w", err)
	}

	return nil
}

// GetStatePath returns the path to the state file
func (m *Manager) GetStatePath() string {
	return m.statePath
}

// UpdateResource loads state, updates a resource under the active instance, and saves.
// Stamps the active product onto the InstanceState if not already set.
func (m *Manager) UpdateResource(resource *Resource) error {
	state, err := m.Load()
	if err != nil {
		return err
	}

	instName := activeInstance()
	inst := state.getInstance(instName)
	if inst.Product == "" {
		inst.Product = activeProduct()
	}
	state.SetResource(instName, resource)

	return m.Save(state)
}

// RemoveResource loads state, removes a resource from the active instance, and saves
func (m *Manager) RemoveResource(name string) error {
	state, err := m.Load()
	if err != nil {
		return err
	}

	state.DeleteResource(activeInstance(), name)

	return m.Save(state)
}

// GetResource loads state and retrieves a single resource from the active instance
func (m *Manager) GetResource(name string) (*Resource, error) {
	state, err := m.Load()
	if err != nil {
		return nil, err
	}

	resource, exists := state.GetResource(activeInstance(), name)
	if !exists {
		return nil, fmt.Errorf("resource '%s' not found in state", name)
	}

	return resource, nil
}

// ListResources loads state and lists resources of the given type from the active instance
func (m *Manager) ListResources(resourceType string) ([]*Resource, error) {
	state, err := m.Load()
	if err != nil {
		return nil, err
	}

	return state.ListResources(activeInstance(), resourceType), nil
}

// StateExists reports whether state can be read: the resolved state file, or
// a legacy ~/.owlctl/state.json that Load falls back to.
func (m *Manager) StateExists() bool {
	if _, err := os.Stat(m.statePath); err == nil {
		return true
	}
	return m.LegacyStatePath() != ""
}
