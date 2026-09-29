package loader

import (
	"errors"
	"fmt"

	"github.com/dhowden/tag"

	"codeberg.org/karlokomsic/termelody/metadata"
	"codeberg.org/karlokomsic/termelody/playlist"
	"codeberg.org/karlokomsic/termelody/scanner"
)

// Load discovers the tracks under path and enriches each one with the metadata
// that can be read from its file.
//
// A file with no tags is not treated as a failure: tag.ReadFrom reports
// ErrNoTagsFound for any file with no recognizable tag block, which is the
// normal state of plenty of perfectly playable audio, so those tracks are
// returned with their path alone and reported in neither result.
//
// The returned failures hold only what is genuinely wrong, such as a file that
// cannot be opened or read, so the caller can warn about them while the rest of
// the library still loads. The error is reserved for the scan itself failing,
// which means there are no tracks to return at all.
func Load(path string) (tracks []playlist.Track, failures []error, err error) {
	tracks, err = scanner.Scan(path)
	if err != nil {
		return nil, nil, fmt.Errorf("could not scan %s: %w", path, err)
	}

	for i, track := range tracks {
		loadedTrack, err := metadata.Load(track)
		if err == nil {
			tracks[i] = loadedTrack
			continue
		}

		if errors.Is(err, tag.ErrNoTagsFound) {
			continue
		}

		failures = append(failures, fmt.Errorf("%s: %w", track.Path, err))
	}

	return tracks, failures, nil
}
