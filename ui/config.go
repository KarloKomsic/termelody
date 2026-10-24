package ui

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
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
