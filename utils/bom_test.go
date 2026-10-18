package utils

import "testing"

func TestStripUTF8BOM(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"with BOM", []byte("\xEF\xBB\xBF<xml/>"), "<xml/>"},
		{"without BOM", []byte("<xml/>"), "<xml/>"},
		{"empty", []byte{}, ""},
		{"nil", nil, ""},
		{"BOM only", []byte("\xEF\xBB\xBF"), ""},
		{"partial BOM", []byte("\xEF\xBB<"), "\xEF\xBB<"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(StripUTF8BOM(tt.in)); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
