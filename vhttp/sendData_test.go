package vhttp

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/shapedthought/owlctl/utils"
)

func TestSendData_BOMPrefixedJSONFileSendsValidJSON(t *testing.T) {
	var gotBody []byte
	profile := setupGetDataTest(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})

	path := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(path, []byte("\xEF\xBB\xBF{\"name\":\"x\"}"), 0o600); err != nil {
		t.Fatal(err)
	}

	SendData(os.Getenv("OWLCTL_URL"), path, "jobs", "POST", profile, utils.ReadSettings())

	var m map[string]string
	if err := json.Unmarshal(gotBody, &m); err != nil {
		t.Fatalf("body %q is not valid JSON: %v", gotBody, err)
	}
	if m["name"] != "x" {
		t.Errorf("name = %q, want x", m["name"])
	}
}
