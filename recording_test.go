package salsa_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/those-who-run/salsa"
)

const pitStop = "real-nuc-suzuka-pit-stop-2026-10-15.jsonl.gz"

// null is a JSON null to put in a line; a nil value in edited deletes the key.
var null = json.RawMessage("null")

// AC1: every fixture reads to the end with no error, one frame for each line.
func TestRecordingFixtures(t *testing.T) {
	for _, path := range v1Fixtures(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			lines, err := countLines(readFile(t, path))
			if err != nil {
				t.Fatal(err)
			}
			read := readRecording(t, path)
			if len(read) != lines || lines == 0 {
				t.Errorf("%d frames from %d lines", len(read), lines)
			}
			for i, r := range read {
				if !r.FromV1Recording() || r.Kind() != salsa.KindA {
					t.Fatalf("frame %d: from a v1 recording %v, kind %s", i+1, r.FromV1Recording(), r.Kind())
				}
			}
		})
	}
}

// AC2: for every frame of real-nuc-suzuka-pit-stop-2026-10-15, the packet's lap
// counters, lap times, fuel, position, car code and flags bits 0 to 2 are the
// recorded frame's.
func TestRecordingPitStop(t *testing.T) {
	path := filepath.Join("testdata", "v1", pitStop)
	frames := readV1Frames(t, path)
	read := readRecording(t, path)
	if len(read) != len(frames) || len(frames) != 874 {
		t.Fatalf("%d frames read, %d in the file, want 874", len(read), len(frames))
	}
	for i, r := range read {
		f := frames[i]
		where := fmt.Sprintf("line %d", i+1)
		for key, got := range map[string]int64{
			"current_lap": int64(r.CurrentLap), "total_laps": int64(r.TotalLaps),
			"best_lap_ms": int64(r.BestLapMs), "last_lap_ms": int64(r.LastLapMs),
			"race_start_pos": int64(r.RacePosition), "car_code": int64(r.CarCode),
		} {
			if want := f.int(t, key); got != want {
				t.Errorf("%s: %s = %d, the frame has %d", where, key, got, want)
			}
		}
		for key, got := range map[string]float32{
			"fuel_level": r.FuelLevel, "fuel_capacity": r.FuelCapacity,
			"pos_x": r.Position.X, "pos_y": r.Position.Y, "pos_z": r.Position.Z,
		} {
			if want := f.float(t, key); float64(got) != want {
				t.Errorf("%s: %s = %v, the frame has %v", where, key, got, want)
			}
		}
		phase := f.phase(t)
		if r.Flags.CarsOnTrack() != (string(f["cars_on_track"]) == "true") ||
			r.Flags.Paused() != (phase == "paused") || r.Flags.Loading() != (phase == "loading") {
			t.Errorf("%s: flags %#x for phase %s, cars_on_track %s", where, uint16(r.Flags), phase, f["cars_on_track"])
		}
	}
}

