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

func TestSaveConfigCreatesFileAndDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "termelody", "config")
	want := Config{
		SeekStep:   10 * time.Second,
		ShowHidden: true,
		Icons:      IconsNerd,
		VolumeStep: 4,
	}

	if err := SaveConfig(path, want); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	got, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig after save: %v", err)
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("config mode = %o, want 600", mode)
	}

	dir := filepath.Dir(path)
	info, err = os.Stat(dir)
	if err != nil {
		t.Fatalf("stat config directory: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Errorf("config directory mode = %o, want 700", mode)
	}

	// The temporary file used for the atomic rename must not survive.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read config directory: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "config" {
		t.Errorf("config directory holds %v, want only config", entries)
	}
}

func TestSaveConfigPreservesCommentsOrderAndUnknownLines(t *testing.T) {
	path := writeConfig(t, `# top comment
volume_step = 2  # press size

# icons below
icons = none
future_setting = keep me
seek = 1s
`)

	cfg := Config{
		SeekStep:   15 * time.Second,
		ShowHidden: true,
		Icons:      IconsNerd,
		VolumeStep: 6,
	}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	want := `# top comment
volume_step = 6 # press size

# icons below
icons = nerd
future_setting = keep me
seek = 15s
show_hidden = true
`
	if string(data) != want {
		t.Errorf("config =\n%s\nwant\n%s", data, want)
	}
}

func TestSaveConfigAppendsMissingKeys(t *testing.T) {
	path := writeConfig(t, "# keep\nseek = 2s\n")

	cfg := Config{SeekStep: 2 * time.Second, ShowHidden: false, Icons: IconsUnicode, VolumeStep: 1}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	want := "# keep\nseek = 2s\nicons = unicode\nshow_hidden = false\nvolume_step = 1\n"
	if string(data) != want {
		t.Errorf("config =\n%s\nwant\n%s", data, want)
	}
}

func TestSaveConfigFillsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")

	if err := SaveConfig(path, Config{}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	got, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if want := defaultConfig(); got != want {
		t.Errorf("LoadConfig after saving zero Config = %+v, want %+v", got, want)
	}
}

func TestSaveConfigRequiresAPath(t *testing.T) {
	if err := SaveConfig("", Config{}); err == nil {
		t.Error("SaveConfig with an empty path succeeded, want an error")
	}
}

func TestSaveConfigReportsReadFailure(t *testing.T) {
	// A directory cannot be read as a config file, so the save must refuse
	// rather than replace whatever it does not understand.
	dir := t.TempDir()
	if err := SaveConfig(dir, Config{}); err == nil {
		t.Error("SaveConfig over a directory succeeded, want an error")
	}
}

func TestSaveConfigReportsWriteFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	// The parent of the config path is a regular file, so creating the
	// directory has to fail.
	path := filepath.Join(blocker, "config")
	err := SaveConfig(path, Config{})
	if err == nil {
		t.Fatal("SaveConfig under a regular file succeeded, want an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the config file %s", err, path)
	}
}
