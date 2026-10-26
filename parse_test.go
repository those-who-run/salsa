package salsa_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/those-who-run/salsa"
)

var kinds = []salsa.Kind{salsa.KindA, salsa.KindB, salsa.KindTilde}

func readFile(t testing.TB, elem ...string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(elem...)) //nolint:gosec // a fixture under testdata
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// gtTelem maps each field of gt-telem 1.2.1's Telemetry, as parse_golden.py writes
// it, to the Packet field it is. Integers come back as int64 and floats as float64
// (exactly: a float32 widens without rounding), the way gt-telem holds them.
var gtTelem = map[string]func(p salsa.Packet) any{
	"position_x":       func(p salsa.Packet) any { return p.Position.X },
	"position_y":       func(p salsa.Packet) any { return p.Position.Y },
	"position_z":       func(p salsa.Packet) any { return p.Position.Z },
	"velocity_x":       func(p salsa.Packet) any { return p.Velocity.X },
	"velocity_y":       func(p salsa.Packet) any { return p.Velocity.Y },
	"velocity_z":       func(p salsa.Packet) any { return p.Velocity.Z },
	"rotation_x":       func(p salsa.Packet) any { return p.Rotation.X },
	"rotation_y":       func(p salsa.Packet) any { return p.Rotation.Y },
	"rotation_z":       func(p salsa.Packet) any { return p.Rotation.Z },
	"orientation":      func(p salsa.Packet) any { return p.Orientation },
	"ang_vel_x":        func(p salsa.Packet) any { return p.AngularVelocity.X },
	"ang_vel_y":        func(p salsa.Packet) any { return p.AngularVelocity.Y },
	"ang_vel_z":        func(p salsa.Packet) any { return p.AngularVelocity.Z },
	"body_height":      func(p salsa.Packet) any { return p.BodyHeight },
	"engine_rpm":       func(p salsa.Packet) any { return p.EngineRPM },
	"iv":               func(p salsa.Packet) any { return math.Float32frombits(p.IV) },
	"fuel_level":       func(p salsa.Packet) any { return p.FuelLevel },
	"fuel_capacity":    func(p salsa.Packet) any { return p.FuelCapacity },
	"speed_mps":        func(p salsa.Packet) any { return p.Speed },
	"boost_pressure":   func(p salsa.Packet) any { return p.Boost },
	"oil_pressure":     func(p salsa.Packet) any { return p.OilPressure },
	"water_temp":       func(p salsa.Packet) any { return p.WaterTemp },
	"oil_temp":         func(p salsa.Packet) any { return p.OilTemp },
	"tire_fl_temp":     func(p salsa.Packet) any { return p.TyreTemp.FL },
	"tire_fr_temp":     func(p salsa.Packet) any { return p.TyreTemp.FR },
	"tire_rl_temp":     func(p salsa.Packet) any { return p.TyreTemp.RL },
	"tire_rr_temp":     func(p salsa.Packet) any { return p.TyreTemp.RR },
	"packet_id":        func(p salsa.Packet) any { return p.PacketID },
	"current_lap":      func(p salsa.Packet) any { return p.CurrentLap },
	"total_laps":       func(p salsa.Packet) any { return p.TotalLaps },
	"best_lap_time_ms": func(p salsa.Packet) any { return p.BestLapMs },
	"last_lap_time_ms": func(p salsa.Packet) any { return p.LastLapMs },
	"time_of_day_ms":   func(p salsa.Packet) any { return p.TimeOfDayMs },
	"race_start_pos":   func(p salsa.Packet) any { return p.RacePosition },
	"total_cars":       func(p salsa.Packet) any { return p.TotalCars },
	"min_alert_rpm":    func(p salsa.Packet) any { return p.MinAlertRPM },
	"max_alert_rpm":    func(p salsa.Packet) any { return p.MaxAlertRPM },
	"calc_max_speed":   func(p salsa.Packet) any { return p.CalcMaxSpeed },
	// gt-telem reads the flags word as a signed int16.
	"flags":              func(p salsa.Packet) any { return int16(p.Flags) }, //nolint:gosec // the word's bits, as gt-telem reads them
	"bits":               func(p salsa.Packet) any { return uint8(p.Gears) },
	"throttle":           func(p salsa.Packet) any { return p.Throttle },
	"brake":              func(p salsa.Packet) any { return p.Brake },
	"empty":              func(p salsa.Packet) any { return p.Byte93 },
	"road_plane_x":       func(p salsa.Packet) any { return p.RoadPlane.X },
	"road_plane_y":       func(p salsa.Packet) any { return p.RoadPlane.Y },
	"road_plane_z":       func(p salsa.Packet) any { return p.RoadPlane.Z },
	"road_plane_dist":    func(p salsa.Packet) any { return p.RoadPlaneDistance },
	"wheel_fl_rps":       func(p salsa.Packet) any { return p.WheelSpeed.FL },
	"wheel_fr_rps":       func(p salsa.Packet) any { return p.WheelSpeed.FR },
	"wheel_rl_rps":       func(p salsa.Packet) any { return p.WheelSpeed.RL },
	"wheel_rr_rps":       func(p salsa.Packet) any { return p.WheelSpeed.RR },
	"tire_fl_radius":     func(p salsa.Packet) any { return p.TyreRadius.FL },
	"tire_fr_radius":     func(p salsa.Packet) any { return p.TyreRadius.FR },
	"tire_rl_radius":     func(p salsa.Packet) any { return p.TyreRadius.RL },
	"tire_rr_radius":     func(p salsa.Packet) any { return p.TyreRadius.RR },
	"tire_fl_sus_height": func(p salsa.Packet) any { return p.SuspensionHeight.FL },
	"tire_fr_sus_height": func(p salsa.Packet) any { return p.SuspensionHeight.FR },
	"tire_rl_sus_height": func(p salsa.Packet) any { return p.SuspensionHeight.RL },
	"tire_rr_sus_height": func(p salsa.Packet) any { return p.SuspensionHeight.RR },
	"unused1":            func(p salsa.Packet) any { return p.Unused[0] },
	"unused2":            func(p salsa.Packet) any { return p.Unused[1] },
	"unused3":            func(p salsa.Packet) any { return p.Unused[2] },
	"unused4":            func(p salsa.Packet) any { return p.Unused[3] },
	"unused5":            func(p salsa.Packet) any { return p.Unused[4] },
	"unused6":            func(p salsa.Packet) any { return p.Unused[5] },
	"unused7":            func(p salsa.Packet) any { return p.Unused[6] },
	"unused8":            func(p salsa.Packet) any { return p.Unused[7] },
	"clutch_pedal":       func(p salsa.Packet) any { return p.ClutchPedal },
	"clutch_engagement":  func(p salsa.Packet) any { return p.ClutchEngagement },
	"trans_rpm":          func(p salsa.Packet) any { return p.RPMAfterClutch },
	"trans_top_speed":    func(p salsa.Packet) any { return p.TransmissionTop },
	"gear1":              func(p salsa.Packet) any { return p.GearRatios[0] },
	"gear2":              func(p salsa.Packet) any { return p.GearRatios[1] },
	"gear3":              func(p salsa.Packet) any { return p.GearRatios[2] },
	"gear4":              func(p salsa.Packet) any { return p.GearRatios[3] },
	"gear5":              func(p salsa.Packet) any { return p.GearRatios[4] },
	"gear6":              func(p salsa.Packet) any { return p.GearRatios[5] },
	"gear7":              func(p salsa.Packet) any { return p.GearRatios[6] },
	"gear8":              func(p salsa.Packet) any { return p.GearRatios[7] },
	"car_code":           func(p salsa.Packet) any { return p.CarCode },
}

