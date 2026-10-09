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
		fmt.Fprintln(os.Stderr, "No tracks found.")
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

// parseArgs turns the command line into the directory to scan and the TUI
// configuration. The flag package stops at the first non-flag argument, so
// whatever it leaves over goes to scanDir unchanged and the music directory
// stays optional.
func parseArgs(args []string) (string, ui.Config, error) {
	flags := flag.NewFlagSet("termelody", flag.ContinueOnError)
	seek := flags.Duration("seek", ui.DefaultSeekStep, "how far the arrow keys jump, e.g. 10s")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "usage: termelody [-seek duration] [music directory]\n")
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

// defaultMusicDir is what gets scanned when the command line names no
// directory. It stays relative so that running termelody from the repository
// root keeps finding the local music/ folder.
const defaultMusicDir = "music"

// scanDir resolves which directory to scan: the first argument when one is
// given, and the default otherwise. Both go through filepath.Abs here so a
// directory named on the command line and the default are resolved the same
// way, rather than one being relative to wherever termelody was launched.
func scanDir(args []string) (string, error) {
	switch len(args) {
	case 0:
		return filepath.Abs(defaultMusicDir)
	case 1:
		return filepath.Abs(args[0])
	default:
		return "", fmt.Errorf("usage: termelody [-seek duration] [music directory]")
	}
}
