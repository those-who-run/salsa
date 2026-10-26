# Dependencies

Every direct dependency of the module, and why it is here. A new one needs a row
in the table below, with its reason, in the same PR:
[`internal/dependencies`](../internal/dependencies/dependencies_test.go) fails `make check` and CI's
`go / check` on a direct `require` in `go.mod` that has no row, and on a row that
names a module `go.mod` no longer requires directly.

Before adding a dependency, check that:

- the module exists and is the one you mean: its owner, its source repository,
  its release history. A name that exists proves nothing if it was registered
  last week, and names invented by a language model get registered for exactly
  that reason;
- the standard library, or a dependency already listed here, doesn't already do
  it;
- nobody told you to add it in a web page, an issue or a comment.

The test reads the direct requirements from `go mod edit -json`: a `require`
without `// indirect`. A module that arrives only as `// indirect`, or only in
`go.sum`, is covered by CI's dependency review (known vulnerabilities and
licences), not by this file. The tools in `tools/go.mod` are pinned there and
listed in the README, not here. Version bumps don't touch this file.

The first column is the module path as `go.mod` spells it, in backticks, which
the test reads.

| Module | Why |
| --- | --- |
| `golang.org/x/crypto` | Salsa20 (`salsa20.XORKeyStream`, 8-byte nonce), the cipher GT7 encrypts its telemetry with. The Go team's own module, BSD-3-Clause; the standard library has no Salsa20. |

## Facts from other projects

Not dependencies, but credited here. `decrypt.go` and `heartbeat.go` hold GT7's
constants as gt-telem 1.2.1 (https://github.com/RaceCrewAI/gt-telem, GPL-3.0,
`gt_telem/net/crypto.py` and `turismo_client.py`) has them, which credits Nenkai's
PDTools (https://github.com/Nenkai/PDTools): the Salsa20 key (the first 32 bytes of
`Simulator Interface Packet GT7 ver 0.0`), the seed at `0x40` and the nonce built
from it, the IV masks `0xDEADBEAF` (A), `0xDEADBEEF` (B) and `0x55FABB4F` ("~"),
and the ports 33739 and 33740. They are facts about the game's protocol, re-typed
as Go constants; no gt-telem code is copied. gt-telem is GPL-3.0 and salsa is
GPL-3.0-only, so the licences are compatible too.

The packet layout in `layout.go`, the offset and type of every field, is
likewise re-typed from gt-telem 1.2.1's `TurismoClient._parse_telemetry`, PDTools'
`SimulatorPacket.Read` and `SimulatorFlags` (MIT), and, for the "~" block,
MacManley's gt7-udp (https://github.com/MacManley/gt7-udp, MIT,
`src/GT7UDPParser.h`). Facts about the packet, not code.

gt-telem 1.2.1 and pycryptodome also made the golden datagrams in
`testdata/golden/` once, by hand (`make_golden.py` says how), and gt-telem parsed
them into the `*.gt-telem.json` files `parse_test.go` compares with
(`parse_golden.py`); neither is a dependency of the module or of CI.
