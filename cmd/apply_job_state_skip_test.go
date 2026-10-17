package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/remediation"
	"github.com/shapedthought/owlctl/resources"
)

func skipIsDisabledCfg() *remediation.Config {
	return &remediation.Config{Job: map[string]remediation.FieldPolicy{"isDisabled": remediation.PolicySkip}}
}

// jobSkipTestConfig is a stubbed job apply config that records the isDisabled
// value PostApply sees, without contacting VBR.
func jobSkipTestConfig(called *[]bool) ResourceApplyConfig {
	return ResourceApplyConfig{
		Kind:     "VBRJob",
		Endpoint: "jobs",
		Mode:     ApplyCreateOrUpdate,
		FetchCurrent: func(string, models.Profile) (json.RawMessage, string, error) {
			return nil, "", nil
		},
		PostCreate: func(map[string]interface{}, models.Profile, string) (string, error) {
			return "job-1", nil
		},
		PostApply: func(spec map[string]interface{}, id string, _ models.Profile) error {
			if want, ok := desiredJobDisabled(spec); ok {
				*called = append(*called, want)
			}
			return nil
		},
		PlanPostApply: plannedJobState,
	}
}

func jobSkipTestSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		APIVersion: "owlctl.veeam.com/v1",
		Kind:       "VBRJob",
		Metadata:   resources.Metadata{Name: "owlctl-test-job"},
		Spec:       map[string]interface{}{"name": "owlctl-test-job", "isDisabled": true},
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestApplyResourceSpec_IsDisabledSkippedByPolicy(t *testing.T) {
	t.Setenv("OWLCTL_SETTINGS_PATH", t.TempDir())
	var called []bool
	cfg := jobSkipTestConfig(&called)

	captureStdout(t, func() {
		res := applyResourceSpec(jobSkipTestSpec(), cfg, models.Profile{}, false, skipIsDisabledCfg())
		if res.Error != nil {
			t.Errorf("unexpected error: %v", res.Error)
		}
	})
	if len(called) != 0 {
		t.Fatalf("expected no enable/disable with skip policy, got %v", called)
	}

	out := captureStdout(t, func() {
		applyResourceSpec(jobSkipTestSpec(), cfg, models.Profile{}, true, skipIsDisabledCfg())
	})
	if strings.Contains(out, "Would call:") {
		t.Fatalf("dry-run must not plan a call when isDisabled is skipped, got:\n%s", out)
	}
}

func TestApplyResourceSpec_IsDisabledWithoutPolicyUnchanged(t *testing.T) {
	t.Setenv("OWLCTL_SETTINGS_PATH", t.TempDir())
	var called []bool
	cfg := jobSkipTestConfig(&called)
	empty := &remediation.Config{Job: map[string]remediation.FieldPolicy{}}

	captureStdout(t, func() {
		if res := applyResourceSpec(jobSkipTestSpec(), cfg, models.Profile{}, false, empty); res.Error != nil {
			t.Errorf("unexpected error: %v", res.Error)
		}
	})
	if len(called) != 1 || !called[0] {
		t.Fatalf("expected one disable call without policy, got %v", called)
	}

	out := captureStdout(t, func() {
		applyResourceSpec(jobSkipTestSpec(), cfg, models.Profile{}, true, empty)
	})
	if !strings.Contains(out, "Would call: POST jobs/") {
		t.Fatalf("expected dry-run plan without policy, got:\n%s", out)
	}
}

func TestPlannedJobStateForDryRun_HonoursSkipPolicy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OWLCTL_SETTINGS_PATH", dir)
	t.Setenv("HOME", dir)
	spec := map[string]interface{}{"isDisabled": true}

	if plan := plannedJobStateForDryRun(spec, nil); plan == "" {
		t.Fatal("expected a plan without remediation config")
	}

	cfgYAML := "job:\n  isDisabled: skip\n"
	if err := os.WriteFile(filepath.Join(dir, "remediation-config.yaml"), []byte(cfgYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if plan := plannedJobStateForDryRun(spec, nil); plan != "" {
		t.Fatalf("expected no plan with skip policy, got %q", plan)
	}
}

func TestPostApplySpec_DropsSkippedFields(t *testing.T) {
	spec := map[string]interface{}{"isDisabled": true, "name": "x", "description": "d"}
	out := postApplySpec(spec, skipIsDisabledCfg(), "VBRJob", []SkippedField{{Path: "description"}})
	if _, ok := out["isDisabled"]; ok {
		t.Error("isDisabled should be dropped by policy")
	}
	if _, ok := out["description"]; ok {
		t.Error("description should be dropped via Skipped")
	}
	if out["name"] != "x" {
		t.Error("name should be kept")
	}
	if _, ok := spec["isDisabled"]; !ok {
		t.Error("input spec must not be mutated")
	}
}
