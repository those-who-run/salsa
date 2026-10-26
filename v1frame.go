package salsa

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
)

// A v1 frame is one line of a v1 recording: a RaceState as v1's RaceState.to_dict
// writes it (src/those_who_run/state.py at v1.0.0), which RaceState.from_telemetry
// built from gt-telem's reading of an A packet. parseV1Frame inverts from_telemetry,
// field for field (those-who-run/paddock#32, R3).

// v1Phases are v1's Phase values. A paused frame sets flags bit 1 and a loading
// frame bit 2, as derive_phase read them; the others set neither, and say nothing
// the lap counters don't (R4).
var v1Phases = map[string]Flags{
	"menu": 0, "loading": FlagLoading, "at_track": 0, "in_race": 0, "paused": FlagPaused, "finished": 0,
}

// v1Key is a key of a v1 frame and how it goes into a packet. required is true for
// the fields RaceState has no default for, which every version wrote.
type v1Key struct {
	name     string
	required bool
	read     func(f *v1Fields, raw json.RawMessage) error
}

// v1Fields is a frame being read: the packet, and the parts of it that come from
// more than one key.
type v1Fields struct {
	r         Received
	current   uint8
	suggested uint8
}

func f32Key(field func(p *Packet) *float32) func(*v1Fields, json.RawMessage) error {
	return func(f *v1Fields, raw json.RawMessage) error { return readF32(raw, field(&f.r.Packet)) }
}

func wheelsKey(field func(p *Packet) *Wheels) func(*v1Fields, json.RawMessage) error {
	return func(f *v1Fields, raw json.RawMessage) error {
		var w []float64
		if err := json.Unmarshal(raw, &w); err != nil {
			return err
		}
		if len(w) != 4 {
			return fmt.Errorf("%d values, not one for each of 4 wheels", len(w))
		}
		out := field(&f.r.Packet)
		for i, dst := range []*float32{&out.FL, &out.FR, &out.RL, &out.RR} {
			v, err := narrow(w[i])
			if err != nil {
				return err
			}
			*dst = v
		}
		return nil
	}
}

func i16Key(field func(p *Packet) *int16) func(*v1Fields, json.RawMessage) error {
	return func(f *v1Fields, raw json.RawMessage) error {
		v, err := readInt(raw, math.MinInt16, math.MaxInt16)
		*field(&f.r.Packet) = int16(v) //nolint:gosec // in range, readInt checked
		return err
	}
}

func i32Key(field func(p *Packet) *int32) func(*v1Fields, json.RawMessage) error {
	return func(f *v1Fields, raw json.RawMessage) error {
		v, err := readInt(raw, math.MinInt32, math.MaxInt32)
		*field(&f.r.Packet) = int32(v) //nolint:gosec // in range, readInt checked
		return err
	}
}

func u8Key(hi int64, field func(f *v1Fields) *uint8) func(*v1Fields, json.RawMessage) error {
	return func(f *v1Fields, raw json.RawMessage) error {
		v, err := readInt(raw, 0, hi)
		*field(f) = uint8(v) //nolint:gosec // in range, readInt checked
		return err
	}
}

