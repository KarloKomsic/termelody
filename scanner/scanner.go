package scanner

import (
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
	// Normalize the extension because filesystem filenames may use different
	// casing (e.g. ".MP3" vs ".mp3"), but format support itself is case-insensitive.
	ext = strings.ToLower(ext)

	for _, supportedExt := range supportedExtensions {
		if supportedExt == ext {
			return true
		}
	}

	return false
}

func Scan(path string) ([]playlist.Track, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var tracks []playlist.Track

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		ext := filepath.Ext(name)

		if !isSupportedExtension(ext) {
			continue
		}

		fullPath := filepath.Join(path, name)

		track := playlist.Track{
			Path: fullPath,
		}

		tracks = append(tracks, track)
	}

	return tracks, nil
}
