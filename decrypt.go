package salsa

import (
	"encoding/binary"
	"errors"
	"fmt"

	"golang.org/x/crypto/salsa20"
)

// Kind is the layout of a packet, known from its length before it is decrypted.
type Kind uint8

// The packet kinds GT7 sends, one for each heartbeat. The zero Kind is none of them.
const (
	// KindA is the standard packet, 296 bytes, sent for HeartbeatA.
	KindA Kind = iota + 1
	// KindB is A with the motion block, 316 bytes, sent for HeartbeatB.
	KindB
	// KindTilde is B with the "~" block, 344 bytes, sent for HeartbeatTilde.
	KindTilde
)

// The datagram length of each kind, in bytes.
const (
	// LenA is the length of an A packet.
	LenA = 0x128
	// LenB is the length of a B packet.
	LenB = 0x13C
	// LenTilde is the length of a "~" packet.
	LenTilde = 0x158
)

// Magic is the first four bytes of every GT7 plaintext, little-endian: ASCII "0S7G".
const Magic uint32 = 0x47375330

// seedOffset is where the datagram, as received, carries the seed of its nonce.
const seedOffset = 0x40

// The IV mask of each kind, from gt-telem 1.2.1's PDEncyption (_IV_MASK_A, _B and
// _TILDE). The PS5 encrypts each kind with its own; a datagram decrypted with
// another kind's mask fails the magic check.
const (
	maskA     uint32 = 0xDEADBEAF
	maskB     uint32 = 0xDEADBEEF
	maskTilde uint32 = 0x55FABB4F
)

// key is the first 32 bytes of "Simulator Interface Packet GT7 ver 0.0", gt-telem
// 1.2.1's _GT7_KEY[:32]. GT Sport's key, which gt-telem also has, is out of scope.
var key = [32]byte([]byte("Simulator Interface Packet GT7 ver 0.0")[:32])

// Errors Decrypt returns, wrapped with the detail; compare them with errors.Is.
var (
	// ErrLength is a datagram whose length is not one of the known kinds'. The
	// 368-byte "C" packet is among them until a capture confirms it.
	ErrLength = errors.New("salsa: not a GT7 packet length")
	// ErrMagic is a datagram that does not decrypt to the GT7 magic.
	ErrMagic = errors.New("salsa: no GT7 magic after decryption")
)

// Plain is a decrypted datagram.
type Plain struct {
	// Kind is the packet's layout, from its length.
	Kind Kind
	// Bytes is the plaintext, as long as the datagram, starting with Magic. Bytes
	// 0x40 to 0x43 decrypt to the seed XORed with the keystream, as in gt-telem:
	// the seed itself is the datagram's.
	Bytes []byte
}

// String names the kind as its heartbeat does: "A", "B" or "~".
func (k Kind) String() string {
	switch k {
	case KindA:
		return string(HeartbeatA)
	case KindB:
		return string(HeartbeatB)
	case KindTilde:
		return string(HeartbeatTilde)
	default:
		return fmt.Sprintf("Kind(%d)", uint8(k))
	}
}

// Len returns the datagram length of the kind, or 0 for a Kind that is none of them.
func (k Kind) Len() int {
	switch k {
	case KindA:
		return LenA
	case KindB:
		return LenB
	case KindTilde:
		return LenTilde
	default:
		return 0
	}
}

// mask returns the IV mask of the kind.
func (k Kind) mask() uint32 {
	switch k {
	case KindB:
		return maskB
	case KindTilde:
		return maskTilde
	default:
		return maskA
	}
}

// KindOf returns the kind of a datagram of length n, or an error wrapping ErrLength.
func KindOf(n int) (Kind, error) {
	switch n {
	case LenA:
		return KindA, nil
	case LenB:
		return KindB, nil
	case LenTilde:
		return KindTilde, nil
	default:
		return 0, fmt.Errorf("%w: %d bytes (A is %d, B %d, \"~\" %d)", ErrLength, n, LenA, LenB, LenTilde)
	}
}

// Decrypt decrypts a GT7 datagram as received: Salsa20 with GT7's key and an 8-byte
// nonce, uint32le(seed ^ mask) then uint32le(seed), where seed is the little-endian
// uint32 at 0x40 of the datagram and mask is its kind's. The kind comes from the
// length. It returns an error wrapping ErrLength for any other length and ErrMagic
// when the plaintext does not start with Magic. The datagram is not changed, and the
// plaintext is the only allocation.
func Decrypt(datagram []byte) (Plain, error) {
	kind, err := KindOf(len(datagram))
	if err != nil {
		return Plain{}, err
	}
	out := make([]byte, len(datagram))
	xor(out, datagram, binary.LittleEndian.Uint32(datagram[seedOffset:]), kind)
	if got := binary.LittleEndian.Uint32(out); got != Magic {
		return Plain{}, fmt.Errorf("%w: %s packet starts %#08x, want %#08x", ErrMagic, kind, got, Magic)
	}
	return Plain{Kind: kind, Bytes: out}, nil
}

// xor runs the keystream of kind and seed over in into out. The keystream is its own
// inverse, so tests encrypt with it too.
func xor(out, in []byte, seed uint32, kind Kind) {
	var nonce [8]byte
	binary.LittleEndian.PutUint32(nonce[0:], seed^kind.mask())
	binary.LittleEndian.PutUint32(nonce[4:], seed)
	salsa20.XORKeyStream(out, in, nonce[:], &key)
}
