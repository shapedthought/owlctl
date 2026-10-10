package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupManagerTest creates a temp dir and sets OWLCTL_SETTINGS_PATH.
// Returns the temp dir path and a cleanup function.
func setupManagerTest(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "owlctl-state-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	t.Setenv("OWLCTL_SETTINGS_PATH", tmpDir)
	t.Setenv("OWLCTL_ACTIVE_INSTANCE", "") // ensure default instance

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}
	return tmpDir, cleanup
}

func TestNewManagerUsesSettingsPath(t *testing.T) {
	tmpDir, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()
	expected := filepath.Join(tmpDir, "state.json")
	if m.GetStatePath() != expected {
		t.Errorf("Expected state path %s, got %s", expected, m.GetStatePath())
	}
}

func TestNewManagerFallsBackWithoutEnv(t *testing.T) {
	t.Setenv("OWLCTL_SETTINGS_PATH", "")

	m := NewManager()
	if m.GetStatePath() != "state.json" {
		t.Errorf("Expected state path state.json (current dir), got %s", m.GetStatePath())
	}
}

func TestLoadReturnsEmptyStateWhenNoFile(t *testing.T) {
	_, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()
	state, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if state.Version != CurrentStateVersion {
		t.Errorf("Expected version %d, got %d", CurrentStateVersion, state.Version)
	}
	if state.Instances == nil {
		t.Fatal("Expected Instances map to be initialized")
	}
	if len(state.Instances) != 0 {
		t.Errorf("Expected empty Instances, got %d", len(state.Instances))
	}
}

func TestLoadMigratesV3ToV4(t *testing.T) {
	tmpDir, cleanup := setupManagerTest(t)
	defer cleanup()

	// Write a v3 state with flat resources
	stateJSON := `{
  "version": 3,
  "resources": {
    "TestJob": {
      "type": "VBRJob",
      "id": "abc-123",
      "name": "TestJob",
      "lastApplied": "2025-01-01T00:00:00Z",
      "lastAppliedBy": "admin",
      "origin": "applied",
      "spec": {"key": "value"}
    }
  }
}`
	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, []byte(stateJSON), 0644); err != nil {
		t.Fatalf("Failed to write state file: %v", err)
	}

	m := NewManager()
	state, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if state.Version != CurrentStateVersion {
		t.Errorf("Expected version %d after migration, got %d", CurrentStateVersion, state.Version)
	}
	if state.Resources != nil {
		t.Error("Expected legacy Resources field to be nil after migration")
	}

	r, exists := state.GetResource("default", "TestJob")
	if !exists {
		t.Fatal("Expected TestJob to be migrated into instances[\"default\"]")
	}
	if r.ID != "abc-123" {
		t.Errorf("Expected ID=abc-123, got %s", r.ID)
	}
	if r.Origin != "applied" {
		t.Errorf("Expected Origin=applied, got %s", r.Origin)
	}
}

