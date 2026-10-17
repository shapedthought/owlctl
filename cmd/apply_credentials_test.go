package cmd

import (
	"os"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func loadCredsFixture(t *testing.T, name string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var spec map[string]interface{}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	return spec
}

func credsGC(spec map[string]interface{}) map[string]interface{} {
	return spec["guestProcessing"].(map[string]interface{})["guestCredentials"].(map[string]interface{})
}

func existingWithCreds(list []interface{}) map[string]interface{} {
	return map[string]interface{}{
		"guestProcessing": map[string]interface{}{
			"guestCredentials": map[string]interface{}{
				"credentialsPerMachine": list,
			},
		},
	}
}

func TestPrepareJobPayload_NullCredentialsPerMachine_CreateOmitsKey(t *testing.T) {
	spec := loadCredsFixture(t, "job-credentials-per-machine-null.yaml")
	if l := credsGC(spec)["credentialsPerMachine"].([]interface{}); len(l) != 1 || l[0] != nil {
		t.Fatalf("fixture should parse to [nil], got %v", l)
	}
	out, err := prepareJobPayload(spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := credsGC(out)["credentialsPerMachine"]; ok {
		t.Errorf("credentialsPerMachine should be omitted, got %v", credsGC(out)["credentialsPerMachine"])
	}
}

func TestPrepareJobPayload_NullCredentialsPerMachine_UpdateBackfills(t *testing.T) {
	live := map[string]interface{}{"id": "cred-1"}
	out, _ := prepareJobPayload(loadCredsFixture(t, "job-credentials-per-machine-null.yaml"), existingWithCreds([]interface{}{live}))
	want := []interface{}{live}
	if got := credsGC(out)["credentialsPerMachine"]; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPrepareJobPayload_NullCredentialsPerMachine_EmptyExistingOmits(t *testing.T) {
	out, _ := prepareJobPayload(loadCredsFixture(t, "job-credentials-per-machine-null.yaml"), existingWithCreds([]interface{}{}))
	if _, ok := credsGC(out)["credentialsPerMachine"]; ok {
		t.Error("key should be omitted, never []")
	}
}

func TestPrepareJobPayload_MixedCredentialsPerMachineKeepsValid(t *testing.T) {
	entry := map[string]interface{}{"hostName": "vm1", "credentials": map[string]interface{}{"id": "c"}}
	spec := loadCredsFixture(t, "job-credentials-per-machine-mixed.yaml")
	out, _ := prepareJobPayload(spec, map[string]interface{}{})
	want := []interface{}{entry}
	if got := credsGC(out)["credentialsPerMachine"]; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
