package main

import (
	"fmt"

	"codeberg.org/karlokomsic/termelody/metadata"
	"codeberg.org/karlokomsic/termelody/scanner"
)

func main() {
	// This "music" is temporary.
	// In the future, when we launch termelody without arguements,
	// we want for it to open an interactive file browser in the TUI.
	// With arguement, it can just open the directory without needing the browser
	tracks, err := scanner.Scan("music")
	if err != nil {
		fmt.Println(err)
		return
	}

	// After a track is scanned, we want to load its metadata
	// if it has any
	for _, track := range tracks {
		track, err = metadata.Load(track)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(track)
	}
}