// v1Keys are the keys of RaceState.to_dict, in its order, after the version: the 17
// v1 always wrote, then the full-frame fields of version 2, which v1 read as their
// defaults (zero) when a frame lacks them. Fields v1 never kept stay zero.
var v1Keys = []v1Key{
	{"t", true, func(f *v1Fields, raw json.RawMessage) error { return json.Unmarshal(raw, &f.r.T) }},
	{"packet_id", true, i32Key(func(p *Packet) *int32 { return &p.PacketID })},
	{"phase", true, func(f *v1Fields, raw json.RawMessage) error {
		var phase string
		if err := json.Unmarshal(raw, &phase); err != nil {
			return err
		}
		bits, ok := v1Phases[phase]
		if !ok {
			return fmt.Errorf("%q is not a v1 phase", phase)
		}
		f.r.Flags |= bits
		return nil
	}},
	{"current_lap", true, i16Key(func(p *Packet) *int16 { return &p.CurrentLap })},
	{"total_laps", true, i16Key(func(p *Packet) *int16 { return &p.TotalLaps })},
	{"best_lap_ms", true, i32Key(func(p *Packet) *int32 { return &p.BestLapMs })},
	{"last_lap_ms", true, i32Key(func(p *Packet) *int32 { return &p.LastLapMs })},
	{"fuel_level", true, f32Key(func(p *Packet) *float32 { return &p.FuelLevel })},
	{"fuel_capacity", true, f32Key(func(p *Packet) *float32 { return &p.FuelCapacity })},
	{"speed_kph", true, func(f *v1Fields, raw json.RawMessage) error {
		var kph float64
		if err := json.Unmarshal(raw, &kph); err != nil {
			return err
		}
		v, err := narrow(kph / 3.6) // gt-telem's speed_kph is the packet's m/s × 3.6
		f.r.Speed = v
		return err
	}},
	{"engine_rpm", true, f32Key(func(p *Packet) *float32 { return &p.EngineRPM })},
	{"tire_temp", true, wheelsKey(func(p *Packet) *Wheels { return &p.TyreTemp })},
	{"water_temp", true, f32Key(func(p *Packet) *float32 { return &p.WaterTemp })},
	{"oil_temp", true, f32Key(func(p *Packet) *float32 { return &p.OilTemp })},
	// v1 kept a track id it worked out itself (null in most frames), not a field of
	// the packet: it is read past. Track detection is Prost's.
	{"track_id", true, func(*v1Fields, json.RawMessage) error { return nil }},
	{"race_start_pos", true, i16Key(func(p *Packet) *int16 { return &p.RacePosition })},
	{"total_cars", true, i16Key(func(p *Packet) *int16 { return &p.TotalCars })},
	{"cars_on_track", false, func(f *v1Fields, raw json.RawMessage) error {
		var on bool
		if err := json.Unmarshal(raw, &on); err != nil {
			return err
		}
		if on {
			f.r.Flags |= FlagCarsOnTrack
		}
		return nil
	}},
	{"pos_x", false, f32Key(func(p *Packet) *float32 { return &p.Position.X })},
	{"pos_y", false, f32Key(func(p *Packet) *float32 { return &p.Position.Y })},
	{"pos_z", false, f32Key(func(p *Packet) *float32 { return &p.Position.Z })},
	{"vel_x", false, f32Key(func(p *Packet) *float32 { return &p.Velocity.X })},
	{"vel_y", false, f32Key(func(p *Packet) *float32 { return &p.Velocity.Y })},
	{"vel_z", false, f32Key(func(p *Packet) *float32 { return &p.Velocity.Z })},
	{"rot_x", false, f32Key(func(p *Packet) *float32 { return &p.Rotation.X })},
	{"rot_y", false, f32Key(func(p *Packet) *float32 { return &p.Rotation.Y })},
	{"rot_z", false, f32Key(func(p *Packet) *float32 { return &p.Rotation.Z })},
	{"orientation", false, f32Key(func(p *Packet) *float32 { return &p.Orientation })},
	{"ang_vel_x", false, f32Key(func(p *Packet) *float32 { return &p.AngularVelocity.X })},
	{"ang_vel_y", false, f32Key(func(p *Packet) *float32 { return &p.AngularVelocity.Y })},
	{"ang_vel_z", false, f32Key(func(p *Packet) *float32 { return &p.AngularVelocity.Z })},
	{"throttle", false, u8Key(math.MaxUint8, func(f *v1Fields) *uint8 { return &f.r.Throttle })},
	{"brake", false, u8Key(math.MaxUint8, func(f *v1Fields) *uint8 { return &f.r.Brake })},
	// The gears byte's two nibbles, as gt-telem split it.
	{"current_gear", false, u8Key(0x0F, func(f *v1Fields) *uint8 { return &f.current })},
	{"suggested_gear", false, u8Key(0x0F, func(f *v1Fields) *uint8 { return &f.suggested })},
	{"tire_radius", false, wheelsKey(func(p *Packet) *Wheels { return &p.TyreRadius })},
	{"suspension_height", false, wheelsKey(func(p *Packet) *Wheels { return &p.SuspensionHeight })},
	{"wheel_rps", false, wheelsKey(func(p *Packet) *Wheels { return &p.WheelSpeed })},
	{"boost_pressure", false, f32Key(func(p *Packet) *float32 { return &p.Boost })},
	{"oil_pressure", false, f32Key(func(p *Packet) *float32 { return &p.OilPressure })},
	{"body_height", false, f32Key(func(p *Packet) *float32 { return &p.BodyHeight })},
	{"car_code", false, i32Key(func(p *Packet) *int32 { return &p.CarCode })},
	{"time_of_day_ms", false, i32Key(func(p *Packet) *int32 { return &p.TimeOfDayMs })},
}

