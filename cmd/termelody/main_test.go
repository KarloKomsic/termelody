package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codeberg.org/karlokomsic/termelody/ui"
)

// writeConfig puts a config file in a temporary directory and returns its path,
// so parseArgs tests never read the real user config.
func writeConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return path
}

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
		wantIcons  ui.IconSet
		wantErr    bool
		wantHelp   bool
	}{
		{"no arguments leaves the directory to the picker", nil, "", ui.DefaultSeekStep, false, ui.IconsUnicode, false, false},
		{"a directory alone keeps the default seek", []string{"library"}, mustAbs(t, "library"), ui.DefaultSeekStep, false, ui.IconsUnicode, false, false},
		{"the seek flag precedes the directory", []string{"-seek", "10s", "library"}, mustAbs(t, "library"), 10 * time.Second, false, ui.IconsUnicode, false, false},
		{"the seek flag works without a directory", []string{"-seek", "30s"}, "", 30 * time.Second, false, ui.IconsUnicode, false, false},
		{"the hidden flag enables dotfolders", []string{"-hidden"}, "", ui.DefaultSeekStep, true, ui.IconsUnicode, false, false},
		{"the icons flag selects nerd", []string{"-icons", "nerd"}, "", ui.DefaultSeekStep, false, ui.IconsNerd, false, false},
		{"the icons flag selects none", []string{"-icons", "none"}, "", ui.DefaultSeekStep, false, ui.IconsNone, false, false},
		{"an unknown icon set is rejected", []string{"-icons", "fancy"}, "", 0, false, "", true, false},
		{"a malformed duration is rejected", []string{"-seek", "soon"}, "", 0, false, "", true, false},
		{"a non-positive duration is rejected", []string{"-seek", "0s"}, "", 0, false, "", true, false},
		{"two directories are rejected", []string{"first", "second"}, "", 0, false, "", true, false},
		{"help requests usage without an error message", []string{"-h"}, "", 0, false, "", true, true},
	}

	for _, c := range cases {
		gotDir, gotCfg, err := parseArgs(c.args, "")

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
		if gotCfg.Icons != c.wantIcons {
			t.Errorf("%s: Icons = %q, want %q", c.name, gotCfg.Icons, c.wantIcons)
		}
		if gotCfg.VolumeStep != ui.DefaultVolumeStep {
			t.Errorf("%s: VolumeStep = %d, want %d", c.name, gotCfg.VolumeStep, ui.DefaultVolumeStep)
		}
	}
}

func TestParseArgsReadsConfig(t *testing.T) {
	path := writeConfig(t, `seek = 10s
icons = nerd
show_hidden = true
volume_step = 4
`)

	dir, cfg, err := parseArgs(nil, path)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if dir != "" {
		t.Errorf("dir = %q, want empty so the picker chooses", dir)
	}

	want := ui.Config{
		SeekStep:   10 * time.Second,
		ShowHidden: true,
		Icons:      ui.IconsNerd,
		VolumeStep: 4,
	}
	if cfg != want {
		t.Errorf("Config = %+v, want %+v", cfg, want)
	}
}

func TestParseArgsFlagsOverrideConfig(t *testing.T) {
	path := writeConfig(t, `seek = 10s
icons = nerd
show_hidden = true
volume_step = 4
`)

	_, cfg, err := parseArgs(
		[]string{"-seek", "20s", "-icons", "none", "-hidden=false"},
		path,
	)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}

	// volume_step has no flag, so the config value must survive; the three
	// flags must beat the file values.
	want := ui.Config{
		SeekStep:   20 * time.Second,
		ShowHidden: false,
		Icons:      ui.IconsNone,
		VolumeStep: 4,
	}
	if cfg != want {
		t.Errorf("Config = %+v, want %+v", cfg, want)
	}
}

func TestParseArgsReportsConfigError(t *testing.T) {
	path := writeConfig(t, "seek = soon\n")

	_, _, err := parseArgs(nil, path)
	if err == nil {
		t.Fatal("parseArgs accepted a malformed config")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the config file %s", err, path)
	}
}
