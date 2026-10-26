package salsatest_test

import (
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/those-who-run/salsa"
	"github.com/those-who-run/salsa/salsatest"
)

// Encode makes a datagram of the packet's kind, seeded with its IV, which Decrypt
// and Parse turn back into the packet, IV aside.
func TestEncode(t *testing.T) {
	a := salsa.A{Speed: 50, CurrentLap: 2, TotalLaps: 5, Flags: salsa.FlagTCS, CarCode: 683, IV: 0x89ABCDEF}
	for _, p := range []salsa.Packet{
		{A: a},
		{A: a, B: &salsa.B{Sway: 1.5}},
		{A: a, B: &salsa.B{Heave: -2}, Tilde: &salsa.Tilde{TildeByte1: 7, EnergyRecovery: 3}},
	} {
		datagram := salsatest.Encode(p)
		if len(datagram) != p.Kind().Len() {
			t.Errorf("%s: %d bytes", p.Kind(), len(datagram))
		}
		if seed := binary.LittleEndian.Uint32(datagram[0x40:]); seed != p.IV {
			t.Errorf("%s: seed %#x, want %#x", p.Kind(), seed, p.IV)
		}
		plain, err := salsa.Decrypt(datagram)
		if err != nil {
			t.Fatalf("%s: %v", p.Kind(), err)
		}
		got, err := salsa.Parse(plain)
		if err != nil {
			t.Fatalf("%s: %v", p.Kind(), err)
		}
		got.IV = p.IV
		if !reflect.DeepEqual(got, p) {
			t.Errorf("%s: got %+v, want %+v", p.Kind(), got, p)
		}
	}
}