// parseV1Frame reads one line of a v1 recording into the packet it was built from.
// The version comes first, so a frame of a later version fails on its version,
// whatever keys it has.
func parseV1Frame(line []byte) (Received, error) {
	if len(bytes.TrimSpace(line)) == 0 {
		return Received{}, errors.New("an empty line, not a v1 frame")
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(line, &keys); err != nil {
		return Received{}, fmt.Errorf("not a v1 frame: %w", err)
	}
	if keys == nil {
		return Received{}, errors.New("null, not a v1 frame")
	}
	if raw, ok := keys["v"]; ok {
		// v1's from_dict reads a frame without "v" as version 1, and some
		// version-1 recordings say "v":1.
		v, err := readInt(raw, math.MinInt64, math.MaxInt64)
		if err != nil {
			return Received{}, fmt.Errorf("v: %w", err)
		}
		if v != 1 && v != 2 {
			return Received{}, fmt.Errorf("format version %d, not 1 or 2", v)
		}
		delete(keys, "v")
	}
	f := v1Fields{r: Received{Packet: Packet{fromV1: true}}}
	for _, k := range v1Keys {
		raw, ok := keys[k.name]
		if !ok {
			if k.required {
				return Received{}, fmt.Errorf("no %q, which every v1 frame has", k.name)
			}
			continue
		}
		if k.name != "track_id" && string(raw) == "null" {
			return Received{}, fmt.Errorf("%s: null", k.name)
		}
		if err := k.read(&f, raw); err != nil {
			return Received{}, fmt.Errorf("%s: %w", k.name, err)
		}
		delete(keys, k.name)
	}
	if len(keys) > 0 {
		unknown := make([]string, 0, len(keys))
		for k := range keys {
			unknown = append(unknown, k)
		}
		slices.Sort(unknown)
		return Received{}, fmt.Errorf("%q is not a key of a v1 frame", unknown[0])
	}
	f.r.Gears = MakeGears(f.current, f.suggested)
	return f.r, nil
}

// readF32 reads a JSON number into a float32 field. v1 recorded float32 values
// widened to float64, so they come back exactly; the oldest recordings rounded some
// (82.7 °C), and those come back as the nearest float32.
func readF32(raw json.RawMessage, dst *float32) error {
	var v float64
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	f, err := narrow(v)
	*dst = f
	return err
}

func narrow(v float64) (float32, error) {
	f := float32(v)
	if math.IsInf(float64(f), 0) {
		return 0, fmt.Errorf("%v is out of a float32's range", v)
	}
	return f, nil
}

// readInt reads a JSON integer, which must lie within lo and hi: the range of the
// packet field v1 copied it from.
func readInt(raw json.RawMessage, lo, hi int64) (int64, error) {
	var v int64
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, err
	}
	if v < lo || v > hi {
		return 0, fmt.Errorf("%d is out of the field's range, %d to %d", v, lo, hi)
	}
	return v, nil
}
