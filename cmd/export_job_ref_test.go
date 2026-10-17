package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const (
	idA = "57b3baab-6237-41bf-add7-db63d41d984c"
	idB = "11111111-2222-3333-4444-555555555555"
	idC = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
)

// fakeJobs returns getJob/listJobs closures over a mocked job list and counts calls.
func fakeJobs(jobs []jobListItem) (func(string) (json.RawMessage, error), func() ([]jobListItem, error), *int) {
	lists := 0
	get := func(id string) (json.RawMessage, error) {
		for _, j := range jobs {
			if j.ID == id {
				return json.RawMessage(fmt.Sprintf(`{"id":%q,"name":%q}`, j.ID, j.Name)), nil
			}
		}
		return nil, fmt.Errorf("HTTP 404")
	}
	list := func() ([]jobListItem, error) {
		lists++
		return jobs, nil
	}
	return get, list, &lists
}

func resolvedID(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var m struct{ ID string }
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m.ID
}

func TestResolveJobRef_ByID(t *testing.T) {
	get, list, lists := fakeJobs([]jobListItem{{idA, "Job A"}, {idB, "Job B"}})
	raw, err := resolveJobRef(idA, get, list)
	if err != nil || resolvedID(t, raw) != idA {
		t.Fatalf("got %s, %v", raw, err)
	}
	if *lists != 0 {
		t.Errorf("id lookup should not list jobs")
	}
}

func TestResolveJobRef_ByExactName(t *testing.T) {
	get, list, _ := fakeJobs([]jobListItem{{idA, "Job A"}, {idB, "Job B"}})
	raw, err := resolveJobRef("Job B", get, list)
	if err != nil || resolvedID(t, raw) != idB {
		t.Fatalf("got %s, %v", raw, err)
	}
}

func TestResolveJobRef_NameIsCaseSensitive(t *testing.T) {
	get, list, _ := fakeJobs([]jobListItem{{idA, "Job A"}})
	if _, err := resolveJobRef("job a", get, list); err == nil {
		t.Fatal("expected not found for different case")
	}
}

func TestResolveJobRef_NotFound(t *testing.T) {
	get, list, _ := fakeJobs([]jobListItem{{idA, "Job A"}})
	_, err := resolveJobRef("Nope", get, list)
	if err == nil || err.Error() != `job "Nope" not found` {
		t.Fatalf("got %v", err)
	}
}

func TestResolveJobRef_Ambiguous(t *testing.T) {
	get, list, _ := fakeJobs([]jobListItem{{idA, "Dup"}, {idB, "Dup"}, {idC, "Other"}})
	_, err := resolveJobRef("Dup", get, list)
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	for _, want := range []string{idA, idB, "id"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
	if strings.Contains(err.Error(), idC) {
		t.Errorf("error should not list non-matching ids")
	}
}

func TestResolveJobRef_UUIDShapedName(t *testing.T) {
	// A job whose name looks like a UUID but matches no job id.
	get, list, lists := fakeJobs([]jobListItem{{idA, idC}})
	raw, err := resolveJobRef(idC, get, list)
	if err != nil || resolvedID(t, raw) != idA {
		t.Fatalf("got %s, %v", raw, err)
	}
	if *lists != 1 {
		t.Errorf("expected fallback to list jobs once, got %d", *lists)
	}
}

func TestResolveJobRef_UUIDMatchingNothing(t *testing.T) {
	get, list, _ := fakeJobs([]jobListItem{{idA, "Job A"}})
	_, err := resolveJobRef(idC, get, list)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveJobRef_IDWinsOverName(t *testing.T) {
	// Job B is named after job A's id; the id match takes precedence.
	get, list, _ := fakeJobs([]jobListItem{{idA, "Job A"}, {idB, idA}})
	raw, err := resolveJobRef(idA, get, list)
	if err != nil || resolvedID(t, raw) != idA {
		t.Fatalf("got %s, %v", raw, err)
	}
}

func TestJobExportFlagsUnchanged(t *testing.T) {
	for _, name := range []string{"all", "directory", "output", "as-overlay", "base", "simplified"} {
		if jobExportCmd.Flags().Lookup(name) == nil {
			t.Errorf("job export lost flag --%s", name)
		}
	}
	if jobExportCmd.Use != "export [job-name|job-id]" || exportCmd.Use != jobExportCmd.Use {
		t.Errorf("unexpected Use: %q / %q", jobExportCmd.Use, exportCmd.Use)
	}
}
