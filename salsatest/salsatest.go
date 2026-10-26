// Package salsatest builds GT7 datagrams for tests, the way the PS5 sends them, so
// salsa's tests and its importers' (ecu's) can feed the decoder known packets.
package salsatest

import (
	"encoding/binary"

	"github.com/those-who-run/salsa"
	"github.com/those-who-run/salsa/internal/keystream"
)

// Encode returns the datagram the PS5 would send for p: p.Plain(), encrypted with
// GT7's keystream for p's kind and p.Seed, which is written in clear at 0x40 of the
// datagram. salsa.Decrypt and salsa.Parse give back p, Seed included, but for IV:
// it decrypts to the seed XOR the keystream, as it does for a real datagram, so
// whatever p.IV held is lost. Compare with IV cleared on both sides.
func Encode(p salsa.Packet) []byte {
	plain := p.Plain()
	datagram := make([]byte, len(plain.Bytes))
	keystream.XOR(datagram, plain.Bytes, p.Seed, keystream.Mask(len(datagram)))
	binary.LittleEndian.PutUint32(datagram[keystream.SeedOffset:], p.Seed)
	return datagram
}
