package state

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/shapedthought/owlctl/utils"
)

const legacyV3 = `{"version":3,"resources":{"Repo A":{"type":"VBRRepository","id":"1","name":"Repo A","origin":"observed","spec":{"x":1}}}}`

// setupLegacy chdirs to a fresh dir (env unset) and points HOME at another.
// Returns cwd and the legacy state path.
func setupLegacy(t *testing.T, content string) (string, string) {
	t.Helper()
	cwd := t.TempDir()
	home := t.TempDir()
	t.Chdir(cwd)
	t.Setenv("OWLCTL_SETTINGS_PATH", "")
	t.Setenv("OWLCTL_ACTIVE_INSTANCE", "")
	t.Setenv("OWLCTL_ACTIVE_PRODUCT", "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	legacy := filepath.Join(home, ".owlctl", "state.json")
	if content != "" {
		if err := os.MkdirAll(filepath.Dir(legacy), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(legacy, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return cwd, legacy
}

func TestStateAndSettingsColocated(t *testing.T) {
	setupLegacy(t, "")
	settingsDir := filepath.Dir(utils.SettingPath() + "settings.json")
	stateDir := filepath.Dir(NewManagerQuiet().GetStatePath())
	if settingsDir != stateDir {
		t.Errorf("settings dir %q != state dir %q", settingsDir, stateDir)
	}
}

func TestNewManager_NoSideEffects(t *testing.T) {
	setupLegacy(t, "")
	home, _ := os.UserHomeDir()
	NewManager()
	if _, err := os.Stat(filepath.Join(home, ".owlctl")); !os.IsNotExist(err) {
		t.Errorf("NewManager created ~/.owlctl (err=%v)", err)
	}
}

func TestActiveProduct_ReadsColocatedSettings(t *testing.T) {
	cwd, _ := setupLegacy(t, "")
	if err := os.WriteFile(filepath.Join(cwd, "settings.json"), []byte(`{"selectedProfile":"vbr"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := activeProduct(); got != "vbr" {
		t.Errorf("activeProduct = %q, want vbr", got)
	}
}

func TestLegacyStateWarning(t *testing.T) {
	cwd, legacy := setupLegacy(t, legacyV3)
	legacyWarnOnce = sync.Once{}

	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	m := NewManager()
	w.Close()
	os.Stderr = old
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])

	if want := absPath(m.GetStatePath()); !filepath.IsAbs(want) || !strings.Contains(out, want) {
		t.Errorf("warning missing absolute resolved path %q: %s", want, out)
	}
	if !strings.Contains(out, legacy) || !strings.Contains(out, "state migrate") {
		t.Errorf("warning missing legacy path or migrate hint: %s", out)
	}
	if got, _ := os.ReadFile(legacy); string(got) != legacyV3 {
		t.Error("legacy file was modified")
	}

	if err := m.Save(NewState()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cwd, "state.json")); err != nil {
		t.Errorf("write did not go to CWD: %v", err)
	}
	if got, _ := os.ReadFile(legacy); string(got) != legacyV3 {
		t.Error("legacy file changed by Save")
	}
}

func TestStateMigrate_MovesWithBackup(t *testing.T) {
	cwd, legacy := setupLegacy(t, legacyV3)
	res, err := MigrateState(MigrateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(cwd, "state.json")); string(got) != legacyV3 {
		t.Errorf("destination content differs: %s", got)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Error("source still exists")
	}
	if bak, _ := os.ReadFile(res.SourceBackup); string(bak) != legacyV3 {
		t.Error("backup missing or differs")
	}
	if filepath.Dir(res.SourceBackup) != filepath.Dir(legacy) || !strings.Contains(res.SourceBackup, ".bak-") {
		t.Errorf("backup not next to source: %s", res.SourceBackup)
	}
}

func TestStateMigrate_V3Source(t *testing.T) {
	setupLegacy(t, legacyV3)
	if _, err := MigrateState(MigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	st, err := NewManagerQuiet().Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.Version != CurrentStateVersion {
		t.Errorf("version = %d, want %d", st.Version, CurrentStateVersion)
	}
	if _, ok := st.GetResource("default", "Repo A"); !ok {
		t.Error("resource not under instances[default]")
	}
}

func TestStateMigrate_DestinationExists(t *testing.T) {
	cwd, legacy := setupLegacy(t, legacyV3)
	dest := filepath.Join(cwd, "state.json")
	if err := os.WriteFile(dest, []byte(`{"version":4,"instances":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := MigrateState(MigrateOptions{}); err == nil {
		t.Fatal("expected error without --force")
	}
	if got, _ := os.ReadFile(legacy); string(got) != legacyV3 {
		t.Error("source changed on refusal")
	}
	if got, _ := os.ReadFile(dest); string(got) != `{"version":4,"instances":{}}` {
		t.Error("destination changed on refusal")
	}

	res, err := MigrateState(MigrateOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if bak, _ := os.ReadFile(res.DestBackup); string(bak) != `{"version":4,"instances":{}}` {
		t.Error("destination not backed up")
	}
	if got, _ := os.ReadFile(dest); string(got) != legacyV3 {
		t.Error("destination not replaced")
	}
}

func TestStateMigrate_DryRun(t *testing.T) {
	cwd, legacy := setupLegacy(t, legacyV3)
	if _, err := MigrateState(MigrateOptions{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cwd, "state.json")); !os.IsNotExist(err) {
		t.Error("dry-run created destination")
	}
	entries, _ := os.ReadDir(filepath.Dir(legacy))
	if len(entries) != 1 {
		t.Errorf("dry-run changed source dir: %v", entries)
	}
}

func TestStateMigrate_NothingToMigrate(t *testing.T) {
	setupLegacy(t, "")
	res, err := MigrateState(MigrateOptions{})
	if err != nil || !res.NothingToMigrate {
		t.Errorf("res=%+v err=%v, want NothingToMigrate", res, err)
	}
}

func TestStateMigrate_NonStateJSON(t *testing.T) {
	for _, content := range []string{
		`{"hello":1}`,
		`{"version":9,"instances":{}}`,
		`{"version":4}`,
		`{"version":3,"resources":[]}`,
		`[]`,
	} {
		cwd, legacy := setupLegacy(t, content)
		if _, err := MigrateState(MigrateOptions{}); err == nil {
			t.Errorf("%s: expected error", content)
		}
		if got, _ := os.ReadFile(legacy); string(got) != content {
			t.Errorf("%s: source touched", content)
		}
		if entries, _ := os.ReadDir(filepath.Dir(legacy)); len(entries) != 1 {
			t.Errorf("%s: backup written: %v", content, entries)
		}
		if _, err := os.Stat(filepath.Join(cwd, "state.json")); !os.IsNotExist(err) {
			t.Errorf("%s: destination created", content)
		}
	}
}

func TestLegacyFallback_ReadOnlySeesLegacy_WriteLeavesItUntouched(t *testing.T) {
	cwd, legacy := setupLegacy(t, legacyV3)
	m := NewManagerQuiet()

	if !m.StateExists() {
		t.Fatal("StateExists should be true via legacy fallback")
	}
	st, err := m.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.GetResource("default", "Repo A"); !ok {
		t.Fatal("read-only Load did not see legacy resources")
	}
	if _, err := os.Stat(filepath.Join(cwd, "state.json")); !os.IsNotExist(err) {
		t.Error("read created state.json")
	}

	if err := m.UpdateResource(&Resource{Type: "VBRRepository", ID: "2", Name: "Repo B", Origin: "observed"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(legacy); string(got) != legacyV3 {
		t.Error("legacy file changed by write")
	}
	if _, err := os.Stat(filepath.Join(cwd, "state.json")); err != nil {
		t.Errorf("write did not go to resolved path: %v", err)
	}
}

func TestStateMigrate_InvalidSource(t *testing.T) {
	_, legacy := setupLegacy(t, "{not json")
	if _, err := MigrateState(MigrateOptions{}); err == nil {
		t.Fatal("expected error")
	}
	if got, _ := os.ReadFile(legacy); string(got) != "{not json" {
		t.Error("source touched")
	}
	entries, _ := os.ReadDir(filepath.Dir(legacy))
	if len(entries) != 1 {
		t.Errorf("backup written for invalid source: %v", entries)
	}
}
