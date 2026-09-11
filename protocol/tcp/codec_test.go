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

// element mirrors types.Element's shape without importing the types
// package (which would create an import cycle: types -> ... -> protocol/tcp).
type element struct {
	Type   string `json:"type"`
	From   *int   `json:"from,omitempty"`
	Length *int   `json:"length,omitempty"`
}

// TestMsgpackEncodesTypedSlicesAndStructs guards against a regression hit
// live: encodeValue's fast-path switch only recognized map[string]any and
// []any, so any concretely-typed slice ([]int64 — e.g. UserService.
// GetUsers' contactIds payload) or slice of structs ([]types.Element —
// e.g. MessageService.SendMessage's elements payload) failed with
// "unsupported type", breaking contact resolution and sending any message
// with formatted text. The JSON-fallback path must handle both, and must
// keep integers as msgpack ints rather than promoting them to floats.
func TestMsgpackEncodesTypedSlicesAndStructs(t *testing.T) {
	from := 0
	length := 5
	payload := map[string]any{
		"contactIds": []int64{111, 222, 333},
		"elements":   []element{{Type: "STRONG", From: &from, Length: &length}},
	}

	encoded, err := msgpackEncode(payload)
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

	ids, ok := got["contactIds"].([]any)
	if !ok || len(ids) != 3 {
		t.Fatalf("contactIds: got %#v", got["contactIds"])
	}
	for i, want := range []int64{111, 222, 333} {
		if id, ok := ids[i].(int64); !ok || id != want {
			t.Fatalf("contactIds[%d]: got %#v (%T), want int64(%d) — ints must not be promoted to floats", i, ids[i], ids[i], want)
		}
	}

	els, ok := got["elements"].([]any)
	if !ok || len(els) != 1 {
		t.Fatalf("elements: got %#v", got["elements"])
	}
	el, ok := els[0].(map[string]any)
	if !ok || el["type"] != "STRONG" {
		t.Fatalf("elements[0]: got %#v", els[0])
	}
	if from, ok := el["from"].(int64); !ok || from != 0 {
		t.Fatalf("elements[0].from: got %#v (%T)", el["from"], el["from"])
	}
}

// TestMsgpackEncodesByteSlicesInsideTypedMapSlice guards against a
// regression where a concretely-typed []map[string]any (exactly what
// api.uploadAttachments/toPayload build for a message's "attaches" list)
// fell through to the JSON fallback path because it isn't []any — and that
// fallback, going through encoding/json, silently turned a nested []byte
// field (the voice-message waveform) into a base64 string instead of a
// real msgpack bin value, which Max's server then rejected outright.
func TestMsgpackEncodesByteSlicesInsideTypedMapSlice(t *testing.T) {
	attaches := []map[string]any{
		{"_type": "AUDIO", "wave": []byte{1, 2, 3, 4, 5}},
	}

	encoded, err := msgpackEncode(map[string]any{"attaches": attaches})
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
	list, ok := got["attaches"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("attaches: got %#v", got["attaches"])
	}
	entry, ok := list[0].(map[string]any)
	if !ok {
		t.Fatalf("attaches[0]: got %#v", list[0])
	}
	wave, ok := entry["wave"].([]byte)
	if !ok {
		t.Fatalf("attaches[0][\"wave\"] is %T, want []byte (msgpack bin) — a string here means it was base64-encoded via the JSON fallback", entry["wave"])
	}
	if string(wave) != string([]byte{1, 2, 3, 4, 5}) {
		t.Fatalf("wave = %v, want [1 2 3 4 5]", wave)
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

// TestDecodePayloadMsgpackNilYieldsNoPayload covers a server frame whose
// top-level msgpack value is nil (0xc0), e.g. an ack/notification carrying
// no data. pymax's TcpPayloadDecoder passes that None straight through
// (InboundFrame.payload stays None); decodePayload must likewise report "no
// payload" (a nil map) rather than wrapping the nil value in
// map[string]any{"": nil}, which downstream falsy/`payload == nil` checks
// (mirroring pymax's `if not response.payload:`) would otherwise miss.
func TestDecodePayloadMsgpackNilYieldsNoPayload(t *testing.T) {
	p := NewProtocol()

	nilPayload, err := msgpackEncode(nil)
	if err != nil {
		t.Fatalf("msgpackEncode(nil): %v", err)
	}

	payload, err := p.decodePayload(nilPayload, 0)
	if err != nil {
		t.Fatalf("decodePayload: %v", err)
	}
	if payload != nil {
		t.Fatalf("expected a nil payload, got %#v", payload)
	}
}

// TestProtocolDecodeMsgpackNilPayloadFrame covers the same scenario at the
// full Protocol.Decode level: a packed frame whose payload bytes decode to
// msgpack nil should produce an InboundFrame with a nil Payload/Raw, not one
// that looks like it carries an (empty-keyed) value.
func TestProtocolDecodeMsgpackNilPayloadFrame(t *testing.T) {
	p := NewProtocol()

	nilPayload, err := msgpackEncode(nil)
	if err != nil {
		t.Fatalf("msgpackEncode(nil): %v", err)
	}

	var f Framer
	raw := f.Pack(10, 2, 5, 128, 0, nilPayload)

	decoded := p.Decode(raw)
	if decoded.Payload != nil {
		t.Fatalf("expected nil Payload, got %#v", decoded.Payload)
	}
	if decoded.Raw != nil {
		t.Fatalf("expected nil Raw, got %#v", decoded.Raw)
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
