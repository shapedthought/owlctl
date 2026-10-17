package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/remediation"
)

func renderDryRun(r *DryRunReport) string {
	var buf bytes.Buffer
	printDryRun(&buf, r)
	return buf.String()
}

func TestDryRun_NoChanges(t *testing.T) {
	out := renderDryRun(newDryRunUpdateReport("job1", "VBRJob", nil, nil, ""))
	if !strings.Contains(out, dryRunNoChangesMessage) || !strings.Contains(out, "No changes detected") {
		t.Fatalf("missing no-changes message:\n%s", out)
	}
	if strings.Contains(out, "field(s)") {
		t.Fatalf("unexpected summary:\n%s", out)
	}
	if r := newDryRunUpdateReport("job1", "VBRJob", nil, nil, ""); r.Action != dryRunNoChange {
		t.Fatalf("action = %q", r.Action)
	}
}

func TestDryRun_ModifiedAddedRemoved(t *testing.T) {
	changes := []DryRunChange{
		{Path: "description", Old: "a", New: "b", Severity: SeverityInfo},
		{Path: "storage.retentionPolicy.quantity", Old: float64(7), New: float64(3), Severity: SeverityCritical},
		{Path: "newField", Old: nil, New: true, Severity: SeverityWarning},
		{Path: "oldField", Old: "x", New: nil, Severity: SeverityInfo},
	}
	out := renderDryRun(newDryRunUpdateReport("job1", "VBRJob", changes, nil, ""))
	for _, want := range []string{
		`  INFO ~ description: "a" -> "b"`,
		`  CRITICAL ~ storage.retentionPolicy.quantity: 7 -> 3`,
		`  WARNING + newField: <none> -> true`,
		`  INFO - oldField: "x" -> <none>`,
		"4 field(s) would be changed.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestDryRun_LongStringsNotTruncated(t *testing.T) {
	long := strings.Repeat("x", 120)
	out := renderDryRun(newDryRunUpdateReport("j", "VBRJob",
		[]DryRunChange{{Path: "description", Old: long, New: "short", Severity: SeverityInfo}}, nil, ""))
	if !strings.Contains(out, `"`+long+`"`) || strings.Contains(out, "...") {
		t.Fatalf("string was truncated:\n%s", out)
	}
	// job diff (drift) output keeps its truncation.
	if got := formatValue(long); !strings.Contains(got, "...") {
		t.Fatalf("formatValue should still truncate, got %q", got)
	}
}

func TestDryRun_SkippedAndPostApply(t *testing.T) {
	out := renderDryRun(newDryRunUpdateReport("j", "VBRJob", nil,
		[]SkippedField{{Path: "isDisabled", Reason: "policy"}}, "POST jobs/abc/disable (isDisabled: false -> true)"))
	for _, want := range []string{"Skipped (not sent to VBR):", "  - isDisabled", "    policy", "Would call: POST jobs/abc/disable"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "No changes detected") {
		t.Errorf("skipped fields should not read as no changes:\n%s", out)
	}
}

func TestDryRun_Create(t *testing.T) {
	out := renderDryRun(newDryRunCreateReport("j", "VBRJob", map[string]interface{}{"name": "j", "storage": map[string]interface{}{"a": 1}}, ""))
	for _, want := range []string{"Action: Would CREATE new resource", "Configuration to be created:", `name: "j"`, "storage: {1 fields}"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestDryRun_JSONShape(t *testing.T) {
	r := newDryRunUpdateReport("job1", "VBRJob",
		[]DryRunChange{{Path: "description", Old: "a", New: "b", Severity: SeverityInfo}},
		[]SkippedField{{Path: "isDisabled"}}, "POST jobs/abc/disable (isDisabled: false -> true)")
	var buf bytes.Buffer
	if err := writeDryRunJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if got["resource"] != "job1" || got["kind"] != "VBRJob" || got["action"] != "would-update" {
		t.Fatalf("unexpected header fields: %v", got)
	}
	changes := got["changes"].([]interface{})
	c := changes[0].(map[string]interface{})
	if c["path"] != "description" || c["old"] != "a" || c["new"] != "b" || c["severity"] != "INFO" {
		t.Fatalf("unexpected change: %v", c)
	}
	if got["skipped"].([]interface{})[0].(map[string]interface{})["path"] != "isDisabled" {
		t.Fatalf("unexpected skipped: %v", got["skipped"])
	}
	if got["postApply"] == "" {
		t.Fatalf("postApply missing: %v", got)
	}

	// No-change reports still carry empty arrays, not null.
	buf.Reset()
	_ = writeDryRunJSON(&buf, newDryRunUpdateReport("job1", "VBRJob", nil, nil, ""))
	if !strings.Contains(buf.String(), `"changes": []`) || !strings.Contains(buf.String(), `"action": "no-change"`) {
		t.Fatalf("unexpected no-change JSON:\n%s", buf.String())
	}
}

func TestNewDryRunChangesFromDrifts(t *testing.T) {
	got := newDryRunChangesFromDrifts([]Drift{
		{Path: "b", Action: "modified", State: "new", VBR: "cur", Severity: SeverityWarning},
		{Path: "a", Action: "removed", State: "v", Severity: SeverityInfo},
		{Path: "c", Action: "added", VBR: "cur", Severity: SeverityInfo},
	})
	if len(got) != 3 || got[0].Path != "a" || got[0].Old != nil || got[0].New != "v" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got[1].Old != "cur" || got[1].New != "new" || got[2].New != nil || got[2].Old != "cur" {
		t.Fatalf("unexpected: %+v", got)
	}
	if line := formatDryRunChange(got[1]); line != `  WARNING ~ b: "cur" -> "new"` {
		t.Fatalf("line = %q", line)
	}
}

func TestNewDryRunChanges_SeverityFromMap(t *testing.T) {
	sm := SeverityMap{"isDisabled": SeverityCritical}
	got := newDryRunChanges([]FieldChange{{Path: "isDisabled"}, {Path: "other"}}, sm)
	if got[0].Severity != SeverityCritical || got[1].Severity != SeverityInfo {
		t.Fatalf("unexpected severities: %+v", got)
	}
	if got := newDryRunChanges([]FieldChange{{Path: "x"}}, nil); got[0].Severity != SeverityInfo {
		t.Fatalf("nil map should give INFO, got %v", got[0].Severity)
	}
}

func TestPlannedJobState_RealID(t *testing.T) {
	spec := map[string]interface{}{"isDisabled": true}
	got := plannedJobState(spec, map[string]interface{}{"isDisabled": false}, "abc-123")
	if got != "POST jobs/abc-123/disable (isDisabled: false -> true)" {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "{id}") {
		t.Fatalf("literal {id} in %q", got)
	}
	if got := plannedJobState(spec, nil, ""); got != "POST jobs/<new id>/disable (after create)" {
		t.Fatalf("create: got %q", got)
	}
}

func jobDryRunTestConfig(existing string, id string, post func(spec map[string]interface{}, id string, _ models.Profile) error) ResourceApplyConfig {
	cfg := jobSkipTestConfig(&[]bool{})
	cfg.FetchCurrent = func(string, models.Profile) (json.RawMessage, string, error) {
		if existing == "" {
			return nil, "", nil
		}
		return json.RawMessage(existing), id, nil
	}
	if post != nil {
		cfg.PostApply = post
	}
	return cfg
}

func TestApplyResourceSpec_DryRunWouldCallHasRealID(t *testing.T) {
	cfg := jobDryRunTestConfig(`{"id":"job-42","name":"owlctl-test-job","isDisabled":false}`, "job-42", nil)
	out := captureStdout(t, func() {
		applyResourceSpec(jobSkipTestSpec(), cfg, models.Profile{}, true, &remediation.Config{})
	})
	if !strings.Contains(out, "Would call: POST jobs/job-42/disable (isDisabled: false -> true)") {
		t.Fatalf("expected real id in Would call, got:\n%s", out)
	}
	if !strings.Contains(out, "INFO ~ isDisabled: false -> true") {
		t.Fatalf("expected shared change line, got:\n%s", out)
	}
}

func TestApplyResourceSpec_DryRunCreateWouldCall(t *testing.T) {
	cfg := jobDryRunTestConfig("", "", nil)
	out := captureStdout(t, func() {
		applyResourceSpec(jobSkipTestSpec(), cfg, models.Profile{}, true, &remediation.Config{})
	})
	if !strings.Contains(out, "Would call: POST jobs/<new id>/disable (after create)") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestApplyResourceSpec_RefusalPrintedOnce(t *testing.T) {
	refusal := errors.New("isDisabled: true was not applied, VBR refused to disable the job: Could not disable job")
	cfg := jobDryRunTestConfig("", "", func(map[string]interface{}, string, models.Profile) error { return refusal })
	t.Setenv("OWLCTL_SETTINGS_PATH", t.TempDir())

	var result ApplyResult
	out := captureStdout(t, func() {
		result = applyResourceSpec(jobSkipTestSpec(), cfg, models.Profile{}, false, &remediation.Config{})
	})
	if n := strings.Count(out, "was not applied"); n != 1 {
		t.Fatalf("refusal printed %d times, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, "Warning: isDisabled: true was not applied") {
		t.Fatalf("missing warning line:\n%s", out)
	}
	var reported alreadyReportedError
	if !errors.As(result.Error, &reported) {
		t.Fatalf("expected alreadyReportedError so the caller does not print it again, got %v", result.Error)
	}
	if !errors.Is(result.Error, refusal) {
		t.Fatalf("original error should stay reachable")
	}
}
