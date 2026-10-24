package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/karlokomsic/termelody/loader"
	"codeberg.org/karlokomsic/termelody/player"
	"codeberg.org/karlokomsic/termelody/playlist"
	"codeberg.org/karlokomsic/termelody/ui"
)

func main() {
	configPath, err := userConfigPath()
	if err != nil {
		// The config file is optional, so an unavailable config directory
		// must not stop playback; defaults and flags still work without it.
		fmt.Fprintf(os.Stderr, "config: %v; using defaults\n", err)
	}

	dir, cfg, err := parseArgs(os.Args[1:], configPath)
	if err != nil {
		// flag.ErrHelp means -h already printed the usage, so echoing the
		// error on top of it would turn a routine request into a failure.
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, err)
		}
		return
	}

	// With no directory on the command line there is nothing to scan yet, so
	// ask before starting a player. Choosing first means a canceled picker
	// never leaves an mpv process behind.
	var tracks []playlist.Track
	var failures []error

	if dir == "" {
		start, notice := "", ""
		for {
			dir, err = pickDir(cfg, start, notice)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}
			if dir == "" {
				return
			}

			tracks, failures, err = loader.Load(dir)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}
			if len(tracks) > 0 {
				break
			}

			// Reopen on the rejected folder instead of exiting: the usual
			// cause is a wrong branch, and making the listener navigate back
			// from $HOME would be the whole cost of the mistake.
			start = dir
			notice = fmt.Sprintf("No tracks found in %s.", dir)
		}
	} else {
		tracks, failures, err = loader.Load(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
	}

	// A few unreadable files should not stop the whole library from
	// loading, but they should not pass unnoticed either.
	if len(failures) > 0 {
		fmt.Fprintf(os.Stderr, "%d of %d tracks could not be read; continuing anyway.\n",
			len(failures), len(tracks))
		for _, failure := range failures {
			fmt.Fprintf(os.Stderr, "  %v\n", failure)
		}
	}

	if len(tracks) == 0 {
		fmt.Fprintf(os.Stderr, "No tracks found in %s.\n", dir)
		return
	}

	p, err := player.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer p.Close()

	p.SetPlaylist(tracks)

	m := ui.New(p, tracks, cfg)
	pgm := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := pgm.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		// os.Exit skips deferred calls, so the close the defer above would
		// have done has to happen here too. Without it mpv keeps running
		// and holds its private socket directory open. This is reachable
		// whenever the TUI cannot start, such as when stdin is not a
		// terminal.
		_ = p.Close()
		os.Exit(1)
	}
}

// pickDir runs the folder browser and returns the chosen directory, or "" when
// the listener canceled. An empty start opens at $HOME, with the working
// directory as a fallback when $HOME is unavailable. notice is shown above the
// listing to explain why a previous choice was rejected.
//
// It is a separate program from the playlist UI because no player exists until
// this returns: keeping the two apart means the picker cannot touch playback
// state, and canceling starts nothing.
func pickDir(cfg ui.Config, start, notice string) (string, error) {
	if start == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Without $HOME the working directory is the only meaningful
			// place to start, and it is already absolute.
			start, err = os.Getwd()
			if err != nil {
				return "", err
			}
		} else {
			start = home
		}
	}

	picker := ui.NewPicker(start, cfg).WithNotice(notice)
	final, err := tea.NewProgram(picker, tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}

	picker, ok := final.(ui.Picker)
	if !ok {
		return "", fmt.Errorf("picker returned %T, want ui.Picker", final)
	}
	if picker.Canceled() {
		return "", nil
	}
	return picker.Chosen(), nil
}

// userConfigPath is where preferences live: the XDG config directory plus
// termelody/config. The file itself is optional; a missing one simply means
// defaults, and the path is still useful in the usage text.
func userConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "termelody", "config"), nil
}

// parseArgs turns the command line into the directory to scan and the TUI
// configuration. configPath is the preference file, or "" to skip it. Its
// values become the flag defaults, so an explicit flag still wins. The flag
// package stops at the first non-flag argument, so whatever it leaves over goes
// to scanDir unchanged and the music directory stays optional. An empty
// directory means none was given, so the picker chooses one.
func parseArgs(args []string, configPath string) (string, ui.Config, error) {
	cfg, err := ui.LoadConfig(configPath)
	if err != nil {
		return "", ui.Config{}, err
	}

	flags := flag.NewFlagSet("termelody", flag.ContinueOnError)
	seek := flags.Duration("seek", cfg.SeekStep, "how far the arrow keys jump, e.g. 10s")
	hidden := flags.Bool("hidden", cfg.ShowHidden, "start the folder picker with dot-directories shown")
	icons := flags.String("icons", string(cfg.Icons), "row icons: unicode, nerd (needs a patched font), or none")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "usage: termelody [-seek duration] [-hidden] [-icons set] [music directory]\n")
		if configPath != "" {
			fmt.Fprintf(flags.Output(), "Settings are read from %s (if present).\n", configPath)
		}
		fmt.Fprintln(flags.Output(), "With no music directory, a folder picker opens at $HOME.")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return "", ui.Config{}, err
	}

	// Only an explicit -seek can be non-positive, because the flag default is
	// the positive DefaultSeekStep or a validated config value. ui.Config's
	// defaults would quietly swap such a value back, which hides the user's
	// mistake; refusing it here reports it instead.
	if *seek <= 0 {
		return "", ui.Config{}, fmt.Errorf("-seek must be positive, got %s", *seek)
	}

	iconSet, err := ui.ParseIconSet(*icons)
	if err != nil {
		return "", ui.Config{}, fmt.Errorf("-icons: %w", err)
	}

	dir, err := scanDir(flags.Args())
	if err != nil {
		return "", ui.Config{}, err
	}

	return dir, ui.Config{
		SeekStep:   *seek,
		ShowHidden: *hidden,
		Icons:      iconSet,
		VolumeStep: cfg.VolumeStep,
	}, nil
}

// scanDir resolves a directory named on the command line. An empty result
// means none was given, which is how the picker becomes the no-argument
// behavior rather than an implicit default. A named path is made absolute so
// it does not depend on where termelody was launched.
func scanDir(args []string) (string, error) {
	switch len(args) {
	case 0:
		return "", nil
	case 1:
		return filepath.Abs(args[0])
	default:
		return "", fmt.Errorf("usage: termelody [-seek duration] [-hidden] [-icons set] [music directory]")
	}
}
