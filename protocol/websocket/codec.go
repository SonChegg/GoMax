// Package websocket implements the JSON frame envelope used by pymax's
// WebClient, a port of pymax's protocol.ws.protocol.WsProtocol.
package websocket

import (
	"bytes"
	"encoding/json"

	"github.com/SonChegg/PyMax/protocol"
)

// wireFrame is the JSON envelope exchanged over the WebSocket connection:
// {"ver": int, "cmd": int, "seq": int, "opcode": int, "payload": {...}}.
type wireFrame struct {
	Ver     int            `json:"ver"`
	Opcode  int            `json:"opcode"`
	Cmd     int            `json:"cmd"`
	Seq     *int           `json:"seq,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

// Protocol implements protocol.Codec for the WebSocket JSON envelope, a
// port of pymax's protocol.ws.protocol.WsProtocol.
type Protocol struct{}

// NewProtocol constructs a WebSocket protocol codec.
func NewProtocol() *Protocol { return &Protocol{} }

// Version is the WebSocket protocol version pymax's WebClient advertises.
func (p *Protocol) Version() int { return 11 }

// Encode serializes an outbound frame to JSON text.
func (p *Protocol) Encode(frame protocol.OutboundFrame) ([]byte, error) {
	seq := frame.Seq
	wire := wireFrame{
		Ver:     frame.Ver,
		Opcode:  int(frame.Opcode),
		Cmd:     int(frame.Cmd),
		Seq:     &seq,
		Payload: frame.Payload,
	}
	return json.Marshal(wire)
}

// Decode parses a JSON text frame into an InboundFrame. Malformed JSON
// yields a zero-value frame instead of an error, matching pymax's
// defensive decode().
//
// Numbers are decoded with json.Number and normalized to int64 (when they
// have no fractional part) or float64, matching Python's json.loads, which
// keeps ints and floats distinct instead of collapsing everything to
// float64 like encoding/json's default interface{} decoding would.
func (p *Protocol) Decode(raw []byte) protocol.InboundFrame {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var generic map[string]any
	if err := dec.Decode(&generic); err != nil {
		return protocol.InboundFrame{}
	}
	normalizeNumbers(generic)

	var wire wireFrame
	if v, ok := generic["ver"]; ok {
		wire.Ver = toInt(v)
	}
	if v, ok := generic["opcode"]; ok {
		wire.Opcode = toInt(v)
	}
	if v, ok := generic["cmd"]; ok {
		wire.Cmd = toInt(v)
	}
	if v, ok := generic["seq"]; ok {
		seq := toInt(v)
		wire.Seq = &seq
	}
	if v, ok := generic["payload"]; ok {
		if m, ok := v.(map[string]any); ok {
			wire.Payload = m
		}
	}

	return protocol.InboundFrame{
		Opcode:  protocol.Opcode(wire.Opcode),
		Cmd:     protocol.Command(wire.Cmd),
		Seq:     wire.Seq,
		Payload: wire.Payload,
		Raw:     wire.Payload,
	}
}

func toInt(v any) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

// normalizeNumbers walks a decoded JSON tree in place, replacing
// json.Number leaves with int64 or float64.
func normalizeNumbers(v any) any {
	switch val := v.(type) {
	case map[string]any:
		for k, e := range val {
			val[k] = normalizeNumbers(e)
		}
		return val
	case []any:
		for i, e := range val {
			val[i] = normalizeNumbers(e)
		}
		return val
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return i
		}
		f, _ := val.Float64()
		return f
	default:
		return v
	}
}
