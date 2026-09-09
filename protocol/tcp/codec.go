package tcp

import (
	"fmt"

	"github.com/SonChegg/PyMax/protocol"
)

// Protocol implements protocol.Codec for the raw TCP wire format: a fixed
// binary header (Framer) wrapping a msgpack-encoded, optionally
// LZ4/Zstd-compressed payload. It is a port of pymax's
// protocol.tcp.protocol.TcpProtocol.
type Protocol struct {
	framer      Framer
	lz4         lz4BlockCompression
	zstd        zstdCompression
	maxPayloadN int
}

// NewProtocol constructs a TCP protocol codec.
func NewProtocol() *Protocol {
	return &Protocol{maxPayloadN: defaultMaxOutput}
}

// Version is the TCP protocol version pymax's Client advertises.
func (p *Protocol) Version() int { return 10 }

// Encode serializes an outbound frame into a full TCP packet. Payload
// compression is currently disabled, matching pymax's TcpProtocol.encode
// (the LZ4 compress path is commented out upstream).
func (p *Protocol) Encode(frame protocol.OutboundFrame) ([]byte, error) {
	var payloadBytes []byte
	if frame.Payload != nil {
		encoded, err := msgpackEncode(frame.Payload)
		if err != nil {
			return nil, fmt.Errorf("tcp: encode payload: %w", err)
		}
		payloadBytes = encoded
	}

	flags := 0
	return p.framer.Pack(frame.Ver, int(frame.Cmd), frame.Seq, int(frame.Opcode), flags, payloadBytes), nil
}

// Decode parses a full TCP packet (as produced by the TCP reader) into an
// InboundFrame. Malformed input yields a zero-value frame instead of an
// error, matching pymax's defensive decode().
func (p *Protocol) Decode(raw []byte) protocol.InboundFrame {
	packet, err := p.framer.Unpack(raw)
	if err != nil {
		return protocol.InboundFrame{}
	}

	payload, err := p.decodePayload(packet.PayloadBytes, packet.Header.Flags)
	if err != nil {
		return protocol.InboundFrame{}
	}

	seq := packet.Header.Seq
	return protocol.InboundFrame{
		Opcode:  protocol.Opcode(packet.Header.Opcode),
		Cmd:     protocol.Command(packet.Header.Cmd),
		Seq:     &seq,
		Payload: payload,
		Raw:     payload,
	}
}

// decodePayload decompresses (if needed) and msgpack-decodes a payload,
// a port of pymax's protocol.tcp.payload.TcpPayloadDecoder.decode.
func (p *Protocol) decodePayload(payloadBytes []byte, flags int) (map[string]any, error) {
	if len(payloadBytes) == 0 {
		return map[string]any{}, nil
	}

	switch {
	case flags == 0xFF:
		decompressed, err := p.zstd.decompress(payloadBytes, p.maxPayloadN)
		if err != nil {
			return nil, err
		}
		payloadBytes = decompressed
	case flags > 0x7F:
		return nil, fmt.Errorf("tcp: invalid compression factor: %d", flags)
	case flags > 0:
		decompressed, err := p.lz4.decompress(payloadBytes, p.maxPayloadN)
		if err != nil {
			return nil, err
		}
		payloadBytes = decompressed
	}

	decoded, err := msgpackDecode(payloadBytes)
	if err != nil {
		return nil, err
	}

	if decoded == nil {
		// Top-level payload decoded to msgpack nil (e.g. an ack/notification
		// with no data). pymax's TcpPayloadDecoder._normalize_keys(None)
		// passes None straight through, leaving InboundFrame.payload = None
		// rather than a dict; return nil here (not a map wrapping a nil
		// value) so callers' `payload == nil` / falsy checks agree with
		// pymax's `if not response.payload:` and treat this as "no
		// payload" instead of a present-but-empty one.
		return nil, nil
	}

	m, ok := decoded.(map[string]any)
	if !ok {
		// Top-level payload was not a map; wrap it so callers always see a
		// map, matching pymax's dict[str, Any] decode() return type.
		return map[string]any{"": decoded}, nil
	}
	return m, nil
}
