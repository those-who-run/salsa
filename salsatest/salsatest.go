// Package salsatest builds GT7 datagrams for tests, the way the PS5 sends them, so
// salsa's tests and its importers' (ecu's) can feed the decoder known packets.
package salsatest

import (
	"encoding/binary"

	"github.com/those-who-run/salsa"
	"github.com/those-who-run/salsa/internal/keystream"
)

// Encode returns the datagram the PS5 would send for p: p.Plain(), encrypted with
// GT7's keystream for p's kind and seeded with p.IV, which is written at 0x40 of the
// datagram. salsa.Decrypt and salsa.Parse give back p, but for IV, which decrypts to
// the seed XOR the keystream as it does for a real datagram.
func Encode(p salsa.Packet) []byte {
	plain := p.Plain()
	datagram := make([]byte, len(plain.Bytes))
	keystream.XOR(datagram, plain.Bytes, p.IV, mask(plain.Kind))
	binary.LittleEndian.PutUint32(datagram[keystream.SeedOffset:], p.IV)
	return datagram
}

func mask(k salsa.Kind) uint32 {
	switch k {
	case salsa.KindB:
		return keystream.MaskB
	case salsa.KindTilde:
		return keystream.MaskTilde
	default:
		return keystream.MaskA
	}
}
