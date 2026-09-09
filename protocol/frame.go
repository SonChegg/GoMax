// Package protocol defines the wire-level frame model shared by the TCP and
// WebSocket protocols, along with the opcode/command constants of the Max
// protocol. It is a port of pymax's protocol package (protocol/base.py,
// protocol/enums.py, protocol/models.py).
package protocol

// Command is the low-level frame kind of the Max protocol, a port of
// pymax's protocol.enums.Command.
type Command int

const (
	CommandRequest  Command = 0
	CommandResponse Command = 1
	CommandEvent    Command = 2
	CommandError    Command = 3
)

// OutboundFrame is a request/event frame sent to the server, a port of
// pymax's protocol.models.OutboundFrame.
type OutboundFrame struct {
	Ver     int
	Opcode  Opcode
	Cmd     Command
	Seq     int
	Payload map[string]any
}

// InboundFrame is a frame decoded from the server, a port of pymax's
// protocol.models.InboundFrame.
type InboundFrame struct {
	Opcode  Opcode
	Cmd     Command
	Seq     *int
	Payload map[string]any
	Raw     map[string]any
}

// Codec encodes OutboundFrame values to wire bytes and decodes wire bytes
// back into InboundFrame values. TCP and WebSocket each provide their own
// implementation (a port of pymax's protocol.base.BaseProtocol).
type Codec interface {
	// Version is the protocol version advertised in outbound frames.
	Version() int
	// Encode serializes a frame to bytes ready to hand to a Transport.
	Encode(frame OutboundFrame) ([]byte, error)
	// Decode parses bytes received from a Transport into a frame. Decode
	// never returns an error for malformed input; it returns a zero-value
	// frame instead, matching pymax's defensive decode() behavior.
	Decode(raw []byte) InboundFrame
}
