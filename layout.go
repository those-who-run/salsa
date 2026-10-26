package salsa

import (
	"encoding/binary"
	"math"
)

// The packet layouts: the offset of every field, little-endian, from the start of the
// plaintext (the magic is 0x00). One table serves both directions, Parse and
// Packet.Plain, so the two cannot drift.
//
// Sources, which agree on every offset of A and B:
//   - gt-telem 1.2.1, TurismoClient._parse_telemetry (https://github.com/RaceCrewAI/gt-telem),
//     which reads the fields in order after the magic; v1 parses with it. Its names
//     are in the comments. Its "~" branch reads straight after car_code, skipping B,
//     so it is not a source for "~".
//   - PDTools, SimulatorPacket.Read (https://github.com/Nenkai/PDTools,
//     PDTools.SimulatorInterface/SimulatorPacket.cs at b1fee9f), which reads B from
//     0x13C bytes and "~" from 0x158, B first.
//   - MacManley gt7-udp, src/GT7UDPParser.h (https://github.com/MacManley/gt7-udp),
//     whose PacketTilda extends PacketB.

// codec reads b into fields, or writes fields into b when write is set. Its b is at
// least as long as the block it is given; Parse and Plain see to that.
type codec struct {
	b     []byte
	write bool
}

// blockA is the A block, 0x04 to 0x128.
func (c codec) blockA(a *A) {
	c.vec3(0x04, &a.Position)           // position_x, _y, _z; PDTools Position
	c.vec3(0x10, &a.Velocity)           // velocity_x, _y, _z; Velocity
	c.vec3(0x1C, &a.Rotation)           // rotation_x, _y, _z; Rotation (a quaternion's X, Y, Z)
	c.f32(0x28, &a.Orientation)         // orientation; Rotation's W
	c.vec3(0x2C, &a.AngularVelocity)    // ang_vel_x, _y, _z; AngularVelocity
	c.f32(0x38, &a.BodyHeight)          // body_height; BodyHeight
	c.f32(0x3C, &a.EngineRPM)           // engine_rpm; EngineRPM
	c.u32(0x40, &a.IV)                  // iv (read as a float); PDTools skips it
	c.f32(0x44, &a.FuelLevel)           // fuel_level; GasLevel
	c.f32(0x48, &a.FuelCapacity)        // fuel_capacity; GasCapacity
	c.f32(0x4C, &a.Speed)               // speed_mps; MetersPerSecond
	c.f32(0x50, &a.Boost)               // boost_pressure; TurboBoost
	c.f32(0x54, &a.OilPressure)         // oil_pressure; OilPressure
	c.f32(0x58, &a.WaterTemp)           // water_temp; WaterTemperature
	c.f32(0x5C, &a.OilTemp)             // oil_temp; OilTemperature
	c.wheels(0x60, &a.TyreTemp)         // tire_fl_temp .. tire_rr_temp; TireFL_SurfaceTemperature ..
	c.i32(0x70, &a.PacketID)            // packet_id; PacketId
	c.i16(0x74, &a.CurrentLap)          // current_lap; LapCount
	c.i16(0x76, &a.TotalLaps)           // total_laps; LapsInRace
	c.i32(0x78, &a.BestLapMs)           // best_lap_time_ms; BestLapTime
	c.i32(0x7C, &a.LastLapMs)           // last_lap_time_ms; LastLapTime
	c.i32(0x80, &a.TimeOfDayMs)         // time_of_day_ms; TimeOfDayProgression
	c.i16(0x84, &a.RacePosition)        // race_start_pos; PreRaceStartPositionOrQualiPos
	c.i16(0x86, &a.TotalCars)           // total_cars; NumCarsAtPreRace
	c.i16(0x88, &a.MinAlertRPM)         // min_alert_rpm; MinAlertRPM
	c.i16(0x8A, &a.MaxAlertRPM)         // max_alert_rpm; MaxAlertRPM
	c.i16(0x8C, &a.CalcMaxSpeed)        // calc_max_speed; CalculatedMaxSpeed
	c.u16(0x8E, (*uint16)(&a.Flags))    // flags (int16); Flags (SimulatorFlags, a short)
	c.u8(0x90, (*uint8)(&a.Gears))      // bits; CurrentGear and SuggestedGear
	c.u8(0x91, &a.Throttle)             // throttle; Throttle
	c.u8(0x92, &a.Brake)                // brake; Brake
	c.u8(0x93, &a.Byte93)               // empty; Empty_0x93
	c.vec3(0x94, &a.RoadPlane)          // road_plane_x, _y, _z; RoadPlane
	c.f32(0xA0, &a.RoadPlaneDistance)   // road_plane_dist; RoadPlaneDistance
	c.wheels(0xA4, &a.WheelSpeed)       // wheel_fl_rps ..; WheelFL_RevPerSecond ..
	c.wheels(0xB4, &a.TyreRadius)       // tire_fl_radius ..; TireFL_TireRadius ..
	c.wheels(0xC4, &a.SuspensionHeight) // tire_fl_sus_height ..; TireFL_SusHeight ..
	for i := range a.Unused {
		c.i32(0xD4+4*i, &a.Unused[i]) // unused1 .. unused8; PDTools skips 8 ints
	}
	c.f32(0xF4, &a.ClutchPedal)      // clutch_pedal; ClutchPedal
	c.f32(0xF8, &a.ClutchEngagement) // clutch_engagement; ClutchEngagement
	c.f32(0xFC, &a.ClutchRPM)        // trans_rpm; RPMFromClutchToGearbox
	c.f32(0x100, &a.TransmissionTop) // trans_top_speed; TransmissionTopSpeed
	for i := range a.GearRatios {
		c.f32(0x104+4*i, &a.GearRatios[i]) // gear1 .. gear8; GearRatios[0..6], then empty_or_gearRatio8
	}
	c.i32(0x124, &a.CarCode) // car_code; CarCode, the last field: A ends at LenA, 0x128
}

