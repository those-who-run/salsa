package salsa_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/those-who-run/salsa"
	"github.com/those-who-run/salsa/salsatest"
)

// v1Fixtures returns the v1 recordings copied into testdata/v1/.
func v1Fixtures(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "v1", "*.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no fixtures in testdata/v1")
	}
	return paths
}

// v1Frame is one line of a v1 recording, a RaceState as v1's to_dict writes it,
// read with no help from salsa: the oracle the reader is checked against.
type v1Frame map[string]json.RawMessage

// readV1Frames reads a gzipped v1 recording line by line, as JSON.
func readV1Frames(t *testing.T, path string) []v1Frame {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(readFile(t, path)))
	if err != nil {
		t.Fatal(err)
	}
	var frames []v1Frame
	lines := bufio.NewScanner(zr)
	lines.Buffer(nil, 1<<20)
	for lines.Scan() {
		var f v1Frame
		if err := json.Unmarshal(lines.Bytes(), &f); err != nil {
			t.Fatalf("%s: line %d: %v", path, len(frames)+1, err)
		}
		frames = append(frames, f)
	}
	if err := lines.Err(); err != nil {
		t.Fatal(err)
	}
	return frames
}

// readSource reads a Source to its end.
func readSource(t *testing.T, src salsa.Source) []salsa.Received {
	t.Helper()
	var got []salsa.Received
	for {
		r, err := src.Next(context.Background())
		if errors.Is(err, io.EOF) {
			return got
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
}

// readRecording opens a recording and reads it to its end.
func readRecording(t *testing.T, path string) []salsa.Received {
	t.Helper()
	src, err := salsa.OpenRecording(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := src.Close(); err != nil {
			t.Error(err)
		}
	}()
	return readSource(t, src)
}

func (f v1Frame) has(key string) bool {
	_, ok := f[key]
	return ok
}

// float returns a number of the frame, 0 when the frame lacks it (v1's default).
func (f v1Frame) float(t *testing.T, key string) float64 {
	t.Helper()
	var v float64
	if f.has(key) {
		if err := json.Unmarshal(f[key], &v); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	return v
}

// int returns an integer of the frame, 0 when the frame lacks it (v1's default).
func (f v1Frame) int(t *testing.T, key string) int64 {
	t.Helper()
	var v int64
	if f.has(key) {
		if err := json.Unmarshal(f[key], &v); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	return v
}

// wheels returns four floats of the frame, zeros when the frame lacks them.
func (f v1Frame) wheels(t *testing.T, key string) [4]float64 {
	t.Helper()
	var v [4]float64
	if f.has(key) {
		if err := json.Unmarshal(f[key], &v); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	return v
}

func (f v1Frame) phase(t *testing.T) string {
	t.Helper()
	var phase string
	if err := json.Unmarshal(f["phase"], &phase); err != nil {
		t.Fatal(err)
	}
	return phase
}

// v1Floats are the float fields v1 records under their own names, as they are.
var v1Floats = map[string]func(p *salsa.Packet) *float32{
	"fuel_level":     func(p *salsa.Packet) *float32 { return &p.FuelLevel },
	"fuel_capacity":  func(p *salsa.Packet) *float32 { return &p.FuelCapacity },
	"engine_rpm":     func(p *salsa.Packet) *float32 { return &p.EngineRPM },
	"water_temp":     func(p *salsa.Packet) *float32 { return &p.WaterTemp },
	"oil_temp":       func(p *salsa.Packet) *float32 { return &p.OilTemp },
	"pos_x":          func(p *salsa.Packet) *float32 { return &p.Position.X },
	"pos_y":          func(p *salsa.Packet) *float32 { return &p.Position.Y },
	"pos_z":          func(p *salsa.Packet) *float32 { return &p.Position.Z },
	"vel_x":          func(p *salsa.Packet) *float32 { return &p.Velocity.X },
	"vel_y":          func(p *salsa.Packet) *float32 { return &p.Velocity.Y },
	"vel_z":          func(p *salsa.Packet) *float32 { return &p.Velocity.Z },
	"rot_x":          func(p *salsa.Packet) *float32 { return &p.Rotation.X },
	"rot_y":          func(p *salsa.Packet) *float32 { return &p.Rotation.Y },
	"rot_z":          func(p *salsa.Packet) *float32 { return &p.Rotation.Z },
	"orientation":    func(p *salsa.Packet) *float32 { return &p.Orientation },
	"ang_vel_x":      func(p *salsa.Packet) *float32 { return &p.AngularVelocity.X },
	"ang_vel_y":      func(p *salsa.Packet) *float32 { return &p.AngularVelocity.Y },
	"ang_vel_z":      func(p *salsa.Packet) *float32 { return &p.AngularVelocity.Z },
	"boost_pressure": func(p *salsa.Packet) *float32 { return &p.Boost },
	"oil_pressure":   func(p *salsa.Packet) *float32 { return &p.OilPressure },
	"body_height":    func(p *salsa.Packet) *float32 { return &p.BodyHeight },
}

// v1Ints are the integer fields v1 records, as they are.
var v1Ints = map[string]func(p salsa.Packet) int64{
	"packet_id":      func(p salsa.Packet) int64 { return int64(p.PacketID) },
	"current_lap":    func(p salsa.Packet) int64 { return int64(p.CurrentLap) },
	"total_laps":     func(p salsa.Packet) int64 { return int64(p.TotalLaps) },
	"best_lap_ms":    func(p salsa.Packet) int64 { return int64(p.BestLapMs) },
	"last_lap_ms":    func(p salsa.Packet) int64 { return int64(p.LastLapMs) },
	"time_of_day_ms": func(p salsa.Packet) int64 { return int64(p.TimeOfDayMs) },
	"race_start_pos": func(p salsa.Packet) int64 { return int64(p.RacePosition) },
	"total_cars":     func(p salsa.Packet) int64 { return int64(p.TotalCars) },
	"car_code":       func(p salsa.Packet) int64 { return int64(p.CarCode) },
	"throttle":       func(p salsa.Packet) int64 { return int64(p.Throttle) },
	"brake":          func(p salsa.Packet) int64 { return int64(p.Brake) },
}

// v1Wheels are the four-wheel fields v1 records, as they are.
var v1Wheels = map[string]func(p salsa.Packet) salsa.Wheels{
	"tire_temp":         func(p salsa.Packet) salsa.Wheels { return p.TyreTemp },
	"tire_radius":       func(p salsa.Packet) salsa.Wheels { return p.TyreRadius },
	"suspension_height": func(p salsa.Packet) salsa.Wheels { return p.SuspensionHeight },
	"wheel_rps":         func(p salsa.Packet) salsa.Wheels { return p.WheelSpeed },
}

// The keys checkFrame reads other than those in the tables, and the ones that are
// not packet fields: the track (v1 records it as null) and the format version.
var v1Other = map[string]bool{
	"t": true, "phase": true, "cars_on_track": true, "speed_kph": true,
	"current_gear": true, "suggested_gear": true, "track_id": true, "v": true,
}

// roundedFixture is the one recording that rounded what it recorded: v1's oldest,
// whose floats are not all float32s (82.7 °C) and whose km/h were rounded to two
// places. Every other fixture holds float32s widened to float64, exactly.
const roundedFixture = "real-race-2laps-2026-10-14.jsonl.gz"

// checkFrame compares a packet with the frame v1 recorded, key by key, the way v1
// would have recorded the packet: exactly, float for float and speed included
// (float32(float64(x)*3.6/3.6) is x for every float32 x). For a rounded recording, a
// float comes back as the nearest float32 and speed within 2^-22 of the km/h. A key
// it doesn't know fails the test, so nothing is skipped unseen.
func checkFrame(t *testing.T, where string, f v1Frame, p salsa.Packet, rounded bool) {
	t.Helper()
	for key := range f {
		if v1Floats[key] == nil && v1Ints[key] == nil && v1Wheels[key] == nil && !v1Other[key] {
			t.Fatalf("%s: a v1 frame key the test doesn't know: %q", where, key)
		}
	}
	phase := f.phase(t)
	if p.Flags.Paused() != (phase == "paused") || p.Flags.Loading() != (phase == "loading") {
		t.Errorf("%s: phase %s came back paused %v, loading %v", where, phase, p.Flags.Paused(), p.Flags.Loading())
	}
	if got, want := p.Flags.CarsOnTrack(), string(f["cars_on_track"]) == "true"; got != want {
		t.Errorf("%s: cars_on_track %v, want %v", where, got, want)
	}
	if other := p.Flags &^ (salsa.FlagCarsOnTrack | salsa.FlagPaused | salsa.FlagLoading); other != 0 {
		t.Errorf("%s: flags %#x set that v1 never kept", where, uint16(other))
	}
	kph := f.float(t, "speed_kph")
	if got := p.SpeedKPH(); got != kph && (!rounded || math.Abs(got-kph) > math.Max(kph, 1)*0x1p-22) {
		t.Errorf("%s: speed %v km/h, want %v", where, got, kph)
	}
	s, _ := p.Gears.Suggested()
	if int64(p.Gears.Current()) != f.int(t, "current_gear") || int64(s) != f.int(t, "suggested_gear") {
		t.Errorf("%s: gears %d and %d, want %s and %s", where, p.Gears.Current(), s, f["current_gear"], f["suggested_gear"])
	}
	for key, field := range v1Ints {
		if got, want := field(p), f.int(t, key); got != want {
			t.Errorf("%s: %s = %d, the frame has %d", where, key, got, want)
		}
	}
	for key, field := range v1Wheels {
		w, want := field(p), f.wheels(t, key)
		for i, got := range []float32{w.FL, w.FR, w.RL, w.RR} {
			if !sameFloat(got, want[i], rounded) {
				t.Errorf("%s: %s = %v, the frame has %v", where, key, w, want)
			}
		}
	}
	for key, field := range v1Floats {
		if got, want := *field(&p), f.float(t, key); !sameFloat(got, want, rounded) {
			t.Errorf("%s: %s = %v, the frame has %v", where, key, got, want)
		}
	}
}

// sameFloat reports whether got is what v1 recorded: exactly, or for a rounded
// recording the nearest float32.
func sameFloat(got float32, recorded float64, rounded bool) bool {
	if rounded {
		return got == float32(recorded)
	}
	return float64(got) == recorded
}

// #31's AC2 and #32's AC5: for every frame of every v1 fixture, the packet
// OpenRecording reads holds what the frame does, and encrypted, decrypted and
// parsed, it gives back every field the frame has.
func TestRoundTripV1(t *testing.T) {
	for _, path := range v1Fixtures(t) {
		rounded := filepath.Base(path) == roundedFixture
		t.Run(filepath.Base(path), func(t *testing.T) {
			frames := readV1Frames(t, path)
			read := readRecording(t, path)
			if len(read) != len(frames) {
				t.Fatalf("OpenRecording read %d frames, the file has %d", len(read), len(frames))
			}
			for i, r := range read {
				where := fmt.Sprintf("line %d", i+1)
				if r.T != frames[i].float(t, "t") {
					t.Errorf("%s: T = %v, the frame has %s", where, r.T, frames[i]["t"])
				}
				checkFrame(t, where, frames[i], r.Packet, rounded)
				built := r.Packet
				// Seeds as varied as the frames; Encode writes the seed at 0x40.
				built.Seed = uint32(built.PacketID) * 2654435761 //nolint:gosec // any seed will do
				plain, err := salsa.Decrypt(salsatest.Encode(built))
				if err != nil {
					t.Fatalf("%s: Decrypt: %v", where, err)
				}
				got, err := salsa.Parse(plain)
				if err != nil {
					t.Fatalf("%s: Parse: %v", where, err)
				}
				if got.Kind() != salsa.KindA {
					t.Errorf("%s: came back as a %s packet", where, got.Kind())
				}
				if got.FromV1Recording() {
					t.Errorf("%s: a parsed datagram says it came from a v1 recording", where)
				}
				checkFrame(t, where, frames[i], got, rounded)
				if got.Seed != built.Seed {
					t.Errorf("%s: seed %#x, built with %#x", where, got.Seed, built.Seed)
				}
				// Nothing else changed on the way: the whole A block but the IV,
				// which decrypts to the seed XOR the keystream.
				built.IV, got.IV = 0, 0
				if built.A != got.A {
					t.Errorf("%s: parsed %+v, built %+v", where, got.A, built.A)
				}
			}
		})
	}
}

// The datagram round trip is exact: a datagram decrypted, parsed and encoded again
// is the same bytes, so ecu's replay (those-who-run/paddock#35) can stand a packet
// in for its datagram (the review of salsa#8).
func TestDatagramRoundTrip(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "golden", "*.datagram"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != len(kinds) {
		t.Fatalf("%d golden datagrams, want one for each of the %d kinds", len(paths), len(kinds))
	}
	for _, path := range paths {
		d := readFile(t, path)
		plain, err := salsa.Decrypt(d)
		if err != nil {
			t.Fatal(err)
		}
		p, err := salsa.Parse(plain)
		if err != nil {
			t.Fatal(err)
		}
		if got := salsatest.Encode(p); !bytes.Equal(got, d) {
			t.Errorf("%s: Encode(Parse(Decrypt(d))) differs from d", path)
		}
	}
	// Datagrams of every kind with arbitrary contents and seeds, not only the golden
	// pattern.
	r := newRand(9)
	for _, kind := range kinds {
		for range 50 {
			p, err := salsa.Parse(plainOf(r, kind))
			if err != nil {
				t.Fatal(err)
			}
			p.Seed = uint32(r.Uint64()) //nolint:gosec // any 32 bits will do
			d := salsatest.Encode(p)
			plain, err := salsa.Decrypt(d)
			if err != nil {
				t.Fatal(err)
			}
			back, err := salsa.Parse(plain)
			if err != nil {
				t.Fatal(err)
			}
			if got := salsatest.Encode(back); !bytes.Equal(got, d) {
				t.Fatalf("%s: Encode(Parse(Decrypt(d))) differs from d", kind)
			}
		}
	}
}

func writeTemp(t *testing.T, name string, b []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