// gtTelemB is gt-telem's B block, null in its output for an A packet.
var gtTelemB = map[string]func(b *salsa.B) float32{
	"wheel_rotation_radians": func(b *salsa.B) float32 { return b.WheelRotation },
	"filler_float_fb":        func(b *salsa.B) float32 { return b.FillerFB },
	"sway":                   func(b *salsa.B) float32 { return b.Sway },
	"heave":                  func(b *salsa.B) float32 { return b.Heave },
	"surge":                  func(b *salsa.B) float32 { return b.Surge },
}

// gtTelemTilde is gt-telem's "~" block, which it misreads (it skips B), so it is
// only checked to be null for A and B packets.
var gtTelemTilde = []string{
	"throttle_filtered", "brake_filtered", "unk_tilde_1", "unk_tilde_2",
	"energy_recovery", "unk_tilde_3",
}

// sameAsPython reports whether got, a Packet field, is the value gt-telem wrote:
// floats as the repr of a double, "nan" included, and integers as JSON numbers.
func sameAsPython(t *testing.T, got any, want json.RawMessage) bool {
	t.Helper()
	switch g := got.(type) {
	case float32:
		var s string
		if err := json.Unmarshal(want, &s); err != nil {
			t.Fatalf("a float written as %s, not a repr string", want)
		}
		w, err := strconv.ParseFloat(s, 64)
		if err != nil {
			t.Fatal(err)
		}
		if math.IsNaN(w) {
			return math.IsNaN(float64(g))
		}
		return float64(g) == w
	default:
		var w int64
		if err := json.Unmarshal(want, &w); err != nil {
			t.Fatalf("an integer written as %s", want)
		}
		return reflect.ValueOf(got).Convert(reflect.TypeFor[int64]()).Int() == w
	}
}

