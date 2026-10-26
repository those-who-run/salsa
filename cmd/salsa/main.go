// Command salsa prints a v1 recording's frames as the packets salsa reads from
// them, one JSON object per line, for debugging:
//
//	salsa cat file.jsonl.gz | head
//	salsa cat - < file.jsonl
//
// Each line has the frame's time (t), the packet's kind, whether it came from a v1
// recording, and the packet's fields under their Go names, with speed in m/s and the
// flags word and gears byte as numbers. A bad recording stops it with exit status 1
// and a JSON log line on standard error naming the file and the line; a usage
// error is status 2.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/those-who-run/salsa"
)

const usage = "usage: salsa cat FILE (.jsonl or .jsonl.gz, or - for standard input)"

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// line is what cat prints for each packet.
type line struct {
	T               float64 `json:"t"`
	Kind            string  `json:"kind"`
	FromV1Recording bool    `json:"from_v1_recording"`
	salsa.Packet
}

// run is the command with its arguments and streams, returning the exit status.
// The log goes to stderr, not stdout as the template's services log, because
// stdout carries the packets.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	log := slog.New(slog.NewJSONHandler(stderr, nil))
	if len(args) != 2 || args[0] != "cat" {
		_, _ = fmt.Fprintln(stderr, usage)
		return 2
	}
	var src salsa.Source
	var err error
	if args[1] == "-" {
		src, err = salsa.NewRecording(stdin, "standard input")
	} else {
		src, err = salsa.OpenRecording(args[1])
	}
	if err != nil {
		log.ErrorContext(ctx, "cannot read the recording", "err", err.Error())
		return 1
	}
	defer func() { _ = src.Close() }()
	out := json.NewEncoder(stdout)
	for {
		r, err := src.Next(ctx)
		if errors.Is(err, io.EOF) {
			return 0
		}
		if err != nil {
			log.ErrorContext(ctx, "cannot read the recording", "err", err.Error())
			return 1
		}
		if err := out.Encode(line{T: r.T, Kind: r.Kind().String(), FromV1Recording: r.FromV1Recording(), Packet: r.Packet}); err != nil {
			log.ErrorContext(ctx, "cannot write the packets", "err", err.Error())
			return 1
		}
	}
}
