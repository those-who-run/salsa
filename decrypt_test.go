package salsa

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var kinds = []Kind{KindA, KindB, KindTilde}

// The golden files' base name for each kind (testdata/golden/make_golden.py).
var goldenName = map[Kind]string{KindA: "a", KindB: "b", KindTilde: "tilde"}

// encrypt makes the datagram the PS5 would send for plain, encrypted as kind with
// seed: the keystream over plain, then the seed written at 0x40. It also returns the
// plaintext Decrypt gives back for it: plain, but for the seed's four bytes, which
// decrypt to the seed XOR the keystream.
func encrypt(plain []byte, seed uint32, kind Kind) (datagram, want []byte) {
	datagram = make([]byte, len(plain))
	xor(datagram, plain, seed, kind)
	binary.LittleEndian.PutUint32(datagram[seedOffset:], seed)
	want = bytes.Clone(plain)
	ks := make([]byte, len(plain))
	xor(ks, make([]byte, len(plain)), seed, kind)
	for i := range 4 {
		want[seedOffset+i] = datagram[seedOffset+i] ^ ks[seedOffset+i]
	}
	return datagram, want
}

// newRand returns a seeded source of test bytes, the same on every run.
func newRand(stream byte) *rand.ChaCha8 {
	return rand.NewChaCha8([32]byte{30, stream}) //nolint:gosec // test data, not secrets: repeatable on purpose
}

// randomBytes returns n bytes from r.
func randomBytes(r *rand.ChaCha8, n int) []byte {
	p := make([]byte, n)
	_, _ = r.Read(p) // ChaCha8.Read never fails
	return p
}

// randomSeed returns a seed from r.
func randomSeed(r *rand.ChaCha8) uint32 { return binary.LittleEndian.Uint32(randomBytes(r, 4)) }

// plaintext returns a random plaintext of kind that starts with the magic.
func plaintext(r *rand.ChaCha8, kind Kind) []byte {
	p := randomBytes(r, kind.Len())
	binary.LittleEndian.PutUint32(p, Magic)
	return p
}

func readGolden(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "golden", name)) //nolint:gosec // a fixed fixture name
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// AC1: the golden datagrams decrypt to the plaintexts gt-telem 1.2.1 gave for them.
func TestDecryptGolden(t *testing.T) {
	for _, kind := range kinds {
		t.Run(kind.String(), func(t *testing.T) {
			name := goldenName[kind]
			datagram := readGolden(t, name+".datagram")
			want := readGolden(t, name+".plain")
			got, err := Decrypt(datagram)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if got.Kind != kind {
				t.Errorf("Kind = %s, want %s", got.Kind, kind)
			}
			if !bytes.Equal(got.Bytes, want) {
				t.Errorf("plaintext differs from gt-telem's %s.plain", name)
			}
		})
	}
}

// The golden files are the ones make_golden.py wrote and summed.
func TestGoldenSums(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(string(readGolden(t, "SHA256SUMS"))), "\n")
	if len(lines) != 2*len(kinds) {
		t.Fatalf("SHA256SUMS has %d lines, want %d", len(lines), 2*len(kinds))
	}
	for _, line := range lines {
		sum, name, ok := strings.Cut(line, "  ")
		if !ok {
			t.Fatalf("SHA256SUMS line %q", line)
		}
		got := sha256.Sum256(readGolden(t, name))
		if hex.EncodeToString(got[:]) != sum {
			t.Errorf("%s: sha256 %x, SHA256SUMS says %s", name, got, sum)
		}
	}
}

// Every kind round-trips through the test's encryption, whatever the seed.
func TestDecryptRoundTrip(t *testing.T) {
	r := newRand(1)
	for _, kind := range kinds {
		t.Run(kind.String(), func(t *testing.T) {
			for _, seed := range []uint32{0, 1, 0xFFFFFFFF, maskA, maskB, maskTilde, randomSeed(r), randomSeed(r)} {
				datagram, want := encrypt(plaintext(r, kind), seed, kind)
				got, err := Decrypt(datagram)
				if err != nil {
					t.Fatalf("seed %#x: %v", seed, err)
				}
				if got.Kind != kind || !bytes.Equal(got.Bytes, want) {
					t.Errorf("seed %#x: got %s %x, want %s %x", seed, got.Kind, got.Bytes, kind, want)
				}
				if binary.LittleEndian.Uint32(datagram[seedOffset:]) != seed {
					t.Errorf("seed %#x: Decrypt changed the datagram", seed)
				}
			}
		})
	}
}

// AC2: a datagram decrypts with its own length's mask, and the same bytes encrypted
// under another kind's mask fail the magic check.
func TestDecryptWrongMask(t *testing.T) {
	r := newRand(2)
	for _, kind := range kinds {
		plain := plaintext(r, kind)
		seed := randomSeed(r)
		if _, err := Decrypt(first(encrypt(plain, seed, kind))); err != nil {
			t.Errorf("%s with its own mask: %v", kind, err)
		}
		for _, other := range kinds {
			if other == kind {
				continue
			}
			_, err := Decrypt(first(encrypt(plain, seed, other)))
			if !errors.Is(err, ErrMagic) {
				t.Errorf("%d bytes under %s's mask: err = %v, want ErrMagic", kind.Len(), other, err)
			}
			if errors.Is(err, ErrLength) {
				t.Errorf("%d bytes under %s's mask: also ErrLength", kind.Len(), other)
			}
		}
	}
}

func first(a, _ []byte) []byte { return a }