// AC1: the golden A and B datagrams parse to what gt-telem 1.2.1 parsed from them,
// field for field (testdata/golden/parse_golden.py).
//
// The golden plaintext is make_golden.py's pattern, (i*31 + length) & 0xFF, which
// repeats every 256 bytes: fields 0x100 apart (Position and the first gear ratios,
// say) hold the same bytes, so this test cannot tell such offsets apart.
// TestParseOffsets is the one that pins absolute offsets.
func TestParseGolden(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind salsa.Kind
	}{{"a", salsa.KindA}, {"b", salsa.KindB}} {
		t.Run(tc.kind.String(), func(t *testing.T) {
			plain, err := salsa.Decrypt(readFile(t, "testdata", "golden", tc.name+".datagram"))
			if err != nil {
				t.Fatal(err)
			}
			p, err := salsa.Parse(plain)
			if err != nil {
				t.Fatal(err)
			}
			var want map[string]json.RawMessage
			if err := json.Unmarshal(readFile(t, "testdata", "golden", tc.name+".gt-telem.json"), &want); err != nil {
				t.Fatal(err)
			}
			seen := 0
			for field, get := range gtTelem {
				w, ok := want[field]
				if !ok {
					t.Errorf("%s: not in gt-telem's output", field)
					continue
				}
				seen++
				if got := get(p); !sameAsPython(t, got, w) {
					t.Errorf("%s = %v, gt-telem says %s", field, got, w)
				}
			}
			for field, get := range gtTelemB {
				seen++
				w := want[field]
				switch {
				case tc.kind == salsa.KindA:
					if string(w) != "null" {
						t.Errorf("%s: gt-telem has %s for an A packet", field, w)
					}
				case p.B == nil:
					t.Fatalf("B packet without a B block")
				case !sameAsPython(t, get(p.B), w):
					t.Errorf("%s = %v, gt-telem says %s", field, get(p.B), w)
				}
			}
			for _, field := range gtTelemTilde {
				seen++
				if string(want[field]) != "null" {
					t.Errorf("%s: gt-telem has %s for a %s packet", field, want[field], tc.kind)
				}
			}
			if seen != len(want) {
				t.Errorf("checked %d of gt-telem's %d fields", seen, len(want))
			}
			if p.Kind() != tc.kind || p.Tilde != nil || (p.B == nil) != (tc.kind == salsa.KindA) {
				t.Errorf("parsed as %s with B %v and Tilde %v", p.Kind(), p.B, p.Tilde)
			}
		})
	}
}

// plainOf returns a plaintext of kind: the magic, then bytes from r.
func plainOf(r *rand.ChaCha8, kind salsa.Kind) salsa.Plain {
	b := make([]byte, kind.Len())
	_, _ = r.Read(b) // ChaCha8.Read never fails
	binary.LittleEndian.PutUint32(b, salsa.Magic)
	return salsa.Plain{Kind: kind, Bytes: b}
}

func newRand(stream byte) *rand.ChaCha8 {
	return rand.NewChaCha8([32]byte{31, stream}) //nolint:gosec // test data, not secrets: repeatable on purpose
}

