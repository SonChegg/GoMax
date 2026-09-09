package tcp

import (
	"reflect"
	"testing"

	"github.com/SonChegg/PyMax/protocol"
)

func TestMsgpackRoundtrip(t *testing.T) {
	in := map[string]any{
		"str":   "hello",
		"int":   int64(42),
		"neg":   int64(-42),
		"big":   int64(1 << 40),
		"float": 3.5,
		"bool":  true,
		"null":  nil,
		"list":  []any{int64(1), int64(2), "three"},
		"nested": map[string]any{
			"a": int64(1),
		},
	}

	encoded, err := msgpackEncode(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded, err := msgpackDecode(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	got, ok := decoded.(map[string]any)
	if !ok {
		t.Fatalf("decoded value is not a map: %T", decoded)
	}

	if !reflect.DeepEqual(got, in) {
		t.Fatalf("roundtrip mismatch:\n got=%#v\nwant=%#v", got, in)
	}
}

func TestMsgpackEmptyPayloadDecodesToEmptyMap(t *testing.T) {
	decoded, err := msgpackDecode(nil)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	m, ok := decoded.(map[string]any)
	if !ok || len(m) != 0 {
		t.Fatalf("expected empty map, got %#v", decoded)
	}
}

func TestFramerPackUnpackRoundtrip(t *testing.T) {
	var f Framer
	payload := []byte("hello world")

	packed := f.Pack(10, int(protocol.CommandRequest), 7, 64, 0, payload)

	pkt, err := f.Unpack(packed)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}

	if pkt.Header.Ver != 10 || pkt.Header.Cmd != int(protocol.CommandRequest) ||
		pkt.Header.Seq != 7 || pkt.Header.Opcode != 64 || pkt.Header.Flags != 0 {
		t.Fatalf("unexpected header: %+v", pkt.Header)
	}
	if string(pkt.PayloadBytes) != string(payload) {
		t.Fatalf("payload mismatch: got %q want %q", pkt.PayloadBytes, payload)
	}
}

func TestProtocolEncodeDecodeRoundtrip(t *testing.T) {
	p := NewProtocol()

	frame := protocol.OutboundFrame{
		Ver:    p.Version(),
		Opcode: protocol.OpcodeMsgSend,
		Cmd:    protocol.CommandRequest,
		Seq:    3,
		Payload: map[string]any{
			"chatId": int64(123),
			"text":   "hi",
		},
	}

	raw, err := p.Encode(frame)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded := p.Decode(raw)
	if decoded.Opcode != frame.Opcode || decoded.Cmd != frame.Cmd || decoded.Seq == nil || *decoded.Seq != frame.Seq {
		t.Fatalf("unexpected decoded frame: %+v", decoded)
	}
	if decoded.Payload["chatId"] != int64(123) || decoded.Payload["text"] != "hi" {
		t.Fatalf("unexpected decoded payload: %+v", decoded.Payload)
	}
}

// TestLz4BlockCompressionShortInputRoundtrip covers the one case pymax's
// hand-rolled LZ4 compress() is actually exercised for (a single literal
// chunk under 15 bytes, no back-reference): the wider byte-for-byte port
// intentionally keeps pymax's compress() implementation as-is, including
// its known limitation that a literal chunk without a following match is
// only unambiguous when it lands at end-of-stream. pymax itself never
// calls compress() in the live encode path (it's commented out upstream;
// only decompress() is used, for reading compressed server responses), so
// this test only checks the well-defined case.
func TestLz4BlockCompressionShortInputRoundtrip(t *testing.T) {
	var lz4 lz4BlockCompression
	original := []byte("short input")

	compressed := lz4.compress(original)
	decompressed, err := lz4.decompress(compressed, defaultMaxOutput)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if string(decompressed) != string(original) {
		t.Fatalf("roundtrip mismatch:\n got=%q\nwant=%q", decompressed, original)
	}
}

// TestLz4BlockCompressionDecompressLiteralOnly hand-builds a compressed
// buffer (a single literal token, no match) to test decompress()
// independently of compress()'s narrower guarantees.
func TestLz4BlockCompressionDecompressLiteralOnly(t *testing.T) {
	var lz4 lz4BlockCompression
	literal := []byte("hello")
	token := byte(len(literal) << 4)
	compressed := append([]byte{token}, literal...)

	decompressed, err := lz4.decompress(compressed, defaultMaxOutput)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if string(decompressed) != string(literal) {
		t.Fatalf("roundtrip mismatch:\n got=%q\nwant=%q", decompressed, literal)
	}
}
