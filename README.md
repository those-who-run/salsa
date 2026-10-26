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
```

`Decrypt` is Salsa20 with GT7's key; the nonce comes from the seed at `0x40` of
the datagram and the IV mask of its kind, which its length gives. The heartbeat
payloads (`HeartbeatA`, `HeartbeatB`, `HeartbeatTilde`) and the PS5's ports
(`HeartbeatPort`, 33739/UDP, and `TelemetryPort`, 33740/UDP) are constants
beside it. Parsing the plaintext into fields is still to come.
[`docs/dependencies.md`](docs/dependencies.md) credits where the constants come from.

## Checks

`make check` runs what CI runs; [`AGENTS.md`](AGENTS.md) says what that is and
what a new clone runs first. CI also fuzzes `Decrypt` for 30 s in its `fuzz`
job, which `make fuzz` runs on a laptop.

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
