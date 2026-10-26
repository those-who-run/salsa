package salsa

import (
	"encoding/binary"
	"fmt"
)

// Packet is a parsed GT7 packet: the A block every packet has, and the B and "~"
// blocks when the packet is long enough to carry them. Values are as GT7 sends
// them: speeds in m/s, temperatures in °C, lap times in ms. layout.go has the
// offset of every field and where it comes from.
type Packet struct {
	A
	// B is the motion block, set for B and "~" packets and nil for A packets, so
	// "not sent" is not mistaken for zero.
	B *B
	// Tilde is the "~" block, set for "~" packets only.
	Tilde *Tilde
}

// A is the block every packet starts with, 0x00 to 0x128, the whole of an A packet
// bar its magic.
type A struct {
	Position        Vec3    // world position, m
	Velocity        Vec3    // world velocity, m/s
	Rotation        Vec3    // body rotation, three of the four quaternion components PDTools reads
	Orientation     float32 // the fourth, which gt-telem calls orientation
	AngularVelocity Vec3    // rad/s
	BodyHeight      float32 // m
	EngineRPM       float32
	// IV is the plaintext's 4 bytes at 0x40, which gt-telem reads as a float and
	// PDTools skips. They are not the seed: the seed is the datagram's own bytes there,
	// which decrypt to the seed XOR the keystream (Plain.Bytes).
	IV           uint32
	FuelLevel    float32
	FuelCapacity float32
	Speed        float32 // m/s; SpeedKPH converts
	Boost        float32 // turbo boost; PDTools prints (Boost - 1) × 100 as kPa
	OilPressure  float32
	WaterTemp    float32 // °C
	OilTemp      float32 // °C
	TyreTemp     Wheels  // surface temperature, °C

	PacketID    int32
	CurrentLap  int16 // -1 in menus, 0 before the start
	TotalLaps   int16 // 0 in timed races and practice
	BestLapMs   int32 // -1 when there is none
	LastLapMs   int32 // -1 when there is none
	TimeOfDayMs int32
	// RacePosition is the car's place, which gt-telem calls race_start_pos and PDTools
	// PreRaceStartPositionOrQualiPos; it changes during the race. -1 outside one.
	RacePosition int16
	TotalCars    int16
	MinAlertRPM  int16 // the shift lights' band
	MaxAlertRPM  int16
	CalcMaxSpeed int16 // km/h, as PDTools prints it
	Flags        Flags
	Gears        Gears
	Throttle     uint8 // 0 to 255
	Brake        uint8 // 0 to 255
	Byte93       uint8 // PDTools' Empty_0x93, gt-telem's empty

	RoadPlane         Vec3 // the normal of the road under the car
	RoadPlaneDistance float32
	WheelSpeed        Wheels // rotation speed, rad/s (PDTools' RevPerSecond, gt-telem's rps)
	TyreRadius        Wheels // m
	SuspensionHeight  Wheels // m
	Unused            [8]int32
	ClutchPedal       float32
	ClutchEngagement  float32
	ClutchRPM         float32 // RPM after the clutch, into the gearbox
	TransmissionTop   float32 // transmission top speed, as GT7 sends it
	// GearRatios are gears 1 to 8. GT7 copies the car's ratios here without a bound,
	// PDTools notes, so the eighth is usually 0 and a car with more gears
	// overwrites CarCode.
	GearRatios [8]float32
	CarCode    int32
}

// B is the motion block of B and "~" packets, 0x128 to 0x13C.
type B struct {
	WheelRotation float32 // steering wheel rotation, rad
	// FillerFB is a float PDTools calls FillerFloatFB; nobody names it yet.
	FillerFB float32
	Sway     float32 // lateral acceleration
	Heave    float32 // vertical acceleration
	Surge    float32 // longitudinal acceleration
}

// Tilde is the block "~" packets add after B, 0x13C to 0x158. The sources disagree
// on its first four bytes (PDTools: three unknowns and NoGasConsumption; gt-telem
// and MacManley: throttle and brake filtered, two unknowns) and on the vector
// (MacManley: torque vectoring), so they keep neutral names until a "~" capture
// settles them (those-who-run/paddock#31).
type Tilde struct {
	TildeByte1     uint8
	TildeByte2     uint8
	TildeByte3     uint8
	TildeByte4     uint8
	TildeVector    [4]float32
	EnergyRecovery float32
	TildeFloat     float32 // unknown in every source
}

// Vec3 is a vector of three floats.
type Vec3 struct{ X, Y, Z float32 }

// Wheels is one float for each wheel: front left, front right, rear left, rear right.
type Wheels struct{ FL, FR, RL, RR float32 }

// Flags is the flags word at 0x8E, kept whole; the methods read the bits gt-telem
// and PDTools name. Bits 12 to 15 have no name in either.
type Flags uint16

