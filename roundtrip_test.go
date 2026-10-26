package salsa_test

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
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

// v1Frame is one line of a v1 recording, a RaceState as v1's to_dict writes it.
type v1Frame map[string]json.RawMessage

func (f v1Frame) float(t *testing.T, key string) float64 {
	t.Helper()
	var v float64
	if err := json.Unmarshal(f[key], &v); err != nil {
		t.Fatalf("%s: %v", key, err)
	}
	return v
}

func (f v1Frame) int(t *testing.T, key string) int64 {
	t.Helper()
	var v int64
	if err := json.Unmarshal(f[key], &v); err != nil {
		t.Fatalf("%s: %v", key, err)
	}
	return v
}

func (f v1Frame) wheels(t *testing.T, key string) [4]float64 {
	t.Helper()
	var v [4]float64
	if err := json.Unmarshal(f[key], &v); err != nil {
		t.Fatalf("%s: %v", key, err)
	}
	return v
}

// f32 narrows a value v1 recorded from a float32 field, which it holds exactly.
func f32(t *testing.T, key string, v float64) float32 {
	t.Helper()
	if float64(float32(v)) != v {
		t.Fatalf("%s = %v is not a float32", key, v)
	}
	return float32(v)
}

func i16(t *testing.T, key string, v int64) int16 {
	t.Helper()
	if v < math.MinInt16 || v > math.MaxInt16 {
		t.Fatalf("%s = %d is not an int16", key, v)
	}
	return int16(v) //nolint:gosec // in range, checked above
}

func i32(t *testing.T, key string, v int64) int32 {
	t.Helper()
	if v < math.MinInt32 || v > math.MaxInt32 {
		t.Fatalf("%s = %d is not an int32", key, v)
	}
	return int32(v) //nolint:gosec // in range, checked above
}

func u8(t *testing.T, key string, v int64) uint8 {
	t.Helper()
	if v < 0 || v > math.MaxUint8 {
		t.Fatalf("%s = %d is not a byte", key, v)
	}
	return uint8(v) //nolint:gosec // in range, checked above
}

func toWheels(t *testing.T, key string, w [4]float64) salsa.Wheels {
	return salsa.Wheels{FL: f32(t, key, w[0]), FR: f32(t, key, w[1]), RL: f32(t, key, w[2]), RR: f32(t, key, w[3])}
}

// The keys of a v1 frame that are not packet fields: the receive time, the track
// (v1 records it as null) and the format version.
var notPacket = map[string]bool{"t": true, "track_id": true, "v": true}

