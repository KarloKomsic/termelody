package main

import (
	"fmt"

	"codeberg.org/karlokomsic/termelody/scanner"
)

func main() {
	tracks, err := scanner.Scan("music")
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, track := range tracks {
		fmt.Println(track)
	}
}
