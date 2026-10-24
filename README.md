# Salsa

The GT7 telemetry decoder for Those Who Run v2: it decrypts the game's
datagrams and parses them into typed packets.
It ships as a Go module, not a service.

Issues are off in this repository. Every issue, the roadmap and the decisions
(ADRs) live in [those-who-run/paddock](https://github.com/those-who-run/paddock). A pull request here closes
its paddock issue with `Closes those-who-run/paddock#N.`

## Checks

`make check` runs what CI runs; [`AGENTS.md`](AGENTS.md) says what that is and
what a new clone runs first.