func putFloats(b []byte, off int, fs ...float32) {
	for i, f := range fs {
		binary.LittleEndian.PutUint32(b[off+4*i:], math.Float32bits(f))
	}
}

// AC3: a 316-byte packet has its five B floats at 0x128 to 0x138, and a 344-byte
// packet its "~" block at 0x13C to 0x154, after the B block.
func TestParseBlocks(t *testing.T) {
	r := newRand(1)
	wantB := salsa.B{WheelRotation: 1.5, FillerFB: -2.25, Sway: 3.125, Heave: -4.5, Surge: 5.75}
	wantTilde := salsa.Tilde{
		TildeByte1: 0x11, TildeByte2: 0x22, TildeByte3: 0x33, TildeByte4: 0x44,
		TildeVector:    [4]float32{6.5, -7.5, 8.5, -9.5},
		EnergyRecovery: 10.25,
		TildeFloat:     -11.75,
	}
	for _, kind := range []salsa.Kind{salsa.KindB, salsa.KindTilde} {
		t.Run(kind.String(), func(t *testing.T) {
			plain := plainOf(r, kind)
			b := plain.Bytes
			putFloats(b, 0x128, 1.5, -2.25, 3.125, -4.5, 5.75)
			if kind == salsa.KindTilde {
				copy(b[0x13C:], []byte{0x11, 0x22, 0x33, 0x44})
				putFloats(b, 0x140, 6.5, -7.5, 8.5, -9.5, 10.25, -11.75)
			}
			p, err := salsa.Parse(plain)
			if err != nil {
				t.Fatal(err)
			}
			if p.Kind() != kind {
				t.Errorf("Kind() = %s, want %s", p.Kind(), kind)
			}
			if p.B == nil || *p.B != wantB {
				t.Errorf("B = %+v, want %+v", p.B, wantB)
			}
			switch {
			case kind == salsa.KindB && p.Tilde != nil:
				t.Errorf("a B packet has a \"~\" block: %+v", p.Tilde)
			case kind == salsa.KindTilde && (p.Tilde == nil || *p.Tilde != wantTilde):
				t.Errorf("Tilde = %+v, want %+v", p.Tilde, wantTilde)
			}
			// The A block ends where B starts: car code is the four bytes before it.
			if want := int32(binary.LittleEndian.Uint32(b[0x124:])); p.CarCode != want { //nolint:gosec // the wire's bits
				t.Errorf("CarCode = %d, want %d", p.CarCode, want)
			}
		})
	}
}

// AC4: an A packet has no B or "~" block, and reports both absent.
func TestParseAbsent(t *testing.T) {
	p, err := salsa.Parse(plainOf(newRand(2), salsa.KindA))
	if err != nil {
		t.Fatal(err)
	}
	if p.B != nil || p.Tilde != nil {
		t.Errorf("an A packet has B %+v and Tilde %+v, want both nil", p.B, p.Tilde)
	}
	if p.Kind() != salsa.KindA {
		t.Errorf("Kind() = %s, want A", p.Kind())
	}
	// A zero B block is still a B block: sent, and zero.
	b := plainOf(newRand(3), salsa.KindB)
	clear(b.Bytes[0x128:])
	p, err = salsa.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if p.B == nil || *p.B != (salsa.B{}) {
		t.Errorf("a zero B block parsed to %+v, want a zero B", p.B)
	}
}

