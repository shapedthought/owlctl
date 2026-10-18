package vhttp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func runWrite(t *testing.T, status int, contentType, body string) (string, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	defer srv.Close()

	res, err := http.Post(srv.URL, "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	var stdout, stderr bytes.Buffer
	if err := writeSuccessResponse(res, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	return stdout.String(), stderr.String()
}

func TestWriteSuccessResponseJSON(t *testing.T) {
	out, errOut := runWrite(t, 201, "application/json", `{"id":"abc","n":1}`)
	want := "{\n  \"id\": \"abc\",\n  \"n\": 1\n}\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if !strings.Contains(errOut, "Status Code: 201") {
		t.Errorf("stderr = %q, want status line", errOut)
	}
}

func TestWriteSuccessResponseEmpty204(t *testing.T) {
	out, errOut := runWrite(t, 204, "application/json", "")
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "Status Code: 204") {
		t.Errorf("stderr = %q, want status line", errOut)
	}
}

func TestWriteSuccessResponseRaw(t *testing.T) {
	out, _ := runWrite(t, 200, "text/plain", "not json")
	if out != "not json\n" {
		t.Errorf("stdout = %q, want %q", out, "not json\n")
	}
}

func TestWriteSuccessResponseStdoutOnlyBody(t *testing.T) {
	out, errOut := runWrite(t, 200, "application/json", `{"a":1}`)
	if strings.Contains(out, "Status") {
		t.Errorf("stdout contains status text: %q", out)
	}
	if !strings.Contains(errOut, "Status Code: 200") {
		t.Errorf("stderr = %q, want status line", errOut)
	}
}
