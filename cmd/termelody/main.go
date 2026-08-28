package main

import (
	"fmt"

	"codeberg.org/karlokomsic/termelody/loader"
)

func main() {
	// This "music" is temporary.
	// In the future, when we launch termelody without arguements,
	// we want for it to open an interactive file browser in the TUI.
	// With arguement, it can just open the directory without needing the browser
	tracks, err := loader.Load("music")
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, track := range tracks {
		fmt.Printf(
			"Title: %s\nArtist: %s\nAlbum: %s\nPath: %s\n\n",
			track.Title,
			track.Artist,
			track.Album,
			track.Path,
		)
	}
}
