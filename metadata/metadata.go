package metadata

import (
	"os"

	"codeberg.org/karlokomsic/termelody/playlist"
	"github.com/dhowden/tag"
)

func Load(track playlist.Track) (playlist.Track, error) {
	// Opening a file should be pretty simple
	// and straight-forward, but the defer
	// part is because we want to close it
	// after the function ends
	file, err := os.Open(track.Path)
	if err != nil {
		return track, err
	}
	defer file.Close()

	// We need to read the metadata from the file,
	// so we use ReadFrom the file and extract all
	// the metadata into "m"
	m, err := tag.ReadFrom(file)
	if err != nil {
		return track, err
	}

	track.Title = m.Title()
	track.Artist = m.Artist()
	track.Album = m.Album()

	return track, nil
}
