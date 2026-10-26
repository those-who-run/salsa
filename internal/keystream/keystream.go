// Package keystream is GT7's Salsa20 keystream, shared by salsa's Decrypt and
// salsatest's Encode, which encrypts the way the PS5 does. It is internal so the
// module's API keeps decrypting only.
package keystream

import (
	"encoding/binary"

	"golang.org/x/crypto/salsa20"
)

// SeedOffset is where the datagram, as received, carries the seed of its nonce.
const SeedOffset = 0x40

// The IV mask of each packet kind, from gt-telem 1.2.1's PDEncyption (_IV_MASK_A, _B
// and _TILDE). The PS5 encrypts each kind with its own; a datagram decrypted with
// another kind's mask fails the magic check.
const (
	MaskA     uint32 = 0xDEADBEAF
	MaskB     uint32 = 0xDEADBEEF
	MaskTilde uint32 = 0x55FABB4F
)

// Mask returns the IV mask of the packet kind whose datagram is length bytes long:
// 0x13C is B, 0x158 is "~", and anything else gets A's. salsa's Kind and salsatest
// both pick masks here, so the table is kept once.
func Mask(length int) uint32 {
	switch length {
	case 0x13C:
		return MaskB
	case 0x158:
		return MaskTilde
	default:
		return MaskA
	}
}

// key is the first 32 bytes of "Simulator Interface Packet GT7 ver 0.0", gt-telem
// 1.2.1's _GT7_KEY[:32]. GT Sport's key, which gt-telem also has, is out of scope.
// It is unexported so nothing can change it; Key returns a copy.
var key = [32]byte([]byte("Simulator Interface Packet GT7 ver 0.0")[:32])

// Key returns a copy of GT7's Salsa20 key.
func Key() [32]byte { return key }

// XOR runs the keystream of seed and mask over in into out: Salsa20 with Key and the
// 8-byte nonce uint32le(seed ^ mask) then uint32le(seed). The keystream is its own
// inverse, so it both encrypts and decrypts.
func XOR(out, in []byte, seed, mask uint32) {
	var nonce [8]byte
	binary.LittleEndian.PutUint32(nonce[0:], seed^mask)
	binary.LittleEndian.PutUint32(nonce[4:], seed)
	salsa20.XORKeyStream(out, in, nonce[:], &key)
}
