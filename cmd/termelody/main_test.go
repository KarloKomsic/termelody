package main

import (
	"path/filepath"
	"testing"
)

func TestScanDir(t *testing.T) {
	abs := func(p string) string {
		got, err := filepath.Abs(p)
		if err != nil {
			t.Fatalf("resolving %q: %v", p, err)
		}
		return got
	}

	cases := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{"no argument falls back to the default", nil, abs(defaultMusicDir), false},
		{"an absolute directory is kept", []string{"/music/library"}, "/music/library", false},
		{"a relative directory is resolved", []string{"library"}, abs("library"), false},
		{"a second argument is rejected", []string{"first", "second"}, "", true},
	}

	for _, c := range cases {
		got, err := scanDir(c.args)

		if c.wantErr {
			if err == nil {
				t.Errorf("%s: expected an error, got %q", c.name, got)
			}
			continue
		}

		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
