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
	"codeberg.org/karlokomsic/termelody/ui"
)

func main() {
	dir, cfg, err := parseArgs(os.Args[1:])
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
	if dir == "" {
		dir, err = pickDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if dir == "" {
			return
		}
	}

	tracks, failures, err := loader.Load(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
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
// the listener canceled. It is a separate program from the playlist UI because
// no player exists until this returns: keeping the two apart means the picker
// cannot touch playback state, and canceling starts nothing.
func pickDir() (string, error) {
	start, err := os.UserHomeDir()
	if err != nil {
		// Without $HOME the working directory is the only meaningful place
		// to start, and it is already absolute.
		start, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	final, err := tea.NewProgram(ui.NewPicker(start), tea.WithAltScreen()).Run()
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

// parseArgs turns the command line into the directory to scan and the TUI
// configuration. The flag package stops at the first non-flag argument, so
// whatever it leaves over goes to scanDir unchanged and the music directory
// stays optional. An empty directory means none was given, so the picker
// chooses one.
func parseArgs(args []string) (string, ui.Config, error) {
	flags := flag.NewFlagSet("termelody", flag.ContinueOnError)
	seek := flags.Duration("seek", ui.DefaultSeekStep, "how far the arrow keys jump, e.g. 10s")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "usage: termelody [-seek duration] [music directory]\n")
		fmt.Fprintln(flags.Output(), "With no music directory, a folder picker opens at $HOME.")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return "", ui.Config{}, err
	}

	// Only an explicit -seek can be non-positive, because the flag default is
	// the positive DefaultSeekStep. ui.Config.withDefaults would quietly swap
	// such a value back to the default, which hides the user's mistake;
	// refusing it here reports it instead.
	if *seek <= 0 {
		return "", ui.Config{}, fmt.Errorf("-seek must be positive, got %s", *seek)
	}

	dir, err := scanDir(flags.Args())
	if err != nil {
		return "", ui.Config{}, err
	}

	return dir, ui.Config{SeekStep: *seek}, nil
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
		return "", fmt.Errorf("usage: termelody [-seek duration] [music directory]")
	}
}