// R1: the offsets of the A block. Every field holds its own offset as its value, so
// no two fields share one and any field read from another's offset is caught. This
// is the test that pins absolute offsets: the golden plaintext (TestParseGolden)
// repeats every 256 bytes and cannot tell 0x04 from 0x104.
func TestParseOffsets(t *testing.T) {
	b := make([]byte, salsa.LenA)
	binary.LittleEndian.PutUint32(b, salsa.Magic)
	le := binary.LittleEndian
	floatsAt := func(from, to int) {
		for off := from; off < to; off += 4 {
			putFloats(b, off, float32(off))
		}
	}
	floatsAt(0x04, 0x40)
	le.PutUint32(b[0x40:], 0x40)
	floatsAt(0x44, 0x70)
	for _, off := range []int{0x70, 0x78, 0x7C, 0x80} {
		le.PutUint32(b[off:], uint32(off)) //nolint:gosec // small
	}
	for _, off := range []int{0x74, 0x76, 0x84, 0x86, 0x88, 0x8A, 0x8C, 0x8E} {
		le.PutUint16(b[off:], uint16(off)) //nolint:gosec // small
	}
	copy(b[0x90:], []byte{0x90, 0x91, 0x92, 0x93})
	floatsAt(0x94, 0xD4)
	for off := 0xD4; off < 0xF4; off += 4 {
		le.PutUint32(b[off:], uint32(off)) //nolint:gosec // small
	}
	floatsAt(0xF4, 0x124)
	le.PutUint32(b[0x124:], 0x124)
	p, err := salsa.Parse(salsa.Plain{Kind: salsa.KindA, Bytes: b})
	if err != nil {
		t.Fatal(err)
	}
	want := salsa.A{
		Position: salsa.Vec3{X: 0x04, Y: 0x08, Z: 0x0C}, Velocity: salsa.Vec3{X: 0x10, Y: 0x14, Z: 0x18},
		Rotation: salsa.Vec3{X: 0x1C, Y: 0x20, Z: 0x24}, Orientation: 0x28,
		AngularVelocity: salsa.Vec3{X: 0x2C, Y: 0x30, Z: 0x34}, BodyHeight: 0x38, EngineRPM: 0x3C,
		IV: 0x40, FuelLevel: 0x44, FuelCapacity: 0x48, Speed: 0x4C, Boost: 0x50,
		OilPressure: 0x54, WaterTemp: 0x58, OilTemp: 0x5C,
		TyreTemp: salsa.Wheels{FL: 0x60, FR: 0x64, RL: 0x68, RR: 0x6C},
		PacketID: 0x70, CurrentLap: 0x74, TotalLaps: 0x76, BestLapMs: 0x78, LastLapMs: 0x7C,
		TimeOfDayMs: 0x80, RacePosition: 0x84, TotalCars: 0x86, MinAlertRPM: 0x88,
		MaxAlertRPM: 0x8A, CalcMaxSpeed: 0x8C, Flags: 0x8E, Gears: 0x90,
		Throttle: 0x91, Brake: 0x92, Byte93: 0x93,
		RoadPlane: salsa.Vec3{X: 0x94, Y: 0x98, Z: 0x9C}, RoadPlaneDistance: 0xA0,
		WheelSpeed:       salsa.Wheels{FL: 0xA4, FR: 0xA8, RL: 0xAC, RR: 0xB0},
		TyreRadius:       salsa.Wheels{FL: 0xB4, FR: 0xB8, RL: 0xBC, RR: 0xC0},
		SuspensionHeight: salsa.Wheels{FL: 0xC4, FR: 0xC8, RL: 0xCC, RR: 0xD0},
		Unused:           [8]int32{0xD4, 0xD8, 0xDC, 0xE0, 0xE4, 0xE8, 0xEC, 0xF0},
		ClutchPedal:      0xF4, ClutchEngagement: 0xF8, RPMAfterClutch: 0xFC, TransmissionTop: 0x100,
		GearRatios: [8]float32{0x104, 0x108, 0x10C, 0x110, 0x114, 0x118, 0x11C, 0x120},
		CarCode:    0x124,
	}
	if p.A != want {
		t.Errorf("A =\n%+v\nwant\n%+v", p.A, want)
	}
	if got := p.SpeedKPH(); math.Abs(got-0x4C*3.6) > 1e-9 {
		t.Errorf("SpeedKPH() = %v, want %v", got, 0x4C*3.6)
	}
}

// Parse refuses what Decrypt would not have given it.
func TestParseErrors(t *testing.T) {
	r := newRand(4)
	for _, tc := range []struct {
		name  string
		plain salsa.Plain
		want  error
	}{
		{"empty", salsa.Plain{}, salsa.ErrLength},
		{"short", salsa.Plain{Kind: salsa.KindA, Bytes: make([]byte, 295)}, salsa.ErrLength},
		{"C", salsa.Plain{Kind: salsa.KindTilde, Bytes: make([]byte, 368)}, salsa.ErrLength},
		{"kind not length", salsa.Plain{Kind: salsa.KindB, Bytes: plainOf(r, salsa.KindA).Bytes}, salsa.ErrLength},
		{"no kind", salsa.Plain{Bytes: plainOf(r, salsa.KindA).Bytes}, salsa.ErrLength},
		{"no magic", salsa.Plain{Kind: salsa.KindA, Bytes: make([]byte, salsa.LenA)}, salsa.ErrMagic},
	} {
		p, err := salsa.Parse(tc.plain)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
		if !reflect.DeepEqual(p, salsa.Packet{}) {
			t.Errorf("%s: a Packet with the error", tc.name)
		}
	}
}

