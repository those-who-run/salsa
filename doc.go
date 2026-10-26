// Package salsa decodes GT7's telemetry datagrams. Decrypt takes a datagram as the
// PS5 sends it and returns its plaintext, the kind of packet (A, B or "~") read from
// its length; parsing the plaintext into fields comes later
// (those-who-run/paddock#31). The heartbeat facts the live listener needs, its
// payloads and ports, are constants here too.
//
// The algorithm and its constants (the Salsa20 key, where the seed sits, the IV mask
// of each packet kind) are those of gt-telem 1.2.1 (https://github.com/RaceCrewAI/gt-telem,
// GPL-3.0, `gt_telem/net/crypto.py`), which credits Nenkai's PDTools
// (https://github.com/Nenkai/PDTools); docs/dependencies.md says how they are used.
package salsa
