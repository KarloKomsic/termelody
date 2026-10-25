package ui

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// LoadConfig reads preferences from path. An empty path and a missing file both
// mean "use defaults", because the config file is optional. Errors carry the
// file and line so a hand-edited mistake is easy to find.
func LoadConfig(path string) (Config, error) {
	cfg := Config{}
	if path == "" {
		return cfg.withDefaults(), nil
	}

	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg.withDefaults(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	defer file.Close()

	seen := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())

		// A comment can be a whole line or trail a value. None of the current
		// values can contain #, so cutting at the first one keeps the format
		// familiar without needing quoting rules.
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !ok || key == "" {
			return Config{}, fmt.Errorf("%s:%d: expected key = value", path, lineNo)
		}
		if seen[key] {
			return Config{}, fmt.Errorf("%s:%d: duplicate setting %q", path, lineNo, key)
		}
		seen[key] = true

		switch key {
		case "seek":
			step, err := time.ParseDuration(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: seek: %w", path, lineNo, err)
			}
			// withDefaults would quietly replace a non-positive value with the
			// default, which hides the mistake; report it instead.
			if step <= 0 {
				return Config{}, fmt.Errorf("%s:%d: seek must be positive, got %s", path, lineNo, step)
			}
			cfg.SeekStep = step

		case "icons":
			set, err := ParseIconSet(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: %w", path, lineNo, err)
			}
			cfg.Icons = set

		case "show_hidden":
			switch value {
			case "true":
				cfg.ShowHidden = true
			case "false":
				cfg.ShowHidden = false
			default:
				return Config{}, fmt.Errorf("%s:%d: show_hidden must be true or false, got %q", path, lineNo, value)
			}

		case "volume_step":
			step, err := strconv.Atoi(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s:%d: volume_step: %w", path, lineNo, err)
			}
			if step <= 0 {
				return Config{}, fmt.Errorf("%s:%d: volume_step must be positive, got %d", path, lineNo, step)
			}
			cfg.VolumeStep = step

		default:
			return Config{}, fmt.Errorf("%s:%d: unknown setting %q", path, lineNo, key)
		}
	}
	if err := scanner.Err(); err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}

	return cfg.withDefaults(), nil
}

// SaveConfig writes cfg to path, creating the directory when needed. Existing
// lines are kept: known keys are updated in place so comments and ordering
// survive, missing keys are appended, and unknown lines are left alone for a
// newer version or a human note. The write goes through a temporary file and a
// rename so an interrupted save cannot truncate the settings.
func SaveConfig(path string, cfg Config) error {
	if path == "" {
		return errors.New("no config path")
	}
	cfg = cfg.withDefaults()

	values := map[string]string{
		"seek":        cfg.SeekStep.String(),
		"icons":       string(cfg.Icons),
		"show_hidden": strconv.FormatBool(cfg.ShowHidden),
		"volume_step": strconv.Itoa(cfg.VolumeStep),
	}
	order := []string{"seek", "icons", "show_hidden", "volume_step"}

	lines, err := readConfigLines(path)
	if errors.Is(err, fs.ErrNotExist) {
		lines = []string{
			"# Termelody settings.",
			"# Command-line flags override values from this file.",
		}
	} else if err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}

	written := make(map[string]bool, len(order))
	for i, line := range lines {
		body, comment := splitConfigComment(line)
		key, _, ok := strings.Cut(body, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if _, known := values[key]; !known {
			continue
		}

		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		updated := indent + key + " = " + values[key]
		if comment != "" {
			updated += " " + comment
		}
		lines[i] = updated
		written[key] = true
	}

	for _, key := range order {
		if !written[key] {
			lines = append(lines, key+" = "+values[key])
		}
	}

	return writeConfigFile(path, strings.Join(lines, "\n")+"\n")
}

// readConfigLines keeps the file's line structure so SaveConfig can update only
// the values it owns. One trailing newline is dropped so joining and appending
// does not accumulate blank lines at the end.
func readConfigLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

// splitConfigComment separates a line from a trailing # comment. The comment
// keeps its leading # so callers can put it back unchanged.
func splitConfigComment(line string) (body, comment string) {
	if i := strings.IndexByte(line, '#'); i >= 0 {
		return line[:i], strings.TrimSpace(line[i:])
	}
	return line, ""
}

// writeConfigFile replaces path atomically. Keeping the temporary file in the
// same directory means the rename stays on one filesystem, and the deferred
// remove is a no-op after a successful rename.
func writeConfigFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}

	tmp, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("config %s: %w", path, err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("config %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}

	return nil
}
