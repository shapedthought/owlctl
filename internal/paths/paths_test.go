package paths

import (
	"path/filepath"
	"testing"
)

func TestPaths_UnsetUsesCWD(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("OWLCTL_SETTINGS_PATH", "")

	if got, want := File("state.json"), "state.json"; got != want {
		t.Errorf("File = %q, want %q", got, want)
	}
	if got := ConfigDir(); got != "." {
		t.Errorf("ConfigDir = %q, want .", got)
	}
}

func TestPaths_EnvSet_TrailingSlashVariants(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "state.json")
	for _, v := range []string{dir, dir + string(filepath.Separator)} {
		t.Setenv("OWLCTL_SETTINGS_PATH", v)
		if got := File("state.json"); got != want {
			t.Errorf("OWLCTL_SETTINGS_PATH=%q: File = %q, want %q", v, got, want)
		}
	}
}
