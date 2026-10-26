package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var fixture = filepath.Join("..", "..", "testdata", "v1", "real-nuc-suzuka-pit-stop-2026-10-15.jsonl.gz")

func readFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(fixture) //nolint:gosec // a fixture under testdata
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// firstFrame returns the fixture's first line, as v1 wrote it.
func firstFrame(t *testing.T) string {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(readFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(string(b), "\n")
	return first
}

// R7: salsa cat prints one line per frame, each a decoded packet.
func TestCat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"cat", fixture}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, &stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 874 {
		t.Fatalf("%d lines, want 874", len(lines))
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{"kind": "A", "from_v1_recording": true, "PacketID": 43645.0, "CarCode": 683.0, "CurrentLap": -1.0} {
		if first[key] != want {
			t.Errorf("%s = %v, want %v", key, first[key], want)
		}
	}
	if _, ok := first["t"].(float64); !ok {
		t.Errorf("t = %v", first["t"])
	}
}

// salsa cat - reads standard input.
func TestCatStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"cat", "-"}, bytes.NewReader(readFixture(t)), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, &stderr)
	}
	if n := strings.Count(stdout.String(), "\n"); n != 874 {
		t.Errorf("%d lines, want 874", n)
	}
}

// A bad recording is status 1, with a JSON log line naming the file and the line;
// a usage error is status 2.
func TestCatErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		args  []string
		stdin string
		code  int
		want  string
	}{
		"no arguments":          {nil, "", 2, "usage"},
		"not cat":               {[]string{"dog", "x"}, "", 2, "usage"},
		"no file":               {[]string{"cat", filepath.Join("no", "such.jsonl")}, "", 1, "no such file"},
		"a bad line":            {[]string{"cat", "-"}, "{}\n", 1, `standard input:1: no \"t\"`},
		"a gzip header":         {[]string{"cat", "-"}, "\x1f\x8b\x00", 1, "standard input: damaged gzip header"},
		"a frame, then no JSON": {[]string{"cat", "-"}, firstFrame(t) + "\nnot json\n", 1, "standard input:2: not a v1 frame"},
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), tc.args, strings.NewReader(tc.stdin), &stdout, &stderr)
			if code != tc.code || !strings.Contains(stderr.String(), tc.want) {
				t.Errorf("exit %d, stderr %q; want %d and %q", code, &stderr, tc.code, tc.want)
			}
		})
	}
}

// A write that fails stops it with status 1.
func TestCatWriteError(t *testing.T) {
	var stderr bytes.Buffer
	if code := run(context.Background(), []string{"cat", fixture}, nil, failingWriter{}, &stderr); code != 1 {
		t.Errorf("exit %d", code)
	}
	if !strings.Contains(stderr.String(), "closed") {
		t.Errorf("stderr %q", &stderr)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }
