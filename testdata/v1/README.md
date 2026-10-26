# v1 recordings

Outing recordings copied unchanged from v1,
[those-who-run/those-who-run](https://github.com/those-who-run/those-who-run) at
tag `v1.0.0`, `tests/fixtures/`, for the tests of `salsa` and `ecu`. They are
GPL-3.0, like `salsa`. Each is a v1 fixture, a real drive cut down with v1's
`those-who-run thin`; its entry in v1's
[`FIXTURES.yaml`](https://github.com/those-who-run/those-who-run/blob/v1.0.0/tests/fixtures/FIXTURES.yaml)
says what was driven and what it pins.

[`FIXTURES.yaml`](FIXTURES.yaml) here lists every file with its v1 path, its
sha256, its frame count and a link to that entry. `TestV1FixtureSums`
(`fixtures_test.go`) fails when a file and its entry disagree.

- `real-race-2laps-2026-10-14.jsonl.gz`: practice laps, then a two-lap race,
  in format version 1 (no full-frame fields), with some values rounded.
- `real-quali-restart-quit-2026-10-14.jsonl.gz`: qualifying with a restart and
  a quit, paused three times.
- `real-nuc-suzuka-no-stop-2026-10-15.jsonl.gz`: a six-lap race at Suzuka East
  with no stop, ten minutes paused on lap 2.
- `real-nuc-suzuka-pit-stop-2026-10-15.jsonl.gz`: a six-lap race at Suzuka East
  with a stop, the pit-in lap posted on a loading frame.
- `real-nuc-willow-springs-pit-stop-2026-10-17.jsonl.gz`: laps 6 to 8 of a race
  at Willow Springs, with a stop.
- `real-nuc-sugo-two-stops-2026-10-18.jsonl.gz`: a 12-lap race at SUGO with two
  stops.
- `real-nuc-sugo-restarts-2026-10-18.jsonl.gz`: three two-lap races at SUGO,
  restarted on lap 2 and then on lap 1, with 45 s of GT7's frozen pause stream.

Whole outings stay in v1's outing archive (v1's ADR 0002); a test that needs one
fetches it by id.
