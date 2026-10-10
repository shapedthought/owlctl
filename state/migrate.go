package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shapedthought/owlctl/internal/paths"
)

// MigrateOptions controls MigrateState.
type MigrateOptions struct {
	From   string // source state file; defaults to the legacy ~/.owlctl/state.json
	To     string // destination; defaults to the resolved state path
	Force  bool   // replace an existing destination (it is backed up first)
	DryRun bool   // report what would happen without touching the filesystem
}

// MigrateResult describes what MigrateState did (or would do with DryRun).
type MigrateResult struct {
	From             string
	To               string
	SourceBackup     string // backup written next to the source
	DestBackup       string // backup of a replaced destination (--force only)
	NothingToMigrate bool
}

// MigrateState moves a state file (any version) to the current state location.
// The source is backed up before it is removed, so a copy always exists.
func MigrateState(opts MigrateOptions) (*MigrateResult, error) {
	from, to := opts.From, opts.To
	if from == "" {
		from = paths.LegacyStatePath()
		if from == "" {
			return nil, fmt.Errorf("cannot determine home directory; use --from")
		}
	}
	if to == "" {
		to = NewManagerQuiet().GetStatePath()
	}
	from, to = absPath(from), absPath(to)
	res := &MigrateResult{From: from, To: to}

	data, err := os.ReadFile(from)
	if os.IsNotExist(err) {
		res.NothingToMigrate = true
		return res, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read source state: %w", err)
	}
	if from == to {
		return nil, fmt.Errorf("source and destination are the same file: %s", from)
	}

	if err := validateStateJSON(data); err != nil {
		return nil, fmt.Errorf("source %s is not owlctl state: %w", from, err)
	}

	_, statErr := os.Stat(to)
	destExists := statErr == nil
	if destExists && !opts.Force {
		return nil, fmt.Errorf("destination %s already exists; use --force to replace it (it will be backed up)", to)
	}

	stamp := time.Now().UTC().Format("20060102T150405Z")
	res.SourceBackup = from + ".bak-" + stamp
	if destExists {
		res.DestBackup = to + ".bak-" + stamp
	}
	if opts.DryRun {
		return res, nil
	}

	lock := NewLock()
	if err := lock.Acquire(); err != nil {
		return nil, err
	}
	defer func() { _ = lock.Release() }()

	if err := writeFileSync(res.SourceBackup, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write backup: %w", err)
	}
	if destExists {
		old, err := os.ReadFile(to)
		if err != nil {
			return nil, fmt.Errorf("failed to read destination for backup: %w", err)
		}
		if err := writeFileSync(res.DestBackup, old, 0600); err != nil {
			return nil, fmt.Errorf("failed to back up destination: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(to), 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}
	if err := os.Rename(from, to); err != nil {
		// Likely a cross-filesystem move: copy, verify, then remove the source.
		if err := copyVerified(data, to); err != nil {
			return nil, err
		}
		if err := os.Remove(from); err != nil {
			return nil, fmt.Errorf("copied to %s but failed to remove source: %w", to, err)
		}
	}
	return res, nil
}

// validateStateJSON checks that data is an owlctl state file (v1 to current):
// a "version" in range, a "resources" map (v1-v3) or "instances" map (v4),
// and that it decodes into State.
func validateStateJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("invalid JSON object: %w", err)
	}
	vraw, ok := raw["version"]
	if !ok {
		return fmt.Errorf("missing \"version\" field")
	}
	var version int
	if err := json.Unmarshal(vraw, &version); err != nil {
		return fmt.Errorf("\"version\" is not an integer")
	}
	if version < 1 || version > CurrentStateVersion {
		return fmt.Errorf("unsupported state version %d (expected 1-%d)", version, CurrentStateVersion)
	}
	key := "resources"
	if version >= 4 {
		key = "instances"
	}
	field, ok := raw[key]
	if !ok {
		return fmt.Errorf("missing %q field for state v%d", key, version)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(field, &obj); err != nil || obj == nil {
		return fmt.Errorf("%q must be an object", key)
	}
	var parsed State
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("invalid state structure: %w", err)
	}
	return nil
}

func writeFileSync(path string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// copyVerified writes data to a temp file beside dest, syncs it, renames it
// into place and re-reads it to confirm the content.
func copyVerified(data []byte, dest string) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), "state.json.tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write destination: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to sync destination: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return fmt.Errorf("failed to move into place: %w", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || !bytes.Equal(got, data) {
		return fmt.Errorf("verification of %s failed; source left in place", dest)
	}
	return nil
}
