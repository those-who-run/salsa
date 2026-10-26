package salsa

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Close closes the file OpenRecording opened: closing it again fails, as it would
// not if Close had left it open.
func TestRecordingClosesFile(t *testing.T) {
	src, err := OpenRecording(filepath.Join("testdata", "v1", "real-race-2laps-2026-10-14.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	rec, ok := src.(*recording)
	if !ok || rec.file == nil {
		t.Fatalf("OpenRecording returned %T, with no file", src)
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rec.file.Close(); !errors.Is(err, os.ErrClosed) {
		t.Errorf("the file is still open after Close: closing it again gave %v", err)
	}
}
