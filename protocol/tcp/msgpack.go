package tcp

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// ExtType represents an undecoded MessagePack extension value (any ext code
// other than wrappedValueExtCode), mirroring msgpack.ExtType from the
// Python msgpack library.
type ExtType struct {
	Code int8
	Data []byte
}

// wrappedValueExtCode is pymax's MsgpackPayloadCodec.WRAPPED_VALUE_EXT_CODE:
// an extension type whose payload is itself a nested MessagePack value that
// must be recursively unpacked.
const wrappedValueExtCode = 1

// msgpackEncode serializes a Go value (map[string]any, []any, string,
// []byte, integers, float64, bool, nil) to MessagePack bytes, a port of
// pymax's protocol.tcp.payload.MsgpackPayloadCodec.encode.
func msgpackEncode(v any) ([]byte, error) {
	var buf []byte
	buf, err := encodeValue(buf, v)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func encodeValue(buf []byte, v any) ([]byte, error) {
	switch val := v.(type) {
	case nil:
		return append(buf, 0xc0), nil
	case bool:
		if val {
			return append(buf, 0xc3), nil
		}
		return append(buf, 0xc2), nil
	case string:
		return encodeString(buf, val), nil
	case []byte:
		return encodeBin(buf, val), nil
	case int:
		return encodeInt(buf, int64(val)), nil
	case int8:
		return encodeInt(buf, int64(val)), nil
	case int16:
		return encodeInt(buf, int64(val)), nil
	case int32:
		return encodeInt(buf, int64(val)), nil
	case int64:
		return encodeInt(buf, val), nil
	case uint:
		return encodeUint(buf, uint64(val)), nil
	case uint8:
		return encodeUint(buf, uint64(val)), nil
	case uint16:
		return encodeUint(buf, uint64(val)), nil
	case uint32:
		return encodeUint(buf, uint64(val)), nil
	case uint64:
		return encodeUint(buf, val), nil
	case float32:
		return encodeFloat64(buf, float64(val)), nil
	case float64:
		return encodeFloat64(buf, val), nil
	case map[string]any:
		return encodeMap(buf, val)
	case []any:
		return encodeArray(buf, val)
	default:
		return encodeViaJSONFallback(buf, v)
	}
}

// encodeViaJSONFallback handles any Go value the fast-path switch in
// encodeValue doesn't recognize directly: a concretely-typed slice
// ([]int64, []types.Element, ...), a typed map, a pointer, or a struct.
// Rather than hand-writing reflection for every shape, it round-trips the
// value through encoding/json (using its `json:"..."` tags — the same
// tags pymax's CamelModel payload fields rely on) into the generic
// map[string]any/[]any/... shape encodeValue already knows how to encode,
// mirroring how pydantic's model_dump() flattens nested models before
// pymax's msgpack codec ever sees them.
//
// json.Number (via UseNumber) avoids round-tripping every integer through
// float64, which would change ints to msgpack floats on the wire.
func encodeViaJSONFallback(buf []byte, v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("tcp: msgpack: unsupported type %T: %w", v, err)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var generic any
	if err := dec.Decode(&generic); err != nil {
		return nil, fmt.Errorf("tcp: msgpack: unsupported type %T: %w", v, err)
	}

	return encodeValue(buf, normalizeJSONNumbers(generic))
}

// normalizeJSONNumbers walks a tree decoded with json.Number in place,
// replacing each json.Number with an int64 (if it has no fractional part
// or exponent) or a float64 otherwise.
func normalizeJSONNumbers(v any) any {
	switch val := v.(type) {
	case map[string]any:
		for k, e := range val {
			val[k] = normalizeJSONNumbers(e)
		}
		return val
	case []any:
		for i, e := range val {
			val[i] = normalizeJSONNumbers(e)
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

func encodeString(buf []byte, s string) []byte {
	n := len(s)
	switch {
	case n < 32:
		buf = append(buf, 0xa0|byte(n))
	case n < 1<<8:
		buf = append(buf, 0xd9, byte(n))
	case n < 1<<16:
		buf = append(buf, 0xda, byte(n>>8), byte(n))
	default:
		buf = append(buf, 0xdb, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
	return append(buf, s...)
}

func encodeBin(buf, b []byte) []byte {
	n := len(b)
	switch {
	case n < 1<<8:
		buf = append(buf, 0xc4, byte(n))
	case n < 1<<16:
		buf = append(buf, 0xc5, byte(n>>8), byte(n))
	default:
		buf = append(buf, 0xc6, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
	return append(buf, b...)
}

func encodeInt(buf []byte, i int64) []byte {
	switch {
	case i >= 0:
		return encodeUint(buf, uint64(i))
	case i >= -32:
		return append(buf, byte(i))
	case i >= math.MinInt8:
		return append(buf, 0xd0, byte(i))
	case i >= math.MinInt16:
		u := uint16(i)
		return append(buf, 0xd1, byte(u>>8), byte(u))
	case i >= math.MinInt32:
		u := uint32(i)
		return append(buf, 0xd2, byte(u>>24), byte(u>>16), byte(u>>8), byte(u))
	default:
		u := uint64(i)
		return append(buf, 0xd3,
			byte(u>>56), byte(u>>48), byte(u>>40), byte(u>>32),
			byte(u>>24), byte(u>>16), byte(u>>8), byte(u))
	}
}

func encodeUint(buf []byte, u uint64) []byte {
	switch {
	case u < 128:
		return append(buf, byte(u))
	case u < 1<<8:
		return append(buf, 0xcc, byte(u))
	case u < 1<<16:
		return append(buf, 0xcd, byte(u>>8), byte(u))
	case u < 1<<32:
		return append(buf, 0xce, byte(u>>24), byte(u>>16), byte(u>>8), byte(u))
	default:
		return append(buf, 0xcf,
			byte(u>>56), byte(u>>48), byte(u>>40), byte(u>>32),
			byte(u>>24), byte(u>>16), byte(u>>8), byte(u))
	}
}

func encodeFloat64(buf []byte, f float64) []byte {
	bits := math.Float64bits(f)
	buf = append(buf, 0xcb)
	return append(buf,
		byte(bits>>56), byte(bits>>48), byte(bits>>40), byte(bits>>32),
		byte(bits>>24), byte(bits>>16), byte(bits>>8), byte(bits))
}

func encodeMap(buf []byte, m map[string]any) ([]byte, error) {
	n := len(m)
	switch {
	case n < 16:
		buf = append(buf, 0x80|byte(n))
	case n < 1<<16:
		buf = append(buf, 0xde, byte(n>>8), byte(n))
	default:
		buf = append(buf, 0xdf, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}

	var err error
	for k, v := range m {
		buf = encodeString(buf, k)
		buf, err = encodeValue(buf, v)
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}

func encodeArray(buf []byte, a []any) ([]byte, error) {
	n := len(a)
	switch {
	case n < 16:
		buf = append(buf, 0x90|byte(n))
	case n < 1<<16:
		buf = append(buf, 0xdc, byte(n>>8), byte(n))
	default:
		buf = append(buf, 0xdd, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}

	var err error
	for _, v := range a {
		buf, err = encodeValue(buf, v)
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}

// msgpackDecoder decodes MessagePack bytes into generic Go values,
// recursively unwrapping wrappedValueExtCode extensions, a port of pymax's
// protocol.tcp.payload.MsgpackPayloadCodec.decode.
type msgpackDecoder struct {
	data []byte
	pos  int
}

func msgpackDecode(data []byte) (any, error) {
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	d := &msgpackDecoder{data: data}
	v, err := d.decodeValue()
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (d *msgpackDecoder) readByte() (byte, error) {
	if d.pos >= len(d.data) {
		return 0, fmt.Errorf("tcp: msgpack: unexpected end of data")
	}
	b := d.data[d.pos]
	d.pos++
	return b, nil
}

func (d *msgpackDecoder) readN(n int) ([]byte, error) {
	if d.pos+n > len(d.data) {
		return nil, fmt.Errorf("tcp: msgpack: unexpected end of data")
	}
	b := d.data[d.pos : d.pos+n]
	d.pos += n
	return b, nil
}

func (d *msgpackDecoder) decodeValue() (any, error) {
	c, err := d.readByte()
	if err != nil {
		return nil, err
	}

	switch {
	case c <= 0x7f: // positive fixint
		return int64(c), nil
	case c >= 0xe0: // negative fixint
		return int64(int8(c)), nil
	case c&0xf0 == 0x80: // fixmap
		return d.decodeMap(int(c & 0x0f))
	case c&0xf0 == 0x90: // fixarray
		return d.decodeArray(int(c & 0x0f))
	case c&0xe0 == 0xa0: // fixstr
		return d.decodeStr(int(c & 0x1f))
	}

	switch c {
	case 0xc0:
		return nil, nil
	case 0xc2:
		return false, nil
	case 0xc3:
		return true, nil
	case 0xc4:
		n, err := d.readByte()
		if err != nil {
			return nil, err
		}
		return d.readN(int(n))
	case 0xc5:
		b, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return d.readN(int(binary.BigEndian.Uint16(b)))
	case 0xc6:
		b, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return d.readN(int(binary.BigEndian.Uint32(b)))
	case 0xc7: // ext8
		n, err := d.readByte()
		if err != nil {
			return nil, err
		}
		return d.decodeExt(int(n))
	case 0xc8: // ext16
		b, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return d.decodeExt(int(binary.BigEndian.Uint16(b)))
	case 0xc9: // ext32
		b, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return d.decodeExt(int(binary.BigEndian.Uint32(b)))
	case 0xca:
		b, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return float64(math.Float32frombits(binary.BigEndian.Uint32(b))), nil
	case 0xcb:
		b, err := d.readN(8)
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(binary.BigEndian.Uint64(b)), nil
	case 0xcc:
		b, err := d.readByte()
		return int64(b), err
	case 0xcd:
		b, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return int64(binary.BigEndian.Uint16(b)), nil
	case 0xce:
		b, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return int64(binary.BigEndian.Uint32(b)), nil
	case 0xcf:
		b, err := d.readN(8)
		if err != nil {
			return nil, err
		}
		return int64(binary.BigEndian.Uint64(b)), nil
	case 0xd0:
		b, err := d.readByte()
		return int64(int8(b)), err
	case 0xd1:
		b, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return int64(int16(binary.BigEndian.Uint16(b))), nil
	case 0xd2:
		b, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return int64(int32(binary.BigEndian.Uint32(b))), nil
	case 0xd3:
		b, err := d.readN(8)
		if err != nil {
			return nil, err
		}
		return int64(binary.BigEndian.Uint64(b)), nil
	case 0xd4, 0xd5, 0xd6, 0xd7, 0xd8: // fixext 1/2/4/8/16
		sizes := map[byte]int{0xd4: 1, 0xd5: 2, 0xd6: 4, 0xd7: 8, 0xd8: 16}
		return d.decodeExt(sizes[c])
	case 0xd9:
		n, err := d.readByte()
		if err != nil {
			return nil, err
		}
		return d.decodeStr(int(n))
	case 0xda:
		b, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return d.decodeStr(int(binary.BigEndian.Uint16(b)))
	case 0xdb:
		b, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return d.decodeStr(int(binary.BigEndian.Uint32(b)))
	case 0xdc:
		b, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return d.decodeArray(int(binary.BigEndian.Uint16(b)))
	case 0xdd:
		b, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return d.decodeArray(int(binary.BigEndian.Uint32(b)))
	case 0xde:
		b, err := d.readN(2)
		if err != nil {
			return nil, err
		}
		return d.decodeMap(int(binary.BigEndian.Uint16(b)))
	case 0xdf:
		b, err := d.readN(4)
		if err != nil {
			return nil, err
		}
		return d.decodeMap(int(binary.BigEndian.Uint32(b)))
	}

	return nil, fmt.Errorf("tcp: msgpack: unsupported leading byte 0x%02x", c)
}

func (d *msgpackDecoder) decodeStr(n int) (string, error) {
	b, err := d.readN(n)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (d *msgpackDecoder) decodeArray(n int) ([]any, error) {
	out := make([]any, n)
	for i := 0; i < n; i++ {
		v, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (d *msgpackDecoder) decodeMap(n int) (map[string]any, error) {
	out := make(map[string]any, n)
	for i := 0; i < n; i++ {
		key, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		val, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		out[normalizeKey(key)] = val
	}
	return out, nil
}

func (d *msgpackDecoder) decodeExt(n int) (any, error) {
	codeByte, err := d.readByte()
	if err != nil {
		return nil, err
	}
	data, err := d.readN(n)
	if err != nil {
		return nil, err
	}

	if int8(codeByte) == wrappedValueExtCode {
		return msgpackDecode(data)
	}
	return ExtType{Code: int8(codeByte), Data: data}, nil
}

// normalizeKey mirrors pymax's TcpPayloadDecoder._normalize_key: integer
// keys become their decimal string, byte-string keys decode as UTF-8 or
// fall back to hex.
func normalizeKey(key any) string {
	switch k := key.(type) {
	case string:
		return k
	case int64:
		return strconv.FormatInt(k, 10)
	case []byte:
		if isValidUTF8(k) {
			return string(k)
		}
		return fmt.Sprintf("%x", k)
	default:
		return fmt.Sprint(k)
	}
}

func isValidUTF8(b []byte) bool {
	for i := 0; i < len(b); {
		r := b[i]
		switch {
		case r < 0x80:
			i++
		case r&0xE0 == 0xC0:
			if i+1 >= len(b) || b[i+1]&0xC0 != 0x80 {
				return false
			}
			i += 2
		case r&0xF0 == 0xE0:
			if i+2 >= len(b) || b[i+1]&0xC0 != 0x80 || b[i+2]&0xC0 != 0x80 {
				return false
			}
			i += 3
		case r&0xF8 == 0xF0:
			if i+3 >= len(b) || b[i+1]&0xC0 != 0x80 || b[i+2]&0xC0 != 0x80 || b[i+3]&0xC0 != 0x80 {
				return false
			}
			i += 4
		default:
			return false
		}
	}
	return true
}
