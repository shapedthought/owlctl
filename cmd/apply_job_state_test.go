package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/shapedthought/owlctl/models"
)

type mockJobStateClient struct {
	disabled   bool // state reported by IsDisabled (VBR ignores isDisabled on create)
	setErr     error
	readErr    error
	setCalls   []bool
	readCalled int
}

func (m *mockJobStateClient) IsDisabled(id string) (bool, error) {
	m.readCalled++
	return m.disabled, m.readErr
}

func (m *mockJobStateClient) SetDisabled(id string, disabled bool) error {
	m.setCalls = append(m.setCalls, disabled)
	return m.setErr
}

func withMockJobState(t *testing.T, m *mockJobStateClient) {
	t.Helper()
	orig := newJobStateClient
	newJobStateClient = func(models.Profile) jobStateClient { return m }
	t.Cleanup(func() { newJobStateClient = orig })
}

func TestApplyJobState_CreateDisabledCallsDisable(t *testing.T) {
	// A freshly created job is enabled even though the spec says isDisabled: true.
	m := &mockJobStateClient{disabled: false}
	withMockJobState(t, m)

	_, err := applyJobState(map[string]interface{}{"isDisabled": true}, "job-1", models.Profile{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.setCalls) != 1 || !m.setCalls[0] {
		t.Fatalf("expected one disable call, got %v", m.setCalls)
	}
}

func TestApplyJobState_UpdateEnableCallsEnable(t *testing.T) {
	m := &mockJobStateClient{disabled: true}
	withMockJobState(t, m)

	if _, err := applyJobState(map[string]interface{}{"isDisabled": false}, "job-1", models.Profile{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.setCalls) != 1 || m.setCalls[0] {
		t.Fatalf("expected one enable call, got %v", m.setCalls)
	}
}

func TestApplyJobState_RefusalReturnsErrorWithVBRMessage(t *testing.T) {
	vbrMsg := `HTTP 400: POST https://vbr:9419/api/v1/jobs/abc/disable
Response: {"errorCode":"UnknownError","message":"Could not disable job [x]. JobType: [Backup], RunAutomatically: [False]","status":400}`
	m := &mockJobStateClient{disabled: false, setErr: errors.New(vbrMsg)}
	withMockJobState(t, m)

	out, err := applyJobState(map[string]interface{}{"isDisabled": true}, "job-1", models.Profile{})
	if err == nil {
		t.Fatal("expected an error when VBR refuses to disable the job")
	}
	if !strings.Contains(err.Error(), "Could not disable job") || !strings.Contains(err.Error(), "was not applied") {
		t.Fatalf("error should carry VBR's message and say isDisabled was not applied, got: %v", err)
	}
	if _, ok := out.Applied["isDisabled"]; ok || out.Actual["isDisabled"] != false {
		t.Fatalf("refusal should report the actual value and no applied field, got %+v", out)
	}
}

func TestApplyJobState_NoCallWhenStateMatches(t *testing.T) {
	for _, tc := range []bool{true, false} {
		m := &mockJobStateClient{disabled: tc}
		withMockJobState(t, m)

		if _, err := applyJobState(map[string]interface{}{"isDisabled": tc}, "job-1", models.Profile{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(m.setCalls) != 0 {
			t.Fatalf("isDisabled=%v: expected no enable/disable call, got %v", tc, m.setCalls)
		}
	}
}

func TestApplyJobState_NoCallWhenSpecOmitsIsDisabled(t *testing.T) {
	m := &mockJobStateClient{}
	withMockJobState(t, m)

	if _, err := applyJobState(map[string]interface{}{"name": "x"}, "job-1", models.Profile{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.readCalled != 0 || len(m.setCalls) != 0 {
		t.Fatalf("expected no VBR calls, got reads=%d sets=%v", m.readCalled, m.setCalls)
	}
}

func TestApplyJobState_ReadFailureIsError(t *testing.T) {
	m := &mockJobStateClient{readErr: errors.New("boom")}
	withMockJobState(t, m)

	if _, err := applyJobState(map[string]interface{}{"isDisabled": true}, "job-1", models.Profile{}); err == nil {
		t.Fatal("expected an error when job state cannot be read")
	}
	if len(m.setCalls) != 0 {
		t.Fatalf("expected no set call, got %v", m.setCalls)
	}
}

func TestPlannedJobState(t *testing.T) {
	tests := []struct {
		name     string
		spec     map[string]interface{}
		existing map[string]interface{}
		want     string // substring; "" means no plan
	}{
		{"create disabled", map[string]interface{}{"isDisabled": true}, nil, "/disable"},
		{"create enabled", map[string]interface{}{"isDisabled": false}, nil, ""},
		{"update to enabled", map[string]interface{}{"isDisabled": false}, map[string]interface{}{"isDisabled": true}, "/enable"},
		{"update unchanged", map[string]interface{}{"isDisabled": true}, map[string]interface{}{"isDisabled": true}, ""},
		{"not in spec", map[string]interface{}{"name": "x"}, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := plannedJobState(tt.spec, tt.existing, "job-123")
			if tt.want == "" && got != "" {
				t.Fatalf("expected no plan, got %q", got)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Fatalf("expected plan containing %q, got %q", tt.want, got)
			}
		})
	}
}