// The bits of Flags, numbered as in gt-telem 1.2.1's Telemetry and PDTools'
// SimulatorFlags.
const (
	FlagCarsOnTrack Flags = 1 << iota // bit 0
	FlagPaused                        // bit 1
	FlagLoading                       // bit 2
	FlagInGear                        // bit 3
	FlagHasTurbo                      // bit 4
	FlagBit5                          // bit 5, the rev limiter or its shift-light blink
	FlagHandbrake                     // bit 6
	FlagLights                        // bit 7
	FlagHighBeam                      // bit 8
	FlagLowBeam                       // bit 9
	FlagASM                           // bit 10
	FlagTCS                           // bit 11
)

// CarsOnTrack reports bit 0: a race against other cars, not a time trial or practice.
func (f Flags) CarsOnTrack() bool { return f&FlagCarsOnTrack != 0 }

// Paused reports bit 1: the simulation is paused.
func (f Flags) Paused() bool { return f&FlagPaused != 0 }

// Loading reports bit 2: a track or car is loading.
func (f Flags) Loading() bool { return f&FlagLoading != 0 }

// InGear reports bit 3.
func (f Flags) InGear() bool { return f&FlagInGear != 0 }

// HasTurbo reports bit 4: the car has a turbo.
func (f Flags) HasTurbo() bool { return f&FlagHasTurbo != 0 }

// Bit5 reports bit 5: the rev limiter (gt-telem's rev_limit) or its shift-light
// blink (PDTools' RevLimiterBlinkAlertActive). Named neutrally until a drive shows
// which.
func (f Flags) Bit5() bool { return f&FlagBit5 != 0 }

// Handbrake reports bit 6: the handbrake is on.
func (f Flags) Handbrake() bool { return f&FlagHandbrake != 0 }

// Lights reports bit 7: the lights are on.
func (f Flags) Lights() bool { return f&FlagLights != 0 }

// HighBeam reports bit 8.
func (f Flags) HighBeam() bool { return f&FlagHighBeam != 0 }

// LowBeam reports bit 9.
func (f Flags) LowBeam() bool { return f&FlagLowBeam != 0 }

// ASM reports bit 10: active stability management is acting.
func (f Flags) ASM() bool { return f&FlagASM != 0 }

// TCS reports bit 11: traction control is acting.
func (f Flags) TCS() bool { return f&FlagTCS != 0 }

// Gears is the gears byte at 0x90: the current gear in the low 4 bits and the
// suggested gear in the high 4.
type Gears uint8

// NoGear is the suggested gear when there is none.
const NoGear = 15

// MakeGears packs a current and a suggested gear (NoGear for none), 0 to 15 each.
func MakeGears(current, suggested uint8) Gears {
	return Gears(suggested<<4 | current&0x0F)
}

// Current returns the current gear, as GT7 sends it.
func (g Gears) Current() uint8 { return uint8(g) & 0x0F }

// Suggested returns the suggested gear, and false when GT7 suggests none (15).
func (g Gears) Suggested() (uint8, bool) {
	s := uint8(g) >> 4
	return s, s != NoGear
}

// Kind returns the kind of packet p is: "~" with a Tilde block, B with a B block,
// A otherwise.
func (p Packet) Kind() Kind {
	switch {
	case p.Tilde != nil:
		return KindTilde
	case p.B != nil:
		return KindB
	default:
		return KindA
	}
}

// SpeedKPH returns Speed in km/h.
func (p Packet) SpeedKPH() float64 { return float64(p.Speed) * 3.6 }

// Parse reads a plaintext from Decrypt into a Packet: the A block always, the B block
// for B and "~" packets and the "~" block for "~" packets. A Plain whose length is
// not its Kind's is an error wrapping ErrLength, and one without the magic ErrMagic.
// Parse(plain) followed by Packet.Plain gives back plain's bytes exactly.
func Parse(plain Plain) (Packet, error) {
	kind, err := KindOf(len(plain.Bytes))
	if err != nil {
		return Packet{}, err
	}
	if plain.Kind != kind {
		return Packet{}, fmt.Errorf("%w: %d bytes is a %s packet, not %s", ErrLength, len(plain.Bytes), kind, plain.Kind)
	}
	if got := binary.LittleEndian.Uint32(plain.Bytes); got != Magic {
		return Packet{}, fmt.Errorf("%w: plaintext starts %#08x, want %#08x", ErrMagic, got, Magic)
	}
	c := codec{b: plain.Bytes}
	var p Packet
	c.blockA(&p.A)
	if kind != KindA {
		p.B = new(B)
		c.blockB(p.B)
	}
	if kind == KindTilde {
		p.Tilde = new(Tilde)
		c.blockTilde(p.Tilde)
	}
	return p, nil
}

// Plain returns the plaintext of p, the inverse of Parse: the magic, then each block
// p has, as long as its Kind's packet. A "~" packet with a nil B has zeros there.
func (p Packet) Plain() Plain {
	kind := p.Kind()
	c := codec{b: make([]byte, kind.Len()), write: true}
	binary.LittleEndian.PutUint32(c.b, Magic)
	c.blockA(&p.A)
	if kind != KindA {
		b := B{}
		if p.B != nil {
			b = *p.B
		}
		c.blockB(&b)
	}
	if kind == KindTilde {
		c.blockTilde(p.Tilde)
	}
	return Plain{Kind: kind, Bytes: c.b}
}
