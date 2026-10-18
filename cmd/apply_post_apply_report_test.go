package cmd

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/remediation"
	"github.com/shapedthought/owlctl/resources"
	"github.com/shapedthought/owlctl/state"
)

// jobUpdateTestConfig is the real job PostApply wiring (applyJobState with the
// mocked client) over a stubbed existing job and PUT.
func jobUpdateTestConfig(t *testing.T) ResourceApplyConfig {
	t.Helper()
	orig := putResource
	putResource = func(string, interface{}, models.Profile) ([]byte, error) { return nil, nil }
	t.Cleanup(func() { putResource = orig })
	t.Setenv("OWLCTL_SETTINGS_PATH", t.TempDir())

	cfg := jobApplyConfig
	cfg.PreparePayload = nil
	cfg.FetchCurrent = func(string, models.Profile) (json.RawMessage, string, error) {
		return json.RawMessage(`{"id":"job-42","name":"owlctl-test-job","description":"old","isDisabled":false}`), "job-42", nil
	}
	return cfg
}

func jobUpdateTestSpec(extra map[string]interface{}) resources.ResourceSpec {
	spec := map[string]interface{}{"name": "owlctl-test-job"}
	for k, v := range extra {
		spec[k] = v
	}
	return resources.ResourceSpec{
		APIVersion: "owlctl.veeam.com/v1",
		Kind:       "VBRJob",
		Metadata:   resources.Metadata{Name: "owlctl-test-job"},
		Spec:       spec,
	}
}

func savedJob(t *testing.T) *state.Resource {
	t.Helper()
	res, err := state.NewManager().GetResource("owlctl-test-job")
	if err != nil {
		t.Fatalf("state not saved: %v", err)
	}
	return res
}

func TestApplyResourceSpec_RefusedIsDisabledNotReportedOrRecorded(t *testing.T) {
	cfg := jobUpdateTestConfig(t)
	withMockJobState(t, &mockJobStateClient{disabled: false, setErr: errors.New("Could not disable job [x]")})
	spec := jobUpdateTestSpec(map[string]interface{}{"description": "new", "isDisabled": true})

	var result ApplyResult
	out := captureStdout(t, func() {
		result = applyResourceSpec(spec, cfg, models.Profile{}, false, &remediation.Config{})
	})

	if strings.Contains(out, "Applied: isDisabled") {
		t.Errorf("refused isDisabled must not be reported as applied:\n%s", out)
	}
	if !strings.Contains(out, "Applied: description: \"old\" -> \"new\"") && !strings.Contains(out, "Applied: description") {
		t.Errorf("description should still be reported:\n%s", out)
	}
	if !strings.Contains(out, "1 field(s) applied.") {
		t.Errorf("count should exclude isDisabled:\n%s", out)
	}
	if n := strings.Count(out, "Warning: isDisabled: true was not applied, VBR refused"); n != 1 {
		t.Errorf("warning printed %d times, want 1:\n%s", n, out)
	}
	var reported alreadyReportedError
	if !errors.As(result.Error, &reported) {
		t.Errorf("expected alreadyReportedError, got %v", result.Error)
	}

	res := savedJob(t)
	if got := res.Spec["isDisabled"]; got != false {
		t.Errorf("state isDisabled = %v, want actual value false", got)
	}
	if res.Spec["description"] != "new" {
		t.Errorf("other spec fields should be saved, got %v", res.Spec["description"])
	}
	ev := res.History[len(res.History)-1]
	for _, f := range ev.Fields {
		if f == "isDisabled" {
			t.Errorf("event fields must not include isDisabled: %v", ev.Fields)
		}
	}
	if len(ev.Fields) != 1 || ev.Fields[0] != "description" {
		t.Errorf("event fields = %v, want [description]", ev.Fields)
	}
	if spec.Spec["isDisabled"] != true {
		t.Error("caller's spec must not be mutated")
	}
}

func TestApplyResourceSpec_ReadBackFailureDropsIsDisabledFromState(t *testing.T) {
	cfg := jobUpdateTestConfig(t)
	withMockJobState(t, &mockJobStateClient{readErr: errors.New("boom")})
	spec := jobUpdateTestSpec(map[string]interface{}{"description": "new", "isDisabled": true})

	var result ApplyResult
	out := captureStdout(t, func() {
		result = applyResourceSpec(spec, cfg, models.Profile{}, false, &remediation.Config{})
	})
	if result.Error == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(out, "Applied: isDisabled") {
		t.Errorf("unconfirmed isDisabled must not be reported:\n%s", out)
	}
	if _, ok := savedJob(t).Spec["isDisabled"]; ok {
		t.Error("isDisabled should be dropped from state when its value is unknown")
	}
}

func TestApplyResourceSpec_SuccessfulIsDisabledReportedAfterCall(t *testing.T) {
	cfg := jobUpdateTestConfig(t)
	m := &mockJobStateClient{disabled: false}
	withMockJobState(t, m)
	spec := jobUpdateTestSpec(map[string]interface{}{"description": "new", "isDisabled": true})

	var result ApplyResult
	out := captureStdout(t, func() {
		result = applyResourceSpec(spec, cfg, models.Profile{}, false, &remediation.Config{})
	})
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	want := "  Applied: isDisabled: false -> true (via POST jobs/job-42/disable)"
	if !strings.Contains(out, want) {
		t.Fatalf("missing %q in:\n%s", want, out)
	}
	if strings.Index(out, "Setting job state: disable") > strings.Index(out, want) {
		t.Errorf("isDisabled Applied line must follow the enable/disable call:\n%s", out)
	}
	if !strings.Contains(out, "1 field(s) applied.") {
		t.Errorf("count should cover the PUT fields only:\n%s", out)
	}

	res := savedJob(t)
	if res.Spec["isDisabled"] != true {
		t.Errorf("state isDisabled = %v, want true", res.Spec["isDisabled"])
	}
	fields := res.History[len(res.History)-1].Fields
	if len(fields) != 2 {
		t.Errorf("event fields = %v, want description and isDisabled", fields)
	}
}

func TestApplyResourceSpec_SpecWithoutIsDisabledUnchanged(t *testing.T) {
	cfg := jobUpdateTestConfig(t)
	m := &mockJobStateClient{}
	withMockJobState(t, m)
	spec := jobUpdateTestSpec(map[string]interface{}{"description": "new"})

	var result ApplyResult
	out := captureStdout(t, func() {
		result = applyResourceSpec(spec, cfg, models.Profile{}, false, &remediation.Config{})
	})
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if m.readCalled != 0 || len(m.setCalls) != 0 {
		t.Errorf("no VBR job-state calls expected, got reads=%d sets=%v", m.readCalled, m.setCalls)
	}
	if !strings.Contains(out, "\nApplying: owlctl-test-job\n\n  Applied: description: ") || !strings.Contains(out, "\n1 field(s) applied.\n") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if strings.Contains(out, "isDisabled") {
		t.Errorf("isDisabled must not appear:\n%s", out)
	}
	if _, ok := savedJob(t).Spec["isDisabled"]; ok {
		t.Error("isDisabled should not be in state")
	}
}
