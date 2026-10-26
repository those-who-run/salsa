package salsa

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// RecordingError is an error in a recording: a line that is not a v1 frame, or a
// stream that ends damaged (a truncated or corrupt gzip file). Name is the
// recording's path, or the name NewRecording was given, and Line the line, from 1,
// or 0 when the error is in the gzip header, before any line.
type RecordingError struct {
	Name string
	Line int
	Err  error
}

func (e *RecordingError) Error() string {
	if e.Line == 0 {
		return fmt.Sprintf("%s: %v", e.Name, e.Err)
	}
	return fmt.Sprintf("%s:%d: %v", e.Name, e.Line, e.Err)
}

func (e *RecordingError) Unwrap() error { return e.Err }

// maxLine bounds a line of a recording. v1's frames are about 1.2 KB; anything near
// this is not a frame, and the bound keeps a corrupt file from filling memory.
const maxLine = 1 << 20

// gzipMagic starts every gzip stream.
var gzipMagic = []byte{0x1f, 0x8b}

// OpenRecording opens a v1 recording, plain JSON lines or gzip (told from the
// file's first bytes, whatever its name), one RaceState per line as v1's
// RecordingWriter writes them, and returns its frames as packets: FromV1Recording
// says what they hold. A gzip file v1 appended to holds one gzip member per
// append, and they are read as one stream.
//
// Every line is a frame: a line that is not one, blank lines included, is a
// *RecordingError naming the file and the line, and nothing is skipped. So is a
// stream that ends damaged, as a recording cut short by a crash does (v1 read
// those up to the damage only when not strict). A frame in a format version other
// than 1 or 2, a key v1 never wrote, a key missing that v1 always wrote, and a
// value out of its field's range are errors too.
func OpenRecording(path string) (Source, error) {
	f, err := os.Open(path) //nolint:gosec // reading the file the caller names is the point
	if err != nil {
		return nil, err
	}
	src, err := newRecording(f, path)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	src.file = f
	return src, nil
}

// NewRecording reads a v1 recording from r, as OpenRecording reads a file: for
// standard input, or a recording held in memory. name is what errors call it.
// Close does not close r.
func NewRecording(r io.Reader, name string) (Source, error) {
	src, err := newRecording(r, name)
	if err != nil {
		return nil, err
	}
	return src, nil
}

func newRecording(r io.Reader, name string) (*recording, error) {
	src := &recording{name: name}
	in := bufio.NewReader(r)
	head, err := in.Peek(len(gzipMagic))
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, &RecordingError{Name: name, Err: err}
	}
	if !bytes.Equal(head, gzipMagic) {
		src.lines = in
		return src, nil
	}
	src.gz, err = gzip.NewReader(in)
	if err != nil {
		return nil, &RecordingError{Name: name, Err: fmt.Errorf("damaged gzip header: %w", err)}
	}
	src.lines = bufio.NewReader(src.gz)
	return src, nil
}

// recording is the Source OpenRecording and NewRecording return.
type recording struct {
	name  string
	file  *os.File     // the file OpenRecording opened, nil for NewRecording
	gz    *gzip.Reader // nil for plain JSON lines
	lines *bufio.Reader
	line  int   // lines read so far
	err   error // what stopped the source: io.EOF, a *RecordingError or closed
}

var errClosed = errors.New("salsa: recording closed")

func (s *recording) Next(ctx context.Context) (Received, error) {
	if s.err != nil {
		return Received{}, s.err
	}
	if err := ctx.Err(); err != nil {
		return Received{}, err
	}
	line, err := s.readLine()
	if err != nil {
		s.err = err
		return Received{}, err
	}
	r, err := parseV1Frame(line)
	if err != nil {
		s.err = &RecordingError{Name: s.name, Line: s.line, Err: err}
		return Received{}, s.err
	}
	return r, nil
}

// readLine returns the next line without its line ending, io.EOF at the end of the
// stream, or a *RecordingError. The line is valid until the next call.
func (s *recording) readLine() ([]byte, error) {
	var long []byte // a line longer than the buffer, gathered
	for {
		chunk, err := s.lines.ReadSlice('\n')
		if len(long)+len(chunk) > maxLine {
			s.line++
			return nil, &RecordingError{Name: s.name, Line: s.line, Err: fmt.Errorf("a line longer than %d bytes", maxLine)}
		}
		switch {
		case errors.Is(err, bufio.ErrBufferFull):
			long = append(long, chunk...)
			continue
		case err == nil, errors.Is(err, io.EOF):
			if long != nil {
				chunk = append(long, chunk...)
			}
			if errors.Is(err, io.EOF) && len(chunk) == 0 {
				return nil, io.EOF
			}
			// The last line may have no line ending: v1 flushes whole lines, so a
			// recording cut mid-line fails as a frame instead.
			s.line++
			return bytes.TrimRight(chunk, "\r\n"), nil
		default:
			// gzip reports a stream that stops short as io.ErrUnexpectedEOF, and a
			// damaged one as a checksum or flate error.
			return nil, &RecordingError{Name: s.name, Line: s.line + 1, Err: fmt.Errorf("damaged after line %d: %w", s.line, err)}
		}
	}
}

func (s *recording) Close() error {
	if errors.Is(s.err, errClosed) {
		return nil
	}
	s.err = errClosed
	var errs []error
	if s.gz != nil {
		errs = append(errs, s.gz.Close())
	}
	if s.file != nil {
		errs = append(errs, s.file.Close())
	}
	return errors.Join(errs...)
}
