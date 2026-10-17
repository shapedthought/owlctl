package labverification

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func yamlFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.yml")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := filepath.Glob("templates/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, tpl...)
	if len(files) < 2 {
		t.Fatalf("expected pipeline and login template, found %v", files)
	}
	return files
}

func TestYAMLParses(t *testing.T) {
	for _, f := range yamlFiles(t) {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]interface{}
		if err := yaml.Unmarshal(data, &v); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

func TestPipelineShape(t *testing.T) {
	data, err := os.ReadFile("lab-verification.yml")
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Trigger    interface{}              `yaml:"trigger"`
		PR         interface{}              `yaml:"pr"`
		Schedules  interface{}              `yaml:"schedules"`
		Parameters []map[string]interface{} `yaml:"parameters"`
	}
	if err := yaml.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	if p.Trigger != "none" || p.PR != "none" {
		t.Errorf("trigger/pr must be none, got %v / %v", p.Trigger, p.PR)
	}
	if p.Schedules != nil {
		t.Error("schedules must not be defined")
	}
	for _, prm := range p.Parameters {
		if prm["name"] == "runApplyCycle" || prm["name"] == "runBackup" {
			if prm["default"] != false {
				t.Errorf("%v must default to false", prm["name"])
			}
		}
	}
}

func TestTemplatePathsResolve(t *testing.T) {
	re := regexp.MustCompile(`template:\s*(\S+)`)
	for _, f := range yamlFiles(t) {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(data), -1) {
			target := filepath.Join(filepath.Dir(f), m[1])
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s: template %s does not resolve: %v", f, m[1], err)
			}
		}
	}
}

func TestPasswordScanAndPublishSafety(t *testing.T) {
	data, err := os.ReadFile("lab-verification.yml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !regexp.MustCompile(`grep -r[a-zA-Z]*[ql][a-zA-Z]*E '\^\\s\*password`).MatchString(text) {
		t.Error("password scan must use -q or -l so values are never printed")
	}
	if strings.Contains(text, "succeededOrFailed()") {
		t.Error("exports must not be published when the scan fails")
	}
}

func TestNoIPsOrSecrets(t *testing.T) {
	ip := regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	secret := regexp.MustCompile(`(?i)(password|token|secret)\s*[:=]\s*["']?[A-Za-z0-9+/_\-]{8,}`)
	files := append(yamlFiles(t), "README.md")
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if ip.MatchString(line) {
				t.Errorf("%s:%d contains an IP address", f, i+1)
			}
			if secret.MatchString(line) && !strings.Contains(line, "$(") && !strings.Contains(line, "$OWLCTL") {
				t.Errorf("%s:%d looks like a secret: %s", f, i+1, line)
			}
		}
	}
}
