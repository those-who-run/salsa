package salsatest_test

import (
	"encoding/binary"
	"reflect"
	"sync"
	"testing"

	"github.com/those-who-run/salsa"
	"github.com/those-who-run/salsa/salsatest"
)

func tildePacket() salsa.Packet {
	a := salsa.A{Speed: 50, CurrentLap: 2, TotalLaps: 5, Flags: salsa.FlagTCS, CarCode: 683, IV: 0x01020304}
	return salsa.Packet{
		A:     a,
		Seed:  0x0BADF00D,
		B:     &salsa.B{Heave: -2},
		Tilde: &salsa.Tilde{TildeByte1: 7, TildeVector: [4]float32{1, -1, 2, -2}, EnergyRecovery: 3, TildeFloat: -4},
	}
}

// Encode makes a datagram of the packet's kind with its seed at 0x40, which Decrypt
// and Parse turn back into the packet, Seed included, IV aside.
func TestEncode(t *testing.T) {
	tilde := tildePacket()
	for _, p := range []salsa.Packet{
		{A: tilde.A, Seed: 0x89ABCDEF},
		{A: tilde.A, Seed: 0x89ABCDEF, B: &salsa.B{Sway: 1.5}},
		tilde,
	} {
		datagram := salsatest.Encode(p)
		if len(datagram) != p.Kind().Len() {
			t.Errorf("%s: %d bytes", p.Kind(), len(datagram))
		}
		if seed := binary.LittleEndian.Uint32(datagram[0x40:]); seed != p.Seed {
			t.Errorf("%s: seed %#x, want %#x", p.Kind(), seed, p.Seed)
		}
		plain, err := salsa.Decrypt(datagram)
		if err != nil {
			t.Fatalf("%s: %v", p.Kind(), err)
		}
		got, err := salsa.Parse(plain)
		if err != nil {
			t.Fatalf("%s: %v", p.Kind(), err)
		}
		if got.Seed != p.Seed {
			t.Errorf("%s: Seed %#x, want %#x", p.Kind(), got.Seed, p.Seed)
		}
		got.IV = p.IV // the seed XOR the keystream, which Encode can't choose
		if !reflect.DeepEqual(got, p) {
			t.Errorf("%s: got %+v, want %+v", p.Kind(), got, p)
		}
	}
}

// Plain and Encode only read their Packet, so goroutines can share one; under -race
// (make check), a store into p.B or p.Tilde would fail this test.
func TestSharedPacket(t *testing.T) {
	p := tildePacket()
	want := tildePacket()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for range 50 {
				if i%2 == 0 {
					_ = p.Plain()
				} else {
					_ = salsatest.Encode(p)
				}
				_ = p.Tilde.EnergyRecovery + p.B.Heave
			}
		})
	}
	wg.Wait()
	if !reflect.DeepEqual(p, want) {
		t.Errorf("the shared packet changed: %+v", p)
	}
}