func TestLoadMigratesV1ToV4(t *testing.T) {
	tmpDir, cleanup := setupManagerTest(t)
	defer cleanup()

	stateJSON := `{
  "version": 1,
  "resources": {
    "MyJob": {
      "type": "VBRJob",
      "id": "job-1",
      "name": "MyJob",
      "lastApplied": "2025-01-01T00:00:00Z",
      "lastAppliedBy": "admin",
      "origin": "",
      "spec": {}
    },
    "MyRepo": {
      "type": "VBRRepository",
      "id": "repo-1",
      "name": "MyRepo",
      "lastApplied": "2025-01-01T00:00:00Z",
      "lastAppliedBy": "admin",
      "origin": "",
      "spec": {}
    }
  }
}`
	statePath := filepath.Join(tmpDir, "state.json")
	if err := os.WriteFile(statePath, []byte(stateJSON), 0644); err != nil {
		t.Fatalf("Failed to write state file: %v", err)
	}

	m := NewManager()
	state, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if state.Version != CurrentStateVersion {
		t.Errorf("Expected version %d after migration, got %d", CurrentStateVersion, state.Version)
	}

	// v1→v2: VBRJob gets origin "applied", others get "observed"
	job, exists := state.GetResource("default", "MyJob")
	if !exists {
		t.Fatal("Expected MyJob in instances[\"default\"] after migration")
	}
	if job.Origin != "applied" {
		t.Errorf("Expected job Origin=applied after migration, got %s", job.Origin)
	}

	repo, exists := state.GetResource("default", "MyRepo")
	if !exists {
		t.Fatal("Expected MyRepo in instances[\"default\"] after migration")
	}
	if repo.Origin != "observed" {
		t.Errorf("Expected repo Origin=observed after migration, got %s", repo.Origin)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	_, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()

	original := NewState()
	now := time.Now().Truncate(time.Second) // Truncate for JSON round-trip
	original.SetResource("default", &Resource{
		Type:          "VBRJob",
		ID:            "job-456",
		Name:          "BackupJob",
		LastApplied:   now,
		LastAppliedBy: "admin",
		Origin:        "applied",
		Spec:          map[string]interface{}{"name": "BackupJob", "isDisabled": false},
	})

	if err := m.Save(original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Version != original.Version {
		t.Errorf("Expected version %d, got %d", original.Version, loaded.Version)
	}

	r, exists := loaded.GetResource("default", "BackupJob")
	if !exists {
		t.Fatal("Expected BackupJob to exist after round-trip")
	}
	if r.ID != "job-456" {
		t.Errorf("Expected ID=job-456, got %s", r.ID)
	}
	if r.Origin != "applied" {
		t.Errorf("Expected Origin=applied, got %s", r.Origin)
	}
	if r.LastAppliedBy != "admin" {
		t.Errorf("Expected LastAppliedBy=admin, got %s", r.LastAppliedBy)
	}
}

func TestSaveCreatesValidJSON(t *testing.T) {
	tmpDir, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()
	state := NewState()
	state.SetResource("default", &Resource{
		Name: "TestRes",
		Type: "VBRJob",
		ID:   "1",
		Spec: map[string]interface{}{"key": "value"},
	})

	if err := m.Save(state); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	statePath := filepath.Join(tmpDir, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Saved file is not valid JSON: %v", err)
	}

	// Verify instances key is present
	if _, ok := parsed["instances"]; !ok {
		t.Error("Expected 'instances' key in saved JSON")
	}
	if _, ok := parsed["resources"]; ok {
		t.Error("Expected legacy 'resources' key to be absent from v4 JSON")
	}
}

// ---- activeInstance ------------------------------------------------------------

func TestActiveInstanceDefaultsToDefault(t *testing.T) {
	t.Setenv("OWLCTL_ACTIVE_INSTANCE", "")
	if got := activeInstance(); got != "default" {
		t.Errorf("Expected activeInstance()=%q, got %q", "default", got)
	}
}

func TestActiveInstanceReadsEnvVar(t *testing.T) {
	t.Setenv("OWLCTL_ACTIVE_INSTANCE", "vbr-prod")
	if got := activeInstance(); got != "vbr-prod" {
		t.Errorf("Expected activeInstance()=%q, got %q", "vbr-prod", got)
	}
}

// ---- Manager convenience methods -----------------------------------------------

func TestUpdateResource(t *testing.T) {
	_, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()

	r := &Resource{
		Type: "VBRJob",
		ID:   "job-1",
		Name: "NewJob",
		Spec: map[string]interface{}{"name": "NewJob"},
	}
	if err := m.UpdateResource(r); err != nil {
		t.Fatalf("UpdateResource failed: %v", err)
	}

	got, err := m.GetResource("NewJob")
	if err != nil {
		t.Fatalf("GetResource failed: %v", err)
	}
	if got.ID != "job-1" {
		t.Errorf("Expected ID=job-1, got %s", got.ID)
	}
}

func TestUpdateResourceOverwrites(t *testing.T) {
	_, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()

	r1 := &Resource{Type: "VBRJob", ID: "job-1", Name: "MyJob", Spec: map[string]interface{}{"version": "v1"}}
	if err := m.UpdateResource(r1); err != nil {
		t.Fatalf("UpdateResource failed: %v", err)
	}

	r2 := &Resource{Type: "VBRJob", ID: "job-1", Name: "MyJob", Spec: map[string]interface{}{"version": "v2"}}
	if err := m.UpdateResource(r2); err != nil {
		t.Fatalf("UpdateResource overwrite failed: %v", err)
	}

	got, err := m.GetResource("MyJob")
	if err != nil {
		t.Fatalf("GetResource failed: %v", err)
	}
	if got.Spec["version"] != "v2" {
		t.Errorf("Expected spec version=v2, got %v", got.Spec["version"])
	}
}

func TestUpdateResourceInstanceScoping(t *testing.T) {
	_, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()

	// Write under vbr-prod
	t.Setenv("OWLCTL_ACTIVE_INSTANCE", "vbr-prod")
	if err := m.UpdateResource(&Resource{Name: "Production", Type: "VBRJob", ID: "1", Spec: map[string]interface{}{}}); err != nil {
		t.Fatalf("UpdateResource failed: %v", err)
	}

	// Write under azure-prod — same name, different instance
	t.Setenv("OWLCTL_ACTIVE_INSTANCE", "azure-prod")
	if err := m.UpdateResource(&Resource{Name: "Production", Type: "AzurePolicy", ID: "2", Spec: map[string]interface{}{}}); err != nil {
		t.Fatalf("UpdateResource failed: %v", err)
	}

	// Read back vbr-prod
	t.Setenv("OWLCTL_ACTIVE_INSTANCE", "vbr-prod")
	vbrRes, err := m.GetResource("Production")
	if err != nil {
		t.Fatalf("GetResource (vbr-prod) failed: %v", err)
	}
	if vbrRes.Type != "VBRJob" {
		t.Errorf("Expected VBRJob in vbr-prod, got %s", vbrRes.Type)
	}

	// Read back azure-prod
	t.Setenv("OWLCTL_ACTIVE_INSTANCE", "azure-prod")
	azureRes, err := m.GetResource("Production")
	if err != nil {
		t.Fatalf("GetResource (azure-prod) failed: %v", err)
	}
	if azureRes.Type != "AzurePolicy" {
		t.Errorf("Expected AzurePolicy in azure-prod, got %s", azureRes.Type)
	}
}

func TestRemoveResource(t *testing.T) {
	_, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()

	m.UpdateResource(&Resource{Name: "ToRemove", Type: "VBRJob", ID: "1"})

	if err := m.RemoveResource("ToRemove"); err != nil {
		t.Fatalf("RemoveResource failed: %v", err)
	}

	_, err := m.GetResource("ToRemove")
	if err == nil {
		t.Error("Expected error after removing resource")
	}
}

func TestRemoveResourceNoErrorWhenMissing(t *testing.T) {
	_, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()

	if err := m.RemoveResource("nonexistent"); err != nil {
		t.Errorf("Expected no error removing nonexistent resource, got: %v", err)
	}
}

func TestGetResourceNotFound(t *testing.T) {
	_, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()

	_, err := m.GetResource("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent resource")
	}
}

func TestListResourcesFiltersByType(t *testing.T) {
	_, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()

	m.UpdateResource(&Resource{Name: "Job1", Type: "VBRJob", ID: "1"})
	m.UpdateResource(&Resource{Name: "Job2", Type: "VBRJob", ID: "2"})
	m.UpdateResource(&Resource{Name: "Repo1", Type: "VBRRepository", ID: "3"})

	jobs, err := m.ListResources("VBRJob")
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("Expected 2 VBRJob resources, got %d", len(jobs))
	}
}

func TestListResourcesEmptyResult(t *testing.T) {
	_, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()

	result, err := m.ListResources("VBREncryptionPassword")
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 results, got %d", len(result))
	}
}

func TestStateExistsFalseBeforeSave(t *testing.T) {
	_, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()

	if m.StateExists() {
		t.Error("Expected StateExists=false before any save")
	}
}

func TestStateExistsTrueAfterSave(t *testing.T) {
	_, cleanup := setupManagerTest(t)
	defer cleanup()

	m := NewManager()
	if err := m.Save(NewState()); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if !m.StateExists() {
		t.Error("Expected StateExists=true after save")
	}
}
