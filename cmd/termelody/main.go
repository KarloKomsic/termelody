package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/karlokomsic/termelody/loader"
	"codeberg.org/karlokomsic/termelody/player"
	"codeberg.org/karlokomsic/termelody/ui"
)

func main() {
	dir, err := scanDir(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	tracks, failures, err := loader.Load(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	// Say what was actually scanned. The directory may have been named on
	// the command line or resolved from the default, and an argument that
	// was quietly misread looks exactly like an empty library otherwise.
	fmt.Fprintf(os.Stderr, "%s: %d tracks\n", dir, len(tracks))

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

	// cfg is the single place TUI options get set. The zero value means
	// "use the defaults", so only overrides need to appear here. Adding a
	// flag or config file later means parsing it into this struct and
	// nothing in the ui package changes.
	cfg := ui.Config{}

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
		return "", fmt.Errorf("usage: termelody [music directory]")
	}
}
