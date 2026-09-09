// Package tcp implements the raw TCP wire protocol used by pymax.Client: a
// fixed binary header, msgpack payload encoding and optional LZ4/Zstd
// payload compression. It is a port of pymax's protocol.tcp package
// (framing.py, payload.py, compression.py, protocol.py).
package tcp

import (
	"encoding/binary"
	"fmt"
)

// HeaderSize is the size in bytes of a packed TCP packet header: ver(1) +
// cmd(1) + seq(2) + opcode(2) + packedLen(4), matching Python's
// struct.Struct(">BBHHI").
const HeaderSize = 10

// PacketHeader is the fixed binary header preceding every TCP packet
// payload, a port of pymax's protocol.models.TcpPacketHeader.
type PacketHeader struct {
	Ver        int
	Cmd        int
	Seq        int
	Opcode     int
	Flags      int
	PayloadLen int
}

// PackedPacket is a decoded header plus its raw (still possibly compressed)
// payload bytes, a port of pymax's protocol.models.PackedPacket.
type PackedPacket struct {
	Header       PacketHeader
	PayloadBytes []byte
}

// Framer packs and unpacks the fixed TCP header, a port of pymax's
// protocol.tcp.framing.TcpPacketFramer.
type Framer struct{}

// Pack builds a full packet (header + payload) from its fields.
func (Framer) Pack(ver, cmd, seq, opcode, flags int, payload []byte) []byte {
	packedLen := (uint32(flags&0xFF) << 24) | (uint32(len(payload)) & 0x00FFFFFF)

	out := make([]byte, HeaderSize+len(payload))
	out[0] = byte(ver)
	out[1] = byte(cmd)
	binary.BigEndian.PutUint16(out[2:4], uint16(seq))
	binary.BigEndian.PutUint16(out[4:6], uint16(opcode))
	binary.BigEndian.PutUint32(out[6:10], packedLen)
	copy(out[HeaderSize:], payload)
	return out
}

// Unpack parses a full packet (header + payload) from data. It returns an
// error if data is shorter than the declared payload length.
func (Framer) Unpack(data []byte) (*PackedPacket, error) {
	if len(data) < HeaderSize {
		return nil, fmt.Errorf("tcp: short header: %d bytes", len(data))
	}

	ver := int(data[0])
	cmd := int(data[1])
	seq := int(binary.BigEndian.Uint16(data[2:4]))
	opcode := int(binary.BigEndian.Uint16(data[4:6]))
	packedLen := binary.BigEndian.Uint32(data[6:10])

	flags := int((packedLen >> 24) & 0xFF)
	payloadLen := int(packedLen & 0x00FFFFFF)

	totalLen := HeaderSize + payloadLen
	if len(data) < totalLen {
		return nil, fmt.Errorf("tcp: short payload: have %d, want %d", len(data), totalLen)
	}

	return &PackedPacket{
		Header: PacketHeader{
			Ver:        ver,
			Cmd:        cmd,
			Seq:        seq,
			Opcode:     opcode,
			Flags:      flags,
			PayloadLen: payloadLen,
		},
		PayloadBytes: data[HeaderSize:totalLen],
	}, nil
}

// UnpackHeader returns just the declared payload length from a header-sized
// prefix, used by the reader to know how many more bytes to read.
func (Framer) UnpackHeader(data []byte) (int, error) {
	if len(data) < HeaderSize {
		return 0, fmt.Errorf("tcp: short header: %d bytes", len(data))
	}
	packedLen := binary.BigEndian.Uint32(data[6:10])
	return int(packedLen & 0x00FFFFFF), nil
}
