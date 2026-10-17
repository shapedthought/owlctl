package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/resources"
	"github.com/shapedthought/owlctl/state"
)

const mockConfigBackupJSON = `{"isEnabled":true,"backupRepositoryId":"repo-1","restorePointsToKeep":10,"encryption":{"isEnabled":true}}`

func withMockConfigBackup(t *testing.T, raw string) {
	t.Helper()
	orig := fetchConfigBackup
	fetchConfigBackup = func(models.Profile) json.RawMessage { return json.RawMessage(raw) }
	t.Cleanup(func() { fetchConfigBackup = orig })
}

func findVBRResource(t *testing.T, kind string) ResourceExporter {
	t.Helper()
	pe := findProductExporter("vbr")
	if pe == nil {
		t.Fatal("vbr product exporter not registered")
	}
	for _, r := range pe.Resources {
		if r.Kind == kind {
			return r
		}
	}
	t.Fatalf("kind %s not registered", kind)
	return ResourceExporter{}
}

func TestConfigBackupRegistered(t *testing.T) {
	r := findVBRResource(t, resources.KindVBRConfigurationBackup)
	if r.FolderName != "config-backup" {
		t.Errorf("FolderName = %q, want config-backup", r.FolderName)
	}
	if !r.IsSingleton {
		t.Error("expected IsSingleton to be true")
	}
	if r.ExportAll == nil || r.SnapshotAll == nil {
		t.Error("expected both ExportAll and SnapshotAll to be set")
	}
	// The per-item kinds stay non-singleton.
	for _, k := range []string{"VBRJob", "VBRRepository", "VBRScaleOutRepository", "VBREncryptionPassword", "VBRKmsServer"} {
		if findVBRResource(t, k).IsSingleton {
			t.Errorf("%s should not be a singleton", k)
		}
	}
}

func TestExportAll_WritesConfigBackupFile(t *testing.T) {
	withMockConfigBackup(t, mockConfigBackupJSON)
	r := findVBRResource(t, resources.KindVBRConfigurationBackup)

	outDir := filepath.Join(t.TempDir(), r.FolderName)
	if err := r.ExportAll(outDir, models.Profile{}); err != nil {
		t.Fatalf("ExportAll: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "configuration-backup.yaml"))
	if err != nil {
		t.Fatalf("expected configuration-backup.yaml: %v", err)
	}
	content := string(data)
	for _, want := range []string{"kind: VBRConfigurationBackup", "name: ConfigurationBackup", "restorePointsToKeep: 10"} {
		if !strings.Contains(content, want) {
			t.Errorf("exported YAML missing %q:\n%s", want, content)
		}
	}
}

func TestExportAll_ConfigBackupInvalidResponse(t *testing.T) {
	withMockConfigBackup(t, `not json`)
	r := findVBRResource(t, resources.KindVBRConfigurationBackup)
	outDir := t.TempDir()
	if err := r.ExportAll(outDir, models.Profile{}); err == nil {
		t.Fatal("expected error for invalid response")
	}
	if _, err := os.Stat(filepath.Join(outDir, "configuration-backup.yaml")); err == nil {
		t.Error("no file should be written on error")
	}
}

func TestSnapshotAll_IncludesConfigBackup(t *testing.T) {
	t.Setenv("OWLCTL_SETTINGS_PATH", t.TempDir())
	withMockConfigBackup(t, mockConfigBackupJSON)
	r := findVBRResource(t, resources.KindVBRConfigurationBackup)

	if err := r.SnapshotAll(models.Profile{}); err != nil {
		t.Fatalf("SnapshotAll: %v", err)
	}

	res, err := state.NewManager().GetResource(configBackupStateKey)
	if err != nil {
		t.Fatalf("expected config backup in state: %v", err)
	}
	if res.Type != resources.KindVBRConfigurationBackup || res.Origin != "observed" {
		t.Errorf("unexpected state entry: type=%s origin=%s", res.Type, res.Origin)
	}
}

// Every kind that snapshot/apply can write into state.json must have an export
// folder, otherwise 'export --all --from-state' buckets it under unknown/.
// KindVBRSOBR is an unused alias; state records VBRScaleOutRepository.
func TestFromStateKindsAllHaveFolder(t *testing.T) {
	stateKinds := []string{
		resources.KindVBRJob,
		resources.KindVBRRepository,
		resources.KindVBRScaleOutRepository,
		resources.KindVBREncryptionPassword,
		resources.KindVBRKmsServer,
		resources.KindVBRConfigurationBackup,
	}
	folders := registryKindToFolder()
	for _, k := range stateKinds {
		if folders[k] == "" {
			t.Errorf("kind %s has no folder mapping", k)
		}
	}
	// Every kind with from-state ignore fields must also be mapped.
	for k := range fromStateIgnoreFields {
		if folders[k] == "" {
			t.Errorf("kind %s in fromStateIgnoreFields has no folder mapping", k)
		}
	}
}

func TestExportFromState_ConfigBackupNotUnknown(t *testing.T) {
	inst := &state.InstanceState{
		Product: "vbr",
		Resources: map[string]*state.Resource{
			configBackupStateKey: {
				Type: resources.KindVBRConfigurationBackup,
				ID:   configBackupEndpoint,
				Name: configBackupStateKey,
				Spec: map[string]interface{}{"isEnabled": true},
			},
		},
	}
	outDir := t.TempDir()
	exportInstanceFromState("default", inst, outDir)

	if _, err := os.Stat(filepath.Join(outDir, "unknown")); err == nil {
		t.Error("config backup must not be written under unknown/")
	}
	if _, err := os.Stat(filepath.Join(outDir, "config-backup", "configuration-backup.yaml")); err != nil {
		t.Errorf("expected file under config-backup/: %v", err)
	}
}
