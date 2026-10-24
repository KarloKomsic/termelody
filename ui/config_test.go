package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return path
}

func defaultConfig() Config {
	return Config{}.withDefaults()
}

func TestLoadConfigMissingFileUsesDefaults(t *testing.T) {
	got, err := LoadConfig(filepath.Join(t.TempDir(), "missing", "config"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got != defaultConfig() {
		t.Errorf("LoadConfig = %+v, want defaults %+v", got, defaultConfig())
	}
}

func TestLoadConfigEmptyPathUsesDefaults(t *testing.T) {
	got, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got != defaultConfig() {
		t.Errorf("LoadConfig = %+v, want defaults %+v", got, defaultConfig())
	}
}

func TestLoadConfigReadsEverySetting(t *testing.T) {
	path := writeConfig(t, `# Termelody settings
seek = 10s
icons = nerd
show_hidden = true
volume_step = 3
`)

	got, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	want := Config{
		SeekStep:   10 * time.Second,
		ShowHidden: true,
		Icons:      IconsNerd,
		VolumeStep: 3,
	}
	if got != want {
		t.Errorf("LoadConfig = %+v, want %+v", got, want)
	}
}

func TestLoadConfigIgnoresCommentsBlankLinesAndSpacing(t *testing.T) {
	path := writeConfig(t, `
# a whole-line comment
  seek = 1m30s  # a trailing comment

icons=nerd
show_hidden=false
volume_step = 2
`)

	got, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	want := Config{
		SeekStep:   90 * time.Second,
		Icons:      IconsNerd,
		VolumeStep: 2,
	}
	if got != want {
		t.Errorf("LoadConfig = %+v, want %+v", got, want)
	}
}

func TestLoadConfigDefaultsUnsetSettings(t *testing.T) {
	path := writeConfig(t, "seek = 10s\n")

	got, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	want := defaultConfig()
	want.SeekStep = 10 * time.Second
	if got != want {
		t.Errorf("LoadConfig = %+v, want %+v", got, want)
	}
}

func TestLoadConfigRejectsBadInput(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantLine int
	}{
		{"malformed duration", "seek = soon\n", 1},
		{"non-positive seek", "seek = 0s\n", 1},
		{"negative seek", "seek = -5s\n", 1},
		{"unknown icon set", "icons = fancy\n", 1},
		{"non-boolean hidden", "show_hidden = yes\n", 1},
		{"zero volume step", "volume_step = 0\n", 1},
		{"negative volume step", "volume_step = -2\n", 1},
		{"non-integer volume step", "volume_step = 2.5\n", 1},
		{"unknown setting", "colour = blue\n", 1},
		{"missing equals", "seek 5s\n", 1},
		{"empty key", "= 5s\n", 1},
		{"duplicate setting", "seek = 5s\nseek = 10s\n", 2},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writeConfig(t, c.body)

			_, err := LoadConfig(path)
			if err == nil {
				t.Fatalf("LoadConfig(%q) succeeded, want an error", c.body)
			}
			if want := fmt.Sprintf("%s:%d", path, c.wantLine); !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name %s", err, want)
			}
		})
	}
}

func TestLoadConfigReportsReadError(t *testing.T) {
	// Opening a directory succeeds on Unix, but reading it does not, which is
	// enough to exercise the scanner error path.
	dir := t.TempDir()
	if _, err := LoadConfig(dir); err == nil {
		t.Error("LoadConfig on a directory succeeded, want an error")
	}
}
