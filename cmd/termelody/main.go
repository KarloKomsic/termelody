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
	// Resolve to an absolute path so playback works regardless of the
	// directory termelody is launched from.
	dir, err := filepath.Abs("music")
	if err != nil {
		fmt.Println(err)
		return
	}

	tracks, failures, err := loader.Load(dir)
	if err != nil {
		fmt.Println(err)
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
		fmt.Println("No tracks found.")
		return
	}

	p, err := player.New()
	if err != nil {
		fmt.Println(err)
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
		fmt.Println(err)
		os.Exit(1)
	}
}
