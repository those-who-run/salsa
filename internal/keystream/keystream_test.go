package keystream

import (
	"bytes"
	"testing"
)

// The keystream undoes itself, and each mask gives its own.
func TestXOR(t *testing.T) {
	in := bytes.Repeat([]byte("0S7G"), 74)
	var streams [][]byte
	for _, mask := range []uint32{MaskA, MaskB, MaskTilde} {
		out := make([]byte, len(in))
		XOR(out, in, 0x01234567, mask)
		back := make([]byte, len(in))
		XOR(back, out, 0x01234567, mask)
		if !bytes.Equal(back, in) {
			t.Errorf("mask %#x: XOR twice is not the identity", mask)
		}
		for _, other := range streams {
			if bytes.Equal(other, out) {
				t.Errorf("mask %#x: same keystream as another mask", mask)
			}
		}
		streams = append(streams, out)
	}
}
