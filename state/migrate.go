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

	var parsed State
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("source %s is not valid state: %w", from, err)
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
