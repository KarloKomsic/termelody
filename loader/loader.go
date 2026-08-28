package loader

import (
	"codeberg.org/karlokomsic/termelody/metadata"
	"codeberg.org/karlokomsic/termelody/playlist"
	"codeberg.org/karlokomsic/termelody/scanner"
)

func Load(path string) ([]playlist.Track, error) {
	tracks, err := scanner.Scan(path)
	if err != nil {
		return nil, err
	}

	// Enrich each discovered Track with whatever metadata
	// can be read from its file. Tracks whose metadata cannot
	// be read are kept with their path-only information.
	for i, track := range tracks {
		loadedTrack, err := metadata.Load(track)
		if err != nil {
			continue
		}

		tracks[i] = loadedTrack
	}

	return tracks, nil
}