// blockB is the B block, 0x128 to 0x13C.
func (c codec) blockB(b *B) {
	c.f32(0x128, &b.WheelRotation) // PDTools WheelRotation; gt-telem wheel_rotation_radians
	c.f32(0x12C, &b.FillerFB)      // FillerFloatFB; filler_float_fb
	c.f32(0x130, &b.Sway)          // Sway; sway
	c.f32(0x134, &b.Heave)         // Heave; heave
	c.f32(0x138, &b.Surge)         // Surge; surge, the last field: B ends at LenB, 0x13C
}

// blockTilde is the "~" block, 0x13C to 0x158. The names are PDTools' then
// MacManley's; packet.go's Tilde says why they stay neutral.
func (c codec) blockTilde(t *Tilde) {
	c.u8(0x13C, &t.TildeByte1) // Unk1; throttleFiltered
	c.u8(0x13D, &t.TildeByte2) // Unk2; brakeFiltered
	c.u8(0x13E, &t.TildeByte3) // Unk3; UNKNOWNUINT81
	c.u8(0x13F, &t.TildeByte4) // NoGasConsumption; UNKNOWNUINT82
	for i := range t.TildeVector {
		c.f32(0x140+4*i, &t.TildeVector[i]) // Unk5 (a Vector4); torqueVectors[4]
	}
	c.f32(0x150, &t.EnergyRecovery) // EnergyRecovery; energyRecovery
	c.f32(0x154, &t.TildeFloat)     // Unk7; UNKNOWNFLOAT11, the last field: "~" ends at LenTilde, 0x158
}

func (c codec) u8(off int, v *uint8) {
	if c.write {
		c.b[off] = *v
	} else {
		*v = c.b[off]
	}
}

func (c codec) u16(off int, v *uint16) {
	if c.write {
		binary.LittleEndian.PutUint16(c.b[off:], *v)
	} else {
		*v = binary.LittleEndian.Uint16(c.b[off:])
	}
}

func (c codec) i16(off int, v *int16) {
	u := uint16(*v) //nolint:gosec // the int16's bits as they are on the wire
	c.u16(off, &u)
	*v = int16(u) //nolint:gosec // the wire's bits back into an int16
}

func (c codec) u32(off int, v *uint32) {
	if c.write {
		binary.LittleEndian.PutUint32(c.b[off:], *v)
	} else {
		*v = binary.LittleEndian.Uint32(c.b[off:])
	}
}

func (c codec) i32(off int, v *int32) {
	u := uint32(*v) //nolint:gosec // the int32's bits as they are on the wire
	c.u32(off, &u)
	*v = int32(u) //nolint:gosec // the wire's bits back into an int32
}

// f32 keeps a float's bits as they are, NaN payloads included, so Plain gives back
// the bytes Parse read.
func (c codec) f32(off int, v *float32) {
	u := math.Float32bits(*v)
	c.u32(off, &u)
	*v = math.Float32frombits(u)
}

func (c codec) vec3(off int, v *Vec3) {
	c.f32(off, &v.X)
	c.f32(off+4, &v.Y)
	c.f32(off+8, &v.Z)
}

func (c codec) wheels(off int, w *Wheels) {
	c.f32(off, &w.FL)
	c.f32(off+4, &w.FR)
	c.f32(off+8, &w.RL)
	c.f32(off+12, &w.RR)
}
