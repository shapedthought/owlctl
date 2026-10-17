package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/shapedthought/owlctl/models"
	"github.com/shapedthought/owlctl/vhttp"
)

// jobStateClient is the VBR surface needed to reconcile a job's enabled/disabled
// state. VBR ignores isDisabled on POST/PUT of a job, so it has to be applied
// through the dedicated enable/disable endpoints. It is an interface so tests
// can substitute a mock.
type jobStateClient interface {
	// IsDisabled returns the job's current isDisabled value.
	IsDisabled(id string) (bool, error)
	// SetDisabled calls POST jobs/{id}/disable or jobs/{id}/enable.
	SetDisabled(id string, disabled bool) error
}

type vbrJobStateClient struct {
	profile models.Profile
}

func (c vbrJobStateClient) IsDisabled(id string) (bool, error) {
	raw, err := vhttp.GetDataWithError[json.RawMessage](fmt.Sprintf("jobs/%s", id), c.profile)
	if err != nil {
		return false, err
	}
	var job map[string]interface{}
	if err := json.Unmarshal(raw, &job); err != nil {
		return false, fmt.Errorf("failed to parse job %s: %w", id, err)
	}
	disabled, _ := job["isDisabled"].(bool)
	return disabled, nil
}

func (c vbrJobStateClient) SetDisabled(id string, disabled bool) error {
	_, err := vhttp.PostDataWithError(fmt.Sprintf("jobs/%s/%s", id, jobStateVerb(disabled)), nil, c.profile)
	return err
}

// newJobStateClient builds the client used by applyJobState. Replaced in tests.
var newJobStateClient = func(profile models.Profile) jobStateClient {
	return vbrJobStateClient{profile: profile}
}

func jobStateVerb(disabled bool) string {
	if disabled {
		return "disable"
	}
	return "enable"
}

// desiredJobDisabled returns the spec's isDisabled value, and whether the spec sets it.
func desiredJobDisabled(spec map[string]interface{}) (bool, bool) {
	v, ok := spec["isDisabled"].(bool)
	return v, ok
}

// applyJobState reconciles the job's enabled/disabled state with the spec's
// isDisabled after a create or update. It reads the job's actual state back from
// VBR and only calls enable/disable when it differs. A refusal from VBR (for
// example HTTP 400 for jobs that are not scheduled) is returned as an error that
// includes VBR's message.
func applyJobState(spec map[string]interface{}, id string, profile models.Profile) error {
	want, ok := desiredJobDisabled(spec)
	if !ok {
		return nil
	}
	client := newJobStateClient(profile)
	actual, err := client.IsDisabled(id)
	if err != nil {
		return fmt.Errorf("isDisabled was not applied: could not read job state: %w", err)
	}
	if actual == want {
		return nil
	}
	fmt.Printf("Setting job state: %s (isDisabled: %t)\n", jobStateVerb(want), want)
	if err := client.SetDisabled(id, want); err != nil {
		return fmt.Errorf("isDisabled: %t was not applied, VBR refused to %s the job: %w", want, jobStateVerb(want), err)
	}
	return nil
}

// plannedJobState describes the enable/disable call apply would make, for
// --dry-run. existing is nil when the job does not exist yet (VBR creates jobs
// enabled). Returns "" when no call is needed.
func plannedJobState(spec, existing map[string]interface{}) string {
	want, ok := desiredJobDisabled(spec)
	if !ok {
		return ""
	}
	actual := false
	if existing != nil {
		actual, _ = existing["isDisabled"].(bool)
	}
	if actual == want {
		return ""
	}
	return fmt.Sprintf("POST jobs/{id}/%s (isDisabled: %t -> %t)", jobStateVerb(want), actual, want)
}
