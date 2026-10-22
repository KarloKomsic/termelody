package main

import (
	"errors"
	"flag"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/karlokomsic/termelody/ui"
)

// mustAbs resolves p or fails the test, so table cases can compare against
// the same absolute paths scanDir produces.
func mustAbs(t *testing.T, p string) string {
	t.Helper()

	got, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("resolving %q: %v", p, err)
	}
	return got
}

func TestScanDir(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{"no argument leaves the choice to the picker", nil, "", false},
		{"an absolute directory is kept", []string{"/music/library"}, "/music/library", false},
		{"a relative directory is resolved", []string{"library"}, mustAbs(t, "library"), false},
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

func TestParseArgs(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantDir    string
		wantSeek   time.Duration
		wantHidden bool
		wantErr    bool
		wantHelp   bool
	}{
		{"no arguments leaves the directory to the picker", nil, "", ui.DefaultSeekStep, false, false, false},
		{"a directory alone keeps the default seek", []string{"library"}, mustAbs(t, "library"), ui.DefaultSeekStep, false, false, false},
		{"the seek flag precedes the directory", []string{"-seek", "10s", "library"}, mustAbs(t, "library"), 10 * time.Second, false, false, false},
		{"the seek flag works without a directory", []string{"-seek", "30s"}, "", 30 * time.Second, false, false, false},
		{"the hidden flag enables dotfolders", []string{"-hidden"}, "", ui.DefaultSeekStep, true, false, false},
		{"a malformed duration is rejected", []string{"-seek", "soon"}, "", 0, false, true, false},
		{"a non-positive duration is rejected", []string{"-seek", "0s"}, "", 0, false, true, false},
		{"two directories are rejected", []string{"first", "second"}, "", 0, false, true, false},
		{"help requests usage without an error message", []string{"-h"}, "", 0, false, true, true},
	}

	for _, c := range cases {
		gotDir, gotCfg, err := parseArgs(c.args)

		if c.wantErr {
			if err == nil {
				t.Errorf("%s: expected an error, got dir %q", c.name, gotDir)
				continue
			}
			if c.wantHelp && !errors.Is(err, flag.ErrHelp) {
				t.Errorf("%s: got %v, want flag.ErrHelp", c.name, err)
			}
			continue
		}

		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if gotDir != c.wantDir {
			t.Errorf("%s: dir = %q, want %q", c.name, gotDir, c.wantDir)
		}
		if gotCfg.SeekStep != c.wantSeek {
			t.Errorf("%s: SeekStep = %s, want %s", c.name, gotCfg.SeekStep, c.wantSeek)
		}
		if gotCfg.ShowHidden != c.wantHidden {
			t.Errorf("%s: ShowHidden = %t, want %t", c.name, gotCfg.ShowHidden, c.wantHidden)
		}
	}
}
