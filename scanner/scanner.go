package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/karlokomsic/termelody/playlist"
)

// Keep the supported formats in one place rather than spreading extension
// checks throughout the scanner. Adding a new format should require changing
// only this list.
var supportedExtensions = []string{
	".mp3",
	".flac",
	".ogg",
	".wav",
	".opus",
	".m4a",
	".aac",
	".wma",
	".aiff",
	".ape",
}

func isSupportedExtension(ext string) bool {
	// Normalize the extension because filenames may use different casing
	// (e.g. ".MP3" vs ".mp3"), while format support is case-insensitive.
	ext = strings.ToLower(ext)

	for _, supportedExt := range supportedExtensions {
		if supportedExt == ext {
			return true
		}
	}

	return false
}

// Scan walks dir and every directory beneath it, returning the supported
// audio files it finds.
//
// The directory the caller named is followed even when it is itself a
// symlink, because os.ReadDir opens the path and opening follows links. The
// entries inside are not descended into when they are symlinks, since
// DirEntry.IsDir reports false for one, and following those is how a walk
// runs forever after a link points back at an ancestor.
//
// The root's own failure is returned as an error, because there is nothing
// to fall back on. A directory beneath it that cannot be read goes to
// failures instead, so one unreadable folder does not cost the library.
func Scan(dir string) ([]playlist.Track, []error, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}

	tracks, failures := descend(dir, entries, nil, nil)
	return tracks, failures, nil
}

// descend appends the audio files held by one directory to tracks, recursing
// into each of its subdirectories, and collects any directory it cannot read
// into failures.
func descend(dir string, entries []os.DirEntry, tracks []playlist.Track, failures []error) ([]playlist.Track, []error) {
	for _, entry := range entries {
		full := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			sub, err := os.ReadDir(full)
			if err != nil {
				failures = append(failures, fmt.Errorf("%s: %w", full, err))
				continue
			}

			tracks, failures = descend(full, sub, tracks, failures)
			continue
		}

		if isSupportedExtension(filepath.Ext(entry.Name())) {
			tracks = append(tracks, playlist.Track{Path: full})
		}
	}

	return tracks, failures
}
