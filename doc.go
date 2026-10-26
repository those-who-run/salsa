// Package salsa decodes GT7's telemetry datagrams. Decrypt takes a datagram as the
// PS5 sends it and returns its plaintext, the kind of packet (A, B or "~") read from
// its length; Parse reads the plaintext into a typed Packet, with the B and "~"
// blocks when the packet has them. The heartbeat facts the live listener needs, its
// payloads and ports, are constants here too.
//
// OpenRecording reads v1's outing recordings (RaceState JSON lines, plain or gzip) as
// the same Packets, each with its receive time, behind Source, the one interface for
// packets from a recording now and from the live listener later.
//
// The algorithm and its constants (the Salsa20 key, where the seed sits, the IV mask
// of each packet kind) are those of gt-telem 1.2.1 (https://github.com/RaceCrewAI/gt-telem,
// GPL-3.0, `gt_telem/net/crypto.py`), which credits Nenkai's PDTools
// (https://github.com/Nenkai/PDTools); docs/dependencies.md says how they are used.
// The packet layout is gt-telem's and PDTools', with MacManley's gt7-udp for "~";
// layout.go names the source of each field.
package salsa