// packetOf builds the A packet a frame was parsed from, inverting v1's
// RaceState.from_telemetry: speed back to m/s, phase and cars_on_track back into
// flags bits 0 to 2, the gears back into their byte. Every other key is a field as
// it is. A key it doesn't know fails the test, so nothing is skipped unseen.
func packetOf(t *testing.T, f v1Frame) salsa.Packet {
	t.Helper()
	var p salsa.Packet
	var current, suggested uint8
	for key := range f {
		switch key {
		case "packet_id":
			p.PacketID = i32(t, key, f.int(t, key))
		case "phase":
			var phase string
			if err := json.Unmarshal(f[key], &phase); err != nil {
				t.Fatal(err)
			}
			switch phase {
			case "paused":
				p.Flags |= salsa.FlagPaused
			case "loading":
				p.Flags |= salsa.FlagLoading
			}
		case "cars_on_track":
			if string(f[key]) == "true" {
				p.Flags |= salsa.FlagCarsOnTrack
			}
		case "current_lap":
			p.CurrentLap = i16(t, key, f.int(t, key))
		case "total_laps":
			p.TotalLaps = i16(t, key, f.int(t, key))
		case "best_lap_ms":
			p.BestLapMs = i32(t, key, f.int(t, key))
		case "last_lap_ms":
			p.LastLapMs = i32(t, key, f.int(t, key))
		case "time_of_day_ms":
			p.TimeOfDayMs = i32(t, key, f.int(t, key))
		case "race_start_pos":
			p.RacePosition = i16(t, key, f.int(t, key))
		case "total_cars":
			p.TotalCars = i16(t, key, f.int(t, key))
		case "car_code":
			p.CarCode = i32(t, key, f.int(t, key))
		case "throttle":
			p.Throttle = u8(t, key, f.int(t, key))
		case "brake":
			p.Brake = u8(t, key, f.int(t, key))
		case "current_gear":
			current = u8(t, key, f.int(t, key))
		case "suggested_gear":
			suggested = u8(t, key, f.int(t, key))
		case "speed_kph":
			p.Speed = float32(f.float(t, key) / 3.6)
		case "tire_temp":
			p.TyreTemp = toWheels(t, key, f.wheels(t, key))
		case "tire_radius":
			p.TyreRadius = toWheels(t, key, f.wheels(t, key))
		case "suspension_height":
			p.SuspensionHeight = toWheels(t, key, f.wheels(t, key))
		case "wheel_rps":
			p.WheelSpeed = toWheels(t, key, f.wheels(t, key))
		default:
			if notPacket[key] {
				continue
			}
			field, ok := v1Floats[key]
			if !ok {
				t.Fatalf("a v1 frame key the test doesn't know: %q", key)
			}
			*field(&p) = f32(t, key, f.float(t, key))
		}
	}
	p.Gears = salsa.MakeGears(current, suggested)
	// Seeds as varied as the frames; Encode writes the seed at 0x40.
	p.Seed = uint32(p.PacketID) * 2654435761 //nolint:gosec // any seed will do
	return p
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

// checkFrame compares a parsed packet with the frame it was built from, key by key,
// the way v1 would have recorded the packet.
func checkFrame(t *testing.T, where string, f v1Frame, p salsa.Packet) {
	t.Helper()
	var phase string
	_ = json.Unmarshal(f["phase"], &phase) // packetOf already read it
	if p.Flags.Paused() != (phase == "paused") || p.Flags.Loading() != (phase == "loading") {
		t.Errorf("%s: phase %s came back paused %v, loading %v", where, phase, p.Flags.Paused(), p.Flags.Loading())
	}
	if got, want := p.Flags.CarsOnTrack(), string(f["cars_on_track"]) == "true"; got != want {
		t.Errorf("%s: cars_on_track %v, want %v", where, got, want)
	}
	kph := f.float(t, "speed_kph")
	if got := p.SpeedKPH(); math.Abs(got-kph) > math.Max(kph, 1)*0x1p-22 {
		t.Errorf("%s: speed %v km/h, want %v", where, got, kph)
	}
	s, ok := p.Gears.Suggested()
	if int64(p.Gears.Current()) != f.int(t, "current_gear") || int64(s) != f.int(t, "suggested_gear") || ok != (s != salsa.NoGear) {
		t.Errorf("%s: gears %d and %d, want %s and %s", where, p.Gears.Current(), s, f["current_gear"], f["suggested_gear"])
	}
	for key, want := range map[string]int64{
		"packet_id": int64(p.PacketID), "current_lap": int64(p.CurrentLap), "total_laps": int64(p.TotalLaps),
		"best_lap_ms": int64(p.BestLapMs), "last_lap_ms": int64(p.LastLapMs), "time_of_day_ms": int64(p.TimeOfDayMs),
		"race_start_pos": int64(p.RacePosition), "total_cars": int64(p.TotalCars), "car_code": int64(p.CarCode),
		"throttle": int64(p.Throttle), "brake": int64(p.Brake),
	} {
		if got := f.int(t, key); got != want {
			t.Errorf("%s: %s = %d, the frame has %d", where, key, want, got)
		}
	}
	for key, w := range map[string]salsa.Wheels{
		"tire_temp": p.TyreTemp, "tire_radius": p.TyreRadius, "suspension_height": p.SuspensionHeight, "wheel_rps": p.WheelSpeed,
	} {
		if got := [4]float64{float64(w.FL), float64(w.FR), float64(w.RL), float64(w.RR)}; got != f.wheels(t, key) {
			t.Errorf("%s: %s = %v, the frame has %v", where, key, got, f.wheels(t, key))
		}
	}
	for key, field := range v1Floats {
		if _, ok := f[key]; !ok {
			continue // older frames have no rot_* or ang_vel_*
		}
		if got := float64(*field(&p)); got != f.float(t, key) {
			t.Errorf("%s: %s = %v, the frame has %v", where, key, got, f.float(t, key))
		}
	}
}

// AC2 (R7): for every frame of every v1 fixture, an A packet built from the frame,
// encrypted, decrypted and parsed gives back every field the frame has.
func TestRoundTripV1(t *testing.T) {
	for _, path := range v1Fixtures(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			file, err := os.Open(path) //nolint:gosec // a fixture under testdata
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = file.Close() }()
			zr, err := gzip.NewReader(file)
			if err != nil {
				t.Fatal(err)
			}
			lines := bufio.NewScanner(zr)
			lines.Buffer(nil, 1<<20)
			n := 0
			for lines.Scan() {
				n++
				where := fmt.Sprintf("line %d", n)
				var f v1Frame
				if err := json.Unmarshal(lines.Bytes(), &f); err != nil {
					t.Fatalf("%s: %v", where, err)
				}
				built := packetOf(t, f)
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
				checkFrame(t, where, f, got)
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
			if err := lines.Err(); err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				t.Fatal("no frames")
			}
		})
	}
}

// The fixtures are the files copied from v1 (testdata/v1/README.md), unchanged.
func TestV1FixtureSums(t *testing.T) {
	sums := map[string]string{}
	for line := range strings.Lines(string(readFile(t, "testdata", "v1", "SHA256SUMS"))) {
		sum, name, ok := strings.Cut(strings.TrimSpace(line), "  ")
		if !ok {
			t.Fatalf("SHA256SUMS line %q", line)
		}
		sums[name] = sum
	}
	fixtures := v1Fixtures(t)
	if len(sums) != len(fixtures) {
		t.Errorf("SHA256SUMS lists %d files, testdata/v1 has %d fixtures", len(sums), len(fixtures))
	}
	for _, path := range fixtures {
		got := sha256.Sum256(readFile(t, path))
		if want := sums[filepath.Base(path)]; hex.EncodeToString(got[:]) != want {
			t.Errorf("%s: sha256 %x, SHA256SUMS says %q", path, got, want)
		}
	}
}
