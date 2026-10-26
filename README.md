# Salsa

The GT7 telemetry decoder for Those Who Run v2: it decrypts the game's
datagrams with Salsa20, parses them into typed packets, and reads v1's
recordings as the same packets. `ecu` imports it.
It ships as a Go module, not a service: a release is its git tag.

Issues are off in this repository. Every issue, the roadmap and the decisions
(ADRs) live in [those-who-run/paddock](https://github.com/those-who-run/paddock). A pull request here
names its paddock issue: `Closes those-who-run/paddock#N.` when it meets every
acceptance criterion, `Part of those-who-run/paddock#N` otherwise
([`AGENTS.md`](AGENTS.md)).

## Use

```go
plain, err := salsa.Decrypt(datagram) // a datagram as received on salsa.TelemetryPort
if errors.Is(err, salsa.ErrLength) { /* not 296, 316 or 344 bytes */ }
if errors.Is(err, salsa.ErrMagic) { /* didn't decrypt to GT7's magic */ }
// plain.Kind is salsa.KindA, KindB or KindTilde, from the length;
// plain.Bytes is the plaintext.

p, err := salsa.Parse(plain)
// p.Speed is in m/s, as GT7 sends it; p.SpeedKPH() converts.
// p.Flags.TCS(), p.Flags.Paused(), ... read the flags word; p.Gears.Current().
if p.B != nil { /* a B or "~" packet: p.B.Sway, p.B.Heave, p.B.Surge, ... */ }
if p.Tilde != nil { /* a "~" packet: p.Tilde.EnergyRecovery, ... */ }
```

`Decrypt` is Salsa20 with GT7's key; the nonce comes from the seed at `0x40` of
the datagram and the IV mask of its kind, which its length gives. The heartbeat
payloads (`HeartbeatA`, `HeartbeatB`, `HeartbeatTilde`) and the PS5's ports
(`HeartbeatPort`, 33739/UDP, and `TelemetryPort`, 33740/UDP) are constants
beside it.

`Parse` reads every field of the A layout, the ones v1 dropped included (the
whole flags word, the alert RPM band, clutch, gear ratios, road plane), and the B
and "~" blocks when the packet has them; `Packet.B` and `Packet.Tilde` are nil
when it doesn't, so "not sent" is never read as zero. Values keep GT7's units.
[`layout.go`](layout.go) is the offset table, one line per field naming its source.
The "~" block's first four bytes and its vector, and flags bit 5, keep neutral
names (`TildeByte1` to `TildeByte4`, `TildeVector`, `Bit5`) until a capture
shows what they are. `Packet.Plain` is `Parse`'s inverse, and
`salsatest.Encode` turns a `Packet` into the datagram the PS5 would send, for
tests here and in `ecu`. `Plain.Seed` and `Packet.Seed` carry the datagram's
seed; `Packet.IV` is the plaintext's bytes at `0x40`, the seed XOR the keystream,
so a `Packet` through `Encode`, `Decrypt` and `Parse` comes back equal but for
`IV`.

[`docs/dependencies.md`](docs/dependencies.md) credits where the constants and
the layout come from.

## Checks

`make check` runs what CI runs; [`AGENTS.md`](AGENTS.md) says what that is and
what a new clone runs first. CI also fuzzes `Decrypt` and `Parse` for 30 s
each in its `fuzz` job, which `make fuzz` runs on a laptop.

## Conventions

The ones every Go component shares, from the
[`chassis`](https://github.com/those-who-run/chassis) template.
This repository is a library, with no command or service yet; they apply to any
it adds under `cmd/`:

- **Configuration from the environment**, read once at start-up into a typed
  struct; an unset variable takes its default and a value that can't be read
  stops the process.
- **Logging with `log/slog`, as JSON** on standard output, one object per line,
  with key-value pairs.
- **A health endpoint**, `GET /healthz`, which answers 200 while the process can
  serve.

## From the template

This repository was made from `chassis` with `copier copy`, and
`.copier-answers.yml` records the release it came from and the answers given.
Renovate proposes each new `chassis` release as a PR that runs
`copier update`; chassis's README has the rest.

## Releases

Merging the release PR tags `vX.Y.Z`; the module is used by that tag. It ships
no image.
