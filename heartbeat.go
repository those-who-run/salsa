package salsa

// The heartbeat a client sends to the PS5 to keep telemetry coming: one ASCII byte,
// which also picks the kind of packet the game sends back.
const (
	// HeartbeatA asks for A packets, 296 bytes.
	HeartbeatA byte = 'A'
	// HeartbeatB asks for B packets, 316 bytes: A with the motion block.
	HeartbeatB byte = 'B'
	// HeartbeatTilde asks for "~" packets, 344 bytes: B with the "~" block.
	HeartbeatTilde byte = '~'
)

// The PS5's UDP ports for GT7: gt-telem's 33339 and 33340 for GT Sport, plus the
// 400 it adds for GT7.
const (
	// HeartbeatPort is the PS5's port a client sends its heartbeat to.
	HeartbeatPort = 33739
	// TelemetryPort is the client's port the PS5 sends its datagrams to.
	TelemetryPort = 33740
)
