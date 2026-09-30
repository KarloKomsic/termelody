package scanner

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"codeberg.org/karlokomsic/termelody/playlist"
)

// touch creates an empty file at path, making any missing parent directory.
// The scanner decides by name alone, so what is inside the file is irrelevant.
func touch(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// paths unwraps tracks so a test can compare against an expected list in one
// assertion instead of a loop.
func paths(tracks []playlist.Track) []string {
	out := make([]string, len(tracks))
	for i, track := range tracks {
		out[i] = track.Path
	}
	return out
}

func TestScanFindsFilesInSubdirectories(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "root.mp3"))
	touch(t, filepath.Join(dir, "Artist", "02 - other.ogg"))
	touch(t, filepath.Join(dir, "Artist", "Album", "01 - song.flac"))
	touch(t, filepath.Join(dir, "a", "b", "c", "deep.wav"))

	tracks, failures, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("failures = %v, want none", failures)
	}

	// os.ReadDir sorts each directory, and the walk goes depth first as it
	// goes, so this order is what a listener would actually hear.
	want := []string{
		filepath.Join(dir, "Artist", "02 - other.ogg"),
		filepath.Join(dir, "Artist", "Album", "01 - song.flac"),
		filepath.Join(dir, "a", "b", "c", "deep.wav"),
		filepath.Join(dir, "root.mp3"),
	}
	if got := paths(tracks); !slices.Equal(got, want) {
		t.Errorf("Scan returned\n%v\nwant\n%v", got, want)
	}
}

// WalkDir would find nothing here, because it inspects the root with Lstat
// and refuses to descend a symlink. Opening the path the way os.ReadDir does
// is what makes a symlinked music folder work.
func TestScanFollowsTheDirectoryWhenItIsItselfASymlink(t *testing.T) {
	target := t.TempDir()
	touch(t, filepath.Join(target, "song.mp3"))

	link := filepath.Join(t.TempDir(), "music")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	tracks, failures, err := Scan(link)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("failures = %v, want none", failures)
	}

	want := []string{filepath.Join(link, "song.mp3")}
	if got := paths(tracks); !slices.Equal(got, want) {
		t.Errorf("Scan returned %v, want %v", got, want)
	}
}

func TestScanDoesNotDescendSymlinkedDirectories(t *testing.T) {
	// The target sits outside the scanned tree, so the only way to reach it
	// is by following the link. If it shows up, links are being followed.
	target := t.TempDir()
	touch(t, filepath.Join(target, "secret.mp3"))

	dir := t.TempDir()
	touch(t, filepath.Join(dir, "visible.mp3"))
	if err := os.Symlink(target, filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	tracks, _, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	want := []string{filepath.Join(dir, "visible.mp3")}
	if got := paths(tracks); !slices.Equal(got, want) {
		t.Errorf("Scan returned %v, want %v", got, want)
	}
}

// A link pointing back at an ancestor is the case that never terminates, so
// this test is also the guard against walking off into a recursion that has
// no bottom.
func TestScanDoesNotLoopOnASymlinkToAnAncestor(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "song.mp3"))
	if err := os.Symlink(dir, filepath.Join(dir, "loop")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	tracks, failures, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("failures = %v, want none", failures)
	}

	want := []string{filepath.Join(dir, "song.mp3")}
	if got := paths(tracks); !slices.Equal(got, want) {
		t.Errorf("Scan returned %v, want %v", got, want)
	}
}

// A directory that cannot be listed is reported rather than passed over in
// silence, and must not stop the rest of the library from loading.
func TestScanReportsUnreadableDirectoriesAndContinues(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, which can read anything")
	}

	dir := t.TempDir()
	touch(t, filepath.Join(dir, "open.mp3"))

	locked := filepath.Join(dir, "locked")
	touch(t, filepath.Join(locked, "hidden.mp3"))
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	tracks, failures, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	want := []string{filepath.Join(dir, "open.mp3")}
	if got := paths(tracks); !slices.Equal(got, want) {
		t.Errorf("Scan returned %v, want %v", got, want)
	}
	if len(failures) != 1 {
		t.Fatalf("failures = %v, want exactly the locked directory", failures)
	}
}

func TestScanErrorsWhenTheDirectoryDoesNotExist(t *testing.T) {
	tracks, failures, err := Scan(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected an error for a missing directory")
	}
	if tracks != nil {
		t.Errorf("tracks = %v, want nil", tracks)
	}
	if failures != nil {
		t.Errorf("failures = %v, want nil", failures)
	}
}

func TestScanIgnoresUnsupportedFilesAtAnyDepth(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "cover.jpg"))
	touch(t, filepath.Join(dir, "notes.txt"))
	touch(t, filepath.Join(dir, "Album", "cover.jpg"))
	touch(t, filepath.Join(dir, "Album", "track.mp3"))
	touch(t, filepath.Join(dir, "Album", "Track.FLAC"))

	tracks, _, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// Sorted by name, so the uppercase T in Track.FLAC comes before both
	// lowercase entries no matter which extension is being kept.
	want := []string{
		filepath.Join(dir, "Album", "Track.FLAC"),
		filepath.Join(dir, "Album", "track.mp3"),
	}
	if got := paths(tracks); !slices.Equal(got, want) {
		t.Errorf("Scan returned %v, want %v", got, want)
	}
}
