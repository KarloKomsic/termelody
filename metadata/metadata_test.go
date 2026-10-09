package metadata

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/karlokomsic/termelody/playlist"
)

// writeID3v23File writes an ID3v2.3 file holding the given text frames to a
// temporary directory and returns its path. The tag is assembled by hand so
// the fixture stays readable and the repository holds no binary test files.
// tag.ReadFrom only inspects the tag itself, so no MPEG frames are needed
// behind it.
func writeID3v23File(t *testing.T, frames ...[]byte) string {
	t.Helper()

	var payload bytes.Buffer
	for _, frame := range frames {
		payload.Write(frame)
	}
	// The parser stops at a zero-length frame, so trailing zeros give it an
	// unambiguous end instead of running into EOF. It reads a full frame
	// header before noticing that, hence at least ten bytes of padding.
	payload.Write(make([]byte, 64))

	var tag bytes.Buffer
	tag.WriteString("ID3")
	tag.Write([]byte{3, 0, 0}) // version 2.3.0, no header flag bits set
	tag.Write(syncsafe(payload.Len()))
	tag.Write(payload.Bytes())

	path := filepath.Join(t.TempDir(), "track.mp3")
	if err := os.WriteFile(path, tag.Bytes(), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

// textFrame builds one ID3v2.3 text frame: four-character name, big-endian
// payload length, two flag bytes, then an ISO-8859-1 encoding byte (0x00)
// followed by the text.
func textFrame(name, text string) []byte {
	body := append([]byte{0x00}, text...)

	frame := make([]byte, 0, 10+len(body))
	frame = append(frame, name...)
	frame = append(frame,
		byte(len(body)>>24), byte(len(body)>>16),
		byte(len(body)>>8), byte(len(body)),
		0x00, 0x00, // status and format flags
	)
	return append(frame, body...)
}

// syncsafe encodes n the ID3v2 way: every byte keeps its top bit clear so a
// size can never be mistaken for an MPEG frame marker.
func syncsafe(n int) []byte {
	return []byte{
		byte(n>>21) & 0x7f,
		byte(n>>14) & 0x7f,
		byte(n>>7) & 0x7f,
		byte(n) & 0x7f,
	}
}

func TestLoadCopiesTagsOntoTrack(t *testing.T) {
	path := writeID3v23File(t,
		textFrame("TIT2", "Sharp Dressed Man"),
		textFrame("TPE1", "ZZ Top"),
		textFrame("TALB", "Eliminator"),
	)

	got, err := Load(playlist.Track{Path: path})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := playlist.Track{
		Path:   path,
		Title:  "Sharp Dressed Man",
		Artist: "ZZ Top",
		Album:  "Eliminator",
	}
	if got != want {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
}

func TestLoadReportsMissingFile(t *testing.T) {
	_, err := Load(playlist.Track{Path: filepath.Join(t.TempDir(), "gone.mp3")})
	if err == nil {
		t.Fatal("expected an error for a file that does not exist")
	}
}

func TestLoadReportsUnrecognisedFile(t *testing.T) {
	// Neither an ID3 header, nor an ID3v1 trailer, nor any other container
	// the tag library knows. A plain text file must surface an error so the
	// loader can count it among the failures instead of loading it silently.
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, bytes.Repeat([]byte("not audio "), 16), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	if _, err := Load(playlist.Track{Path: path}); err == nil {
		t.Fatal("expected an error for a file without any recognised tags")
	}
}

func TestLoadTagWithoutTextFramesYieldsEmptyFields(t *testing.T) {
	// A well-formed container with nothing in it is a successful read of
	// nothing: no error, and the empty fields the caller decides how to
	// display. This must not be confused with the unrecognised-file case.
	got, err := Load(playlist.Track{Path: writeID3v23File(t)})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Title != "" || got.Artist != "" || got.Album != "" {
		t.Errorf("empty tag produced %+v, want empty fields", got)
	}
}