// Packet.Plain gives back the plaintext Parse read, byte for byte, whatever it holds.
func TestPlainInverse(t *testing.T) {
	r := newRand(5)
	for _, kind := range kinds {
		for range 50 {
			plain := plainOf(r, kind)
			p, err := salsa.Parse(plain)
			if err != nil {
				t.Fatal(err)
			}
			plain.Seed = uint32(r.Uint64()) //nolint:gosec // any 32 bits will do
			p, err = salsa.Parse(plain)
			if err != nil || p.Seed != plain.Seed {
				t.Fatalf("Parse: Seed %#x, want %#x (%v)", p.Seed, plain.Seed, err)
			}
			back := p.Plain()
			if back.Kind != kind || !bytes.Equal(back.Bytes, plain.Bytes) || back.Seed != plain.Seed {
				t.Fatalf("%s: Plain() differs from the plaintext parsed", kind)
			}
		}
	}
	// A "~" packet built without a B block is still a "~" packet, zeros in B.
	got := salsa.Packet{Tilde: &salsa.Tilde{EnergyRecovery: 1}}.Plain()
	if got.Kind != salsa.KindTilde || len(got.Bytes) != salsa.LenTilde || !bytes.Equal(got.Bytes[0x128:0x13C], make([]byte, 0x14)) {
		t.Errorf("Plain() of a \"~\" packet without B: %s, %d bytes", got.Kind, len(got.Bytes))
	}
}

// One allocation for the B block and one for "~", none for A.
func TestParseAllocs(t *testing.T) {
	for kind, want := range map[salsa.Kind]float64{salsa.KindA: 0, salsa.KindB: 1, salsa.KindTilde: 2} {
		plain := plainOf(newRand(6), kind)
		got := testing.AllocsPerRun(100, func() {
			if _, err := salsa.Parse(plain); err != nil {
				t.Fatal(err)
			}
		})
		if got != want {
			t.Errorf("Parse(%s) allocates %v times, want %v", kind, got, want)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	for _, kind := range kinds {
		b.Run(kind.String(), func(b *testing.B) {
			plain := plainOf(newRand(7), kind)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := salsa.Parse(plain); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// R8 and AC6: no plaintext of any length panics, and what parses gives its bytes
// back through Packet.Plain.
func FuzzParse(f *testing.F) {
	r := newRand(8)
	for _, kind := range kinds {
		f.Add(plainOf(r, kind).Bytes)
		f.Add(make([]byte, kind.Len()))
		f.Add(make([]byte, kind.Len()-1))
	}
	f.Add([]byte{})
	f.Add(make([]byte, 368))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) >= 4 && len(b)%2 == 0 {
			binary.LittleEndian.PutUint32(b, salsa.Magic) // half the inputs get past the magic
		}
		kind, lengthErr := salsa.KindOf(len(b))
		before := bytes.Clone(b)
		p, err := salsa.Parse(salsa.Plain{Kind: kind, Bytes: b})
		if !bytes.Equal(b, before) {
			t.Fatal("Parse changed its input")
		}
		switch {
		case lengthErr != nil:
			if !errors.Is(err, salsa.ErrLength) {
				t.Fatalf("%d bytes: err = %v, want ErrLength", len(b), err)
			}
		case err != nil:
			if !errors.Is(err, salsa.ErrMagic) {
				t.Fatalf("%d bytes: err = %v, want ErrMagic", len(b), err)
			}
		default:
			if p.Kind() != kind {
				t.Fatalf("%d bytes parsed as %s", len(b), p.Kind())
			}
			if back := p.Plain(); !bytes.Equal(back.Bytes, b) {
				t.Fatal("Plain() differs from the plaintext parsed")
			}
		}
	})
}
