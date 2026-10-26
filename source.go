package salsa

import (
	"context"
	"io"
)

// Received is a packet with the time it was received.
type Received struct {
	// T is when the packet was received, in seconds, on the source's own clock: for
	// a v1 recording, the frame's t as v1 recorded it. Only differences between
	// times mean anything; T is not wall-clock time.
	T float64
	Packet
}

// Source gives packets one at a time, in the order they were received, from a
// recording now and from the live listener later, so what consumes packets need
// not care where they come from.
//
// Next returns the next packet, io.EOF once there are no more, or the error that
// stopped the source, which every later call returns again. A cancelled ctx makes
// Next return ctx.Err() without reading. Close releases what the source holds;
// Next after Close is an error. A Source is not safe for concurrent use.
type Source interface {
	Next(ctx context.Context) (Received, error)
	io.Closer
}