// firstLine returns the first frame of a fixture as a map, to make lines from.
func firstLine(t *testing.T, name string) map[string]any {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(readFile(t, "testdata", "v1", name)))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	line, _, _ := bytes.Cut(b, []byte("\n"))
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func marshal(t *testing.T, m map[string]any) string {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// edited returns a copy of m with each of set's keys set, or deleted when nil.
func edited(m map[string]any, set map[string]any) map[string]any {
	out := maps.Clone(m)
	for k, v := range set {
		if v == nil {
			delete(out, k)
		} else {
			out[k] = v
		}
	}
	return out
}

// readString reads a recording held in a string, to its end or its first error.
func readString(name, s string) ([]salsa.Received, error) {
	src, err := salsa.NewRecording(strings.NewReader(s), name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = src.Close() }()
	var got []salsa.Received
	for {
		r, err := src.Next(context.Background())
		if errors.Is(err, io.EOF) {
			return got, nil
		}
		if err != nil {
			return got, err
		}
		got = append(got, r)
	}
}

// AC3 (R2): a version-1 line (no "v", or "v":1, and no full-frame fields) and a
// version-2 line both read; a "v":3 line fails, naming the file and the line.
func TestRecordingVersions(t *testing.T) {
	v2 := firstLine(t, pitStop)
	v1 := firstLine(t, "real-race-2laps-2026-10-14.jsonl.gz")
	if v2["v"] != 2.0 || v1["v"] != 1.0 || len(v1) != 19 {
		t.Fatalf("the fixtures' first lines are v %v and %v, with %d keys", v2["v"], v1["v"], len(v1))
	}
	noV := marshal(t, edited(v1, map[string]any{"v": nil}))
	got, err := readString("versions.jsonl", noV+"\n"+marshal(t, v1)+"\n"+marshal(t, v2)+"\n")
	if err != nil || len(got) != 3 {
		t.Fatalf("read %d frames, err %v", len(got), err)
	}
	if got[0].Packet != got[1].Packet {
		t.Errorf("no v and \"v\":1 read differently: %+v, %+v", got[0], got[1])
	}
	// A version-1 frame has no full-frame fields: they are zero, as v1 read them.
	if got[0].CarCode != 0 || got[0].Throttle != 0 || got[0].Gears != 0 || got[0].Position != (salsa.Vec3{}) {
		t.Errorf("version 1: full-frame fields %+v", got[0].A)
	}
	if got[0].T != v1["t"] || got[0].PacketID != 439618 || got[0].LastLapMs != 49672 || got[0].TyreTemp.FR != 82.5 {
		t.Errorf("version 1: %+v", got[0])
	}
	if got[2].CarCode != 683 || got[2].PacketID != int32(v2["packet_id"].(float64)) {
		t.Errorf("version 2: %+v", got[2])
	}

	for _, v := range []any{3, 0, -1, "2", 2.5} {
		line := marshal(t, edited(v2, map[string]any{"v": v}))
		got, err := readString("v3.jsonl", marshal(t, v2)+"\n"+line+"\n")
		var re *salsa.RecordingError
		if !errors.As(err, &re) || re.Name != "v3.jsonl" || re.Line != 2 || len(got) != 1 {
			t.Fatalf("v %v: %d frames, err %v", v, len(got), err)
		}
		if !strings.HasPrefix(err.Error(), "v3.jsonl:2: ") {
			t.Errorf("v %v: error %q names no file and line", v, err)
		}
	}
	// A null version says so, rather than naming a version the line doesn't have.
	_, err = readString("vnull.jsonl", marshal(t, edited(v2, map[string]any{"v": null}))+"\n")
	if err == nil || err.Error() != "vnull.jsonl:1: v: null, not a format version" {
		t.Errorf("v null: %v", err)
	}
	// A later version fails on its version, whatever keys it has.
	_, err = readString("v3.jsonl", marshal(t, edited(v2, map[string]any{"v": 3, "new_field": 1}))+"\n")
	if err == nil || !strings.Contains(err.Error(), "format version 3") {
		t.Errorf("v 3 with a new key: %v", err)
	}
}

// R4: a frame whose phase disagrees with its lap counters reads as v1's from_dict
// reads it, the lap counters winning: the packet carries the counters, and only a
// paused or loading phase sets a flag. A phase is otherwise only these bits.
func TestRecordingPhase(t *testing.T) {
	v2 := firstLine(t, pitStop)
	for _, tc := range []struct {
		phase string
		lap   int
		flags salsa.Flags
	}{
		{"in_race", 0, 0}, // before the lap-count rules: v1 reads it as at_track
		{"at_track", 3, 0},
		{"finished", 2, 0},
		{"menu", -1, 0},
		{"paused", 2, salsa.FlagPaused},
		{"loading", 0, salsa.FlagLoading},
	} {
		got, err := readString("phase.jsonl", marshal(t, edited(v2, map[string]any{
			"phase": tc.phase, "current_lap": tc.lap, "total_laps": 1, "cars_on_track": false,
		})))
		if err != nil || len(got) != 1 {
			t.Fatalf("%s: %v", tc.phase, err)
		}
		if got[0].Flags != tc.flags || got[0].CurrentLap != int16(tc.lap) || got[0].TotalLaps != 1 { //nolint:gosec // small
			t.Errorf("%s on lap %d: flags %#x, laps %d of %d", tc.phase, tc.lap, uint16(got[0].Flags), got[0].CurrentLap, got[0].TotalLaps)
		}
	}
}

// The rest of R3's mapping that the fixtures can't show: speed to m/s, the gears
// byte with no suggested gear, and cars_on_track.
func TestRecordingFields(t *testing.T) {
	v2 := firstLine(t, pitStop)
	got, err := readString("fields.jsonl", marshal(t, edited(v2, map[string]any{
		"speed_kph": 36.0, "current_gear": 3, "suggested_gear": 15, "cars_on_track": true, "throttle": 255, "phase": "in_race",
	})))
	if err != nil {
		t.Fatal(err)
	}
	p := got[0]
	if s, ok := p.Gears.Suggested(); p.Speed != 10 || p.Gears.Current() != 3 || ok || s != salsa.NoGear || p.Throttle != 255 {
		t.Errorf("speed %v m/s, gears %d and %d, throttle %d", p.Speed, p.Gears.Current(), s, p.Throttle)
	}
	if p.Flags != salsa.FlagCarsOnTrack {
		t.Errorf("flags %#x, want cars on track alone", uint16(p.Flags))
	}
	// What v1 never kept is zero; the IV and seed among it.
	if p.IV != 0 || p.Seed != 0 || p.MinAlertRPM != 0 || p.ClutchPedal != 0 || p.GearRatios != [8]float32{} || p.B != nil || p.Tilde != nil {
		t.Errorf("fields v1 never kept: %+v", p)
	}
}

// R5: a line that is not a v1 frame is an error naming the file and the line, and
// every later Next returns it again.
func TestRecordingMalformed(t *testing.T) {
	v2 := firstLine(t, pitStop)
	good := marshal(t, v2)
	for name, line := range map[string]string{
		"not JSON":               `{"t":`,
		"an empty line":          "",
		"spaces":                 "   ",
		"null":                   "null",
		"an array":               "[]",
		"a key v1 never had":     marshal(t, edited(v2, map[string]any{"rpm_after_clutch": 1})),
		"a required key gone":    marshal(t, edited(v2, map[string]any{"fuel_level": nil})),
		"track_id gone":          marshal(t, edited(v2, map[string]any{"track_id": nil})),
		"a null":                 marshal(t, edited(v2, map[string]any{"engine_rpm": null})),
		"a null optional":        marshal(t, edited(v2, map[string]any{"throttle": null})),
		"a phase v1 never had":   marshal(t, edited(v2, map[string]any{"phase": "racing"})),
		"a phase not a string":   marshal(t, edited(v2, map[string]any{"phase": 1})),
		"a lap past int16":       marshal(t, edited(v2, map[string]any{"current_lap": 40000})),
		"a fraction of a lap":    marshal(t, edited(v2, map[string]any{"current_lap": 1.5})),
		"a packet id past int32": marshal(t, edited(v2, map[string]any{"packet_id": 1 << 31})),
		"a gear past 15":         marshal(t, edited(v2, map[string]any{"current_gear": 16})),
		"a negative throttle":    marshal(t, edited(v2, map[string]any{"throttle": -1})),
		"three wheels":           marshal(t, edited(v2, map[string]any{"tire_temp": []float64{1, 2, 3}})),
		"five wheels":            marshal(t, edited(v2, map[string]any{"tire_temp": []float64{1, 2, 3, 4, 5}})),
		"a null wheel":           marshal(t, edited(v2, map[string]any{"tire_temp": []any{91.5, null, 90.5, 82.7}})),
		"wheels not an array":    marshal(t, edited(v2, map[string]any{"suspension_height": 0.2})),
		"wheels not numbers":     marshal(t, edited(v2, map[string]any{"wheel_rps": []string{"a", "b", "c", "d"}})),
		"a wheel past float32":   marshal(t, edited(v2, map[string]any{"tire_radius": []float64{1e39, 0, 0, 0}})),
		"a float past float32":   marshal(t, edited(v2, map[string]any{"oil_temp": 1e39})),
		"a string for a float":   marshal(t, edited(v2, map[string]any{"oil_temp": "hot"})),
		"a speed past float32":   marshal(t, edited(v2, map[string]any{"speed_kph": 1e300})),
		"a speed not a number":   marshal(t, edited(v2, map[string]any{"speed_kph": "fast"})),
		"t not a number":         marshal(t, edited(v2, map[string]any{"t": "now"})),
		"cars_on_track a number": marshal(t, edited(v2, map[string]any{"cars_on_track": 1})),
		"v null":                 marshal(t, edited(v2, map[string]any{"v": null})),
	} {
		t.Run(name, func(t *testing.T) {
			src, err := salsa.NewRecording(strings.NewReader(good+"\n"+good+"\n"+line+"\n"+good+"\n"), "bad.jsonl")
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if _, err := src.Next(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			_, err = src.Next(context.Background())
			var re *salsa.RecordingError
			if !errors.As(err, &re) || re.Line != 3 || !strings.HasPrefix(err.Error(), "bad.jsonl:3: ") {
				t.Fatalf("err %v, want one at bad.jsonl:3", err)
			}
			if _, again := src.Next(context.Background()); again != err { //nolint:errorlint // the same error, not one like it
				t.Errorf("after the error, Next returned %v", again)
			}
			if err := src.Close(); err != nil {
				t.Error(err)
			}
		})
	}
}

// gzipped compresses s as one gzip member.
func gzipped(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A recording cut short, as a crash leaves one, or damaged is an error after the
// frames before it, never a silent end.
func TestRecordingDamaged(t *testing.T) {
	good := marshal(t, firstLine(t, pitStop))
	whole := gzipped(t, strings.Repeat(good+"\n", 200))

	t.Run("gzip truncated", func(t *testing.T) {
		got, err := readString("cut.jsonl.gz", string(whole[:len(whole)-10]))
		var re *salsa.RecordingError
		if !errors.As(err, &re) || !errors.Is(err, io.ErrUnexpectedEOF) || re.Line != len(got)+1 {
			t.Errorf("%d frames, then %v", len(got), err)
		}
	})
	t.Run("gzip corrupt", func(t *testing.T) {
		b := bytes.Clone(whole)
		b[len(b)-6] ^= 0xFF // the CRC-32 in the trailer
		got, err := readString("crc.jsonl.gz", string(b))
		if !errors.Is(err, gzip.ErrChecksum) || len(got) != 200 {
			t.Errorf("%d frames, then %v", len(got), err)
		}
		if !strings.Contains(fmt.Sprint(err), "crc.jsonl.gz:201: damaged after line 200") {
			t.Errorf("error %q", err)
		}
	})
	t.Run("gzip header damaged", func(t *testing.T) {
		_, err := salsa.NewRecording(bytes.NewReader([]byte{0x1f, 0x8b, 0x00}), "head.gz")
		var re *salsa.RecordingError
		if !errors.As(err, &re) || re.Line != 0 || !strings.HasPrefix(err.Error(), "head.gz: ") {
			t.Errorf("err %v", err)
		}
	})
	t.Run("plain cut mid-line", func(t *testing.T) {
		got, err := readString("cut.jsonl", good+"\n"+good[:100])
		var re *salsa.RecordingError
		if !errors.As(err, &re) || re.Line != 2 || len(got) != 1 {
			t.Errorf("%d frames, then %v", len(got), err)
		}
	})
	t.Run("a read error", func(t *testing.T) {
		src, err := salsa.NewRecording(io.MultiReader(strings.NewReader(good+"\n"), iotest.ErrReader(errors.New("disk"))), "disk.jsonl")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := src.Next(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := src.Next(context.Background()); err == nil || !strings.Contains(err.Error(), "disk.jsonl:2: damaged after line 1: disk") {
			t.Errorf("err %v", err)
		}
	})
	t.Run("a read error in padding", func(t *testing.T) {
		disk := errors.New("disk")
		r := io.MultiReader(bytes.NewReader(gzipped(t, good+"\n")), strings.NewReader("\x00\x00\x00"), iotest.ErrReader(disk))
		src, err := salsa.NewRecording(r, "pad.jsonl.gz")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := src.Next(context.Background()); err != nil {
			t.Fatal(err)
		}
		_, err = src.Next(context.Background())
		var re *salsa.RecordingError
		if !errors.Is(err, disk) || !errors.As(err, &re) || re.Line != 2 {
			t.Errorf("a read error after a member and padding: %v, want the disk error at line 2", err)
		}
	})
	t.Run("a read error first", func(t *testing.T) {
		if _, err := salsa.NewRecording(iotest.ErrReader(errors.New("disk")), "disk.jsonl"); err == nil {
			t.Error("no error")
		}
	})
	t.Run("a line too long", func(t *testing.T) {
		got, err := readString("long.jsonl", good+"\n"+strings.Repeat(" ", 1<<20)+good+"\n")
		var re *salsa.RecordingError
		if !errors.As(err, &re) || re.Line != 2 || len(got) != 1 {
			t.Errorf("%d frames, then %v", len(got), err)
		}
	})
}

// How v1 wrote recordings that are still whole: gzip with a member per append
// (RecordingWriter opens with "a"), plain JSON lines, CRLF endings, no newline after
// the last line, a long line, and nothing at all.
func TestRecordingShapes(t *testing.T) {
	good := marshal(t, firstLine(t, pitStop))
	long := strings.Replace(good, "{", "{"+strings.Repeat(" ", 100_000), 1)
	for name, tc := range map[string]struct {
		data   string
		frames int
	}{
		"two gzip members": {string(gzipped(t, good+"\n"+good+"\n")) + string(gzipped(t, good+"\n")), 3},
		"plain":            {good + "\n" + good + "\n", 2},
		"CRLF":             {good + "\r\n" + good + "\r\n", 2},
		"no last newline":  {good + "\n" + good, 2},
		"a long line":      {long + "\n" + good + "\n", 2},
		"empty":            {"", 0},
		"empty gzip":       {string(gzipped(t, "")), 0},
		// Zero bytes after a member are padding, as Python's gzip reads them: at the
		// end, as a power loss can leave a file, and between members.
		"zero padding":                  {string(gzipped(t, good+"\n")) + strings.Repeat("\x00", 4096), 1},
		"padding then a member":         {string(gzipped(t, good+"\n")) + "\x00\x00" + string(gzipped(t, good+"\n")), 2},
		"padding then junk":             {string(gzipped(t, good+"\n")) + "\x00\x00junk", -1},
		"junk after a member":           {string(gzipped(t, good+"\n")) + "junk", -1},
		"half a member after":           {string(gzipped(t, good+"\n")) + string(gzipped(t, good+"\n")[:5]), -1},
		"padding after a second member": {string(gzipped(t, good+"\n")) + string(gzipped(t, good+"\n")) + "\x00\x00\x00\x00", 2},
		"one byte":                      {"\n", -1},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := readString(name, tc.data)
			if tc.frames < 0 {
				if err == nil {
					t.Error("no error")
				}
				return
			}
			if err != nil || len(got) != tc.frames {
				t.Errorf("%d frames, err %v; want %d", len(got), err, tc.frames)
			}
			for _, r := range got {
				if r.PacketID != got[0].PacketID {
					t.Errorf("frames differ: %+v", r)
				}
			}
		})
	}
}

// A cancelled context stops Next without reading; Close ends the source; a missing
// file is an error from OpenRecording.
func TestRecordingLifecycle(t *testing.T) {
	good := marshal(t, firstLine(t, pitStop))
	path := writeTemp(t, "two.jsonl.gz", gzipped(t, good+"\n"+good+"\n"))
	src, err := salsa.OpenRecording(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := src.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
	// Cancelling read nothing: both frames are still there.
	if got := readSource(t, src); len(got) != 2 {
		t.Errorf("%d frames after a cancelled Next", len(got))
	}
	if _, err := src.Next(context.Background()); !errors.Is(err, io.EOF) {
		t.Errorf("after the end: %v", err)
	}
	if err := src.Close(); err != nil {
		t.Error(err)
	}
	if _, err := src.Next(context.Background()); err == nil || errors.Is(err, io.EOF) {
		t.Errorf("after Close: %v", err)
	}
	if err := src.Close(); err != nil {
		t.Errorf("a second Close: %v", err)
	}

	if _, err := salsa.OpenRecording(filepath.Join(t.TempDir(), "none.jsonl")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a missing file: %v", err)
	}
	if _, err := salsa.OpenRecording(writeTemp(t, "bad.gz", []byte{0x1f, 0x8b, 0})); err == nil {
		t.Error("a damaged gzip header opened")
	}
}

// A packet parsed from a datagram is not from a v1 recording.
func TestFromV1Recording(t *testing.T) {
	plain, err := salsa.Decrypt(readFile(t, "testdata", "golden", "a.datagram"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := salsa.Parse(plain)
	if err != nil || p.FromV1Recording() {
		t.Errorf("a parsed datagram: from a v1 recording %v, err %v", p.FromV1Recording(), err)
	}
	if (salsa.Packet{}).FromV1Recording() {
		t.Error("the zero Packet says it came from a v1 recording")
	}
}

func BenchmarkRecording(b *testing.B) {
	data, err := os.ReadFile(filepath.Join("testdata", "v1", pitStop))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	frames := 0
	for b.Loop() {
		src, err := salsa.NewRecording(bytes.NewReader(data), pitStop)
		if err != nil {
			b.Fatal(err)
		}
		for {
			_, err := src.Next(context.Background())
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				b.Fatal(err)
			}
			frames++
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(frames), "ns/frame")
}

// No input makes the reader panic or loop: it ends with io.EOF or a
// *RecordingError, and every frame it gives is a v1 A packet.
func FuzzRecording(f *testing.F) {
	for _, path := range []string{pitStop, "real-race-2laps-2026-10-14.jsonl.gz"} {
		data, err := os.ReadFile(filepath.Join("testdata", "v1", path)) //nolint:gosec // a fixture under testdata
		if err != nil {
			f.Fatal(err)
		}
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			f.Fatal(err)
		}
		plain, err := io.ReadAll(zr)
		if err != nil {
			f.Fatal(err)
		}
		lines := bytes.SplitAfter(plain, []byte("\n"))
		f.Add(bytes.Join(lines[:3], nil))
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write(bytes.Join(lines[:2], nil))
		_ = zw.Close()
		f.Add(buf.Bytes())
	}
	f.Add([]byte{})
	f.Add([]byte("\n"))
	f.Add([]byte(`{"v":3}`))
	f.Add([]byte{0x1f, 0x8b})
	f.Fuzz(func(t *testing.T, b []byte) {
		src, err := salsa.NewRecording(bytes.NewReader(b), "fuzz")
		var re *salsa.RecordingError
		if err != nil {
			if !errors.As(err, &re) {
				t.Fatalf("NewRecording: %v is not a *RecordingError", err)
			}
			return
		}
		defer func() { _ = src.Close() }()
		for n := 0; ; n++ {
			r, err := src.Next(context.Background())
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				if !errors.As(err, &re) || re.Line < 1 {
					t.Fatalf("Next: %v", err)
				}
				return
			}
			if !r.FromV1Recording() || r.Kind() != salsa.KindA {
				t.Fatalf("frame %d: from a v1 recording %v, kind %s", n+1, r.FromV1Recording(), r.Kind())
			}
		}
	})
}
