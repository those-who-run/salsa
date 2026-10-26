package salsa_test

import (
	"testing"

	"github.com/those-who-run/salsa"
)

// The accessors by the bit each reads, as gt-telem 1.2.1 and PDTools number them.
var accessors = []struct {
	name string
	bit  int
	get  func(salsa.Flags) bool
}{
	{"CarsOnTrack", 0, salsa.Flags.CarsOnTrack},
	{"Paused", 1, salsa.Flags.Paused},
	{"Loading", 2, salsa.Flags.Loading},
	{"InGear", 3, salsa.Flags.InGear},
	{"HasTurbo", 4, salsa.Flags.HasTurbo},
	{"Bit5", 5, salsa.Flags.Bit5},
	{"Handbrake", 6, salsa.Flags.Handbrake},
	{"Lights", 7, salsa.Flags.Lights},
	{"HighBeam", 8, salsa.Flags.HighBeam},
	{"LowBeam", 9, salsa.Flags.LowBeam},
	{"ASM", 10, salsa.Flags.ASM},
	{"TCS", 11, salsa.Flags.TCS},
}

// AC5: with each of the 16 bits set alone, the accessor for that bit is true and
// every other false; bits 12 to 15 have none.
func TestFlags(t *testing.T) {
	for bit := range 16 {
		f := salsa.Flags(1 << bit)
		for _, a := range accessors {
			if got, want := a.get(f), a.bit == bit; got != want {
				t.Errorf("bit %d alone: %s() = %v, want %v", bit, a.name, got, want)
			}
		}
	}
	for _, a := range accessors {
		if !a.get(0xFFFF) || a.get(0) {
			t.Errorf("%s() does not follow its bit", a.name)
		}
	}
	consts := []salsa.Flags{
		salsa.FlagCarsOnTrack, salsa.FlagPaused, salsa.FlagLoading, salsa.FlagInGear,
		salsa.FlagHasTurbo, salsa.FlagBit5, salsa.FlagHandbrake, salsa.FlagLights,
		salsa.FlagHighBeam, salsa.FlagLowBeam, salsa.FlagASM, salsa.FlagTCS,
	}
	for i, c := range consts {
		if c != 1<<accessors[i].bit || !accessors[i].get(c) {
			t.Errorf("the constant for %s is %#x", accessors[i].name, uint16(c))
		}
	}
}

// R4: the gears byte, current gear low and suggested high, 15 for none.
func TestGears(t *testing.T) {
	for _, tc := range []struct {
		g         salsa.Gears
		current   uint8
		suggested uint8
		ok        bool
	}{
		{0x00, 0, 0, true},
		{0xF3, 3, 15, false},
		{0x45, 5, 4, true},
		{0x0F, 15, 0, true},
	} {
		if got := tc.g.Current(); got != tc.current {
			t.Errorf("%#02x.Current() = %d, want %d", uint8(tc.g), got, tc.current)
		}
		s, ok := tc.g.Suggested()
		if s != tc.suggested || ok != tc.ok {
			t.Errorf("%#02x.Suggested() = %d, %v; want %d, %v", uint8(tc.g), s, ok, tc.suggested, tc.ok)
		}
		if got := salsa.MakeGears(tc.current, tc.suggested); got != tc.g {
			t.Errorf("MakeGears(%d, %d) = %#02x, want %#02x", tc.current, tc.suggested, uint8(got), uint8(tc.g))
		}
	}
	if got := salsa.MakeGears(3, salsa.NoGear); got != 0xF3 {
		t.Errorf("MakeGears(3, NoGear) = %#02x", uint8(got))
	}
	// Above 15 each keeps its low 4 bits, and neither spills into the other.
	if got := salsa.MakeGears(0x13, 0x24); got != 0x43 {
		t.Errorf("MakeGears(0x13, 0x24) = %#02x, want 0x43", uint8(got))
	}
}
