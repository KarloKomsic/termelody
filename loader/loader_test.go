package loader

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildFile creates an audio fixture with ffmpeg. Returns false when ffmpeg is
// unavailable, so callers skip rather than fail.
func buildFile(t *testing.T, dir string, name string, extraArgs ...string) bool {
	t.Helper()

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return false
	}

	args := []string{"-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono", "-t", "1"}
	args = append(args, extraArgs...)
	args = append(args, "-y", filepath.Join(dir, name))

	if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg could not build a fixture: %v: %s", err, out)
	}

	return true
}

// audioDir builds a directory of tracks under a fresh temp dir, so each test
// gets its own library.
func audioDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func TestLoadKeepsUntaggedFilesWithoutReportingThem(t *testing.T) {
	dir := audioDir(t)
	if !buildFile(t, dir, "untagged.wav") {
		return
	}

	tracks, failures, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("got %d tracks, want 1", len(tracks))
	}
	if len(failures) != 0 {
		t.Errorf("reported %d failures for a tagless file: %v", len(failures), failures)
	}
	if tracks[0].Title != "" {
		t.Errorf("Title = %q, want empty for a file with no tags", tracks[0].Title)
	}
}

func TestLoadFillsInMetadataWhenTagsArePresent(t *testing.T) {
	dir := audioDir(t)
	if !buildFile(t, dir, "tagged.mp3", "-metadata", "title=Skyline", "-metadata", "artist=Someone") {
		return
	}

	tracks, failures, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(failures) != 0 {
		t.Errorf("unexpected failures: %v", failures)
	}
	if len(tracks) != 1 {
		t.Fatalf("got %d tracks, want 1", len(tracks))
	}
	if tracks[0].Title != "Skyline" {
		t.Errorf("Title = %q, want Skyline", tracks[0].Title)
	}
	if tracks[0].Artist != "Someone" {
		t.Errorf("Artist = %q, want Someone", tracks[0].Artist)
	}
}

func TestLoadReportsFilesThatCannotBeRead(t *testing.T) {
	dir := audioDir(t)

	// A file too short to hold a single header makes tag.ReadFrom fail with
	// something other than ErrNoTagsFound, which is what should be reported.
	if err := os.WriteFile(filepath.Join(dir, "broken.mp3"), []byte("id3"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tracks, failures, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("got %d failures, want 1", len(failures))
	}
	if len(tracks) != 1 {
		t.Errorf("got %d tracks, want the unreadable one kept", len(tracks))
	}
}

func TestLoadDistinguishesMissingTagsFromBrokenFiles(t *testing.T) {
	dir := audioDir(t)
	if !buildFile(t, dir, "untagged.wav") {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.mp3"), []byte("id3"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	tracks, failures, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Both files load, but only the broken one is reported. Getting this
	// wrong in either direction would either hide real problems or make
	// untagged audio look broken.
	if len(tracks) != 2 {
		t.Errorf("got %d tracks, want 2", len(tracks))
	}
	if len(failures) != 1 {
		t.Fatalf("got %d failures, want exactly the broken file", len(failures))
	}
}

func TestLoadFailsWhenTheDirectoryDoesNotExist(t *testing.T) {
	tracks, failures, err := Load(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected an error for a missing directory")
	}
	if tracks != nil {
		t.Errorf("got %d tracks, want nil", len(tracks))
	}
	if failures != nil {
		t.Errorf("got %d failures, want nil", len(failures))
	}
}

func TestLoadReturnsNothingForAnEmptyDirectory(t *testing.T) {
	tracks, failures, err := Load(audioDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(tracks) != 0 {
		t.Errorf("got %d tracks, want 0", len(tracks))
	}
	if len(failures) != 0 {
		t.Errorf("got %d failures, want 0", len(failures))
	}
}
