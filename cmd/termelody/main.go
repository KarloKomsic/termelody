package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"codeberg.org/karlokomsic/termelody/loader"
	"codeberg.org/karlokomsic/termelody/player"
	"codeberg.org/karlokomsic/termelody/ui"
)

func main() {
	tracks, err := loader.Load("music")
	if err != nil {
		fmt.Println(err)
		return
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

	m := ui.New(p, tracks)
	pgm := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := pgm.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
