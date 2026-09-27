package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"codeberg.org/karlokomsic/termelody/loader"
	"codeberg.org/karlokomsic/termelody/player"
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

	fmt.Printf("Loaded %d track(s):\n", len(tracks))
	for i, t := range tracks {
		fmt.Printf("  %d. %s - %s\n", i+1, t.Artist, t.Title)
	}
	fmt.Println()

	p, err := player.New()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer p.Close()

	p.SetPlaylist(tracks)

	fmt.Println("Controls: 1-9 play, n next, b prev, p pause, s stop, q quit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())

		switch {
		case input == "q":
			return

		case input == "p":
			switch p.State() {
			case player.StatePlaying:
				p.Pause()
				fmt.Println("Paused")
			case player.StatePaused:
				p.Resume()
				fmt.Println("Resumed")
			default:
				fmt.Println("Nothing playing")
			}

		case input == "s":
			p.Stop()
			fmt.Println("Stopped")

		case input == "n":
			if err := p.Next(); err != nil {
				fmt.Println(err)
			} else {
				t := p.Track()
				fmt.Printf("Playing: %s - %s\n", t.Artist, t.Title)
			}

		case input == "b":
			if err := p.Prev(); err != nil {
				fmt.Println(err)
			} else {
				t := p.Track()
				fmt.Printf("Playing: %s - %s\n", t.Artist, t.Title)
			}

		default:
			n, err := strconv.Atoi(input)
			if err != nil || n < 1 || n > len(tracks) {
				fmt.Println("Unknown command")
				continue
			}
			if err := p.PlayIndex(n - 1); err != nil {
				fmt.Println(err)
				continue
			}
			t := p.Track()
			fmt.Printf("Playing: %s - %s\n", t.Artist, t.Title)
		}
	}
}