// AC3: any other length is ErrLength, and random bytes of a good length ErrMagic.
func TestDecryptLength(t *testing.T) {
	for _, n := range []int{0, 1, 4, 0x44, 295, 297, 315, 317, 343, 345, 368, 1500, 1 << 16} {
		got, err := Decrypt(make([]byte, n))
		if !errors.Is(err, ErrLength) || errors.Is(err, ErrMagic) {
			t.Errorf("%d bytes: err = %v, want ErrLength", n, err)
		}
		if got.Kind != 0 || got.Bytes != nil {
			t.Errorf("%d bytes: got %+v with the error, want the zero Plain", n, got)
		}
	}
	if _, err := Decrypt(nil); !errors.Is(err, ErrLength) {
		t.Errorf("nil: err = %v, want ErrLength", err)
	}
	r := newRand(3)
	for _, kind := range kinds {
		random := randomBytes(r, kind.Len())
		got, err := Decrypt(random)
		if !errors.Is(err, ErrMagic) {
			t.Errorf("%d random bytes: err = %v, want ErrMagic", kind.Len(), err)
		}
		if got.Bytes != nil {
			t.Errorf("%d random bytes: plaintext returned with the error", kind.Len())
		}
	}
}

func TestKindOf(t *testing.T) {
	for _, kind := range kinds {
		got, err := KindOf(kind.Len())
		if err != nil || got != kind {
			t.Errorf("KindOf(%d) = %s, %v; want %s", kind.Len(), got, err, kind)
		}
	}
	if _, err := KindOf(368); !errors.Is(err, ErrLength) {
		t.Errorf("KindOf(368): err = %v, want ErrLength", err)
	}
}

func TestKind(t *testing.T) {
	for _, tc := range []struct {
		kind      Kind
		name      string
		length    int
		heartbeat byte
		mask      uint32
	}{
		{KindA, "A", 296, HeartbeatA, 0xDEADBEAF},
		{KindB, "B", 316, HeartbeatB, 0xDEADBEEF},
		{KindTilde, "~", 344, HeartbeatTilde, 0x55FABB4F},
	} {
		if got := tc.kind.String(); got != tc.name || got != string(tc.heartbeat) {
			t.Errorf("%d.String() = %q, want %q", tc.kind, got, tc.name)
		}
		if got := tc.kind.Len(); got != tc.length {
			t.Errorf("%s.Len() = %d, want %d", tc.kind, got, tc.length)
		}
		if got := tc.kind.mask(); got != tc.mask {
			t.Errorf("%s.mask() = %#x, want %#x", tc.kind, got, tc.mask)
		}
	}
	if got := Kind(0).String(); got != "Kind(0)" {
		t.Errorf("Kind(0).String() = %q", got)
	}
	if got := Kind(9).Len(); got != 0 {
		t.Errorf("Kind(9).Len() = %d, want 0", got)
	}
}

// R6: the heartbeat facts the listener and tests share.
func TestHeartbeat(t *testing.T) {
	if HeartbeatA != 'A' || HeartbeatB != 'B' || HeartbeatTilde != '~' {
		t.Errorf("heartbeats %q %q %q", HeartbeatA, HeartbeatB, HeartbeatTilde)
	}
	if HeartbeatPort != 33739 || TelemetryPort != 33740 {
		t.Errorf("ports %d and %d, want 33739 and 33740", HeartbeatPort, TelemetryPort)
	}
	if string(key[:]) != "Simulator Interface Packet GT7 v" {
		t.Errorf("key %q", key)
	}
}

// R5: the plaintext is the only allocation.
func TestDecryptAllocs(t *testing.T) {
	datagram := readGolden(t, "b.datagram")
	allocs := testing.AllocsPerRun(100, func() {
		if _, err := Decrypt(datagram); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 1 {
		t.Errorf("Decrypt allocates %v times per call, want 1", allocs)
	}
}

// AC5: the time per datagram, for each kind.
func BenchmarkDecrypt(b *testing.B) {
	for _, kind := range kinds {
		b.Run(kind.String(), func(b *testing.B) {
			datagram := readGolden(b, goldenName[kind]+".datagram")
			b.ReportAllocs()
			b.SetBytes(int64(len(datagram)))
			for b.Loop() {
				if _, err := Decrypt(datagram); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// R7 and AC4: no input panics, and what decrypts encrypts back to the datagram.
func FuzzDecrypt(f *testing.F) {
	for _, kind := range kinds {
		f.Add(readGolden(f, goldenName[kind]+".datagram"))
		f.Add(make([]byte, kind.Len()))
		f.Add(make([]byte, kind.Len()+1))
	}
	f.Add([]byte{})
	f.Add(make([]byte, 368))
	f.Fuzz(func(t *testing.T, datagram []byte) {
		before := bytes.Clone(datagram)
		got, err := Decrypt(datagram)
		if !bytes.Equal(datagram, before) {
			t.Fatal("Decrypt changed its input")
		}
		_, lengthErr := KindOf(len(datagram))
		switch {
		case lengthErr != nil:
			if !errors.Is(err, ErrLength) {
				t.Fatalf("%d bytes: err = %v, want ErrLength", len(datagram), err)
			}
		case err != nil:
			if !errors.Is(err, ErrMagic) {
				t.Fatalf("%d bytes: err = %v, want ErrMagic", len(datagram), err)
			}
		default:
			if got.Kind.Len() != len(datagram) || binary.LittleEndian.Uint32(got.Bytes) != Magic {
				t.Fatalf("%d bytes: decrypted to %s starting %x", len(datagram), got.Kind, got.Bytes[:4])
			}
			back := make([]byte, len(datagram))
			xor(back, got.Bytes, binary.LittleEndian.Uint32(datagram[seedOffset:]), got.Kind)
			if !bytes.Equal(back, datagram) {
				t.Fatal("the plaintext does not encrypt back to the datagram")
			}
		}
	})
}
