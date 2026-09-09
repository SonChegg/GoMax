package tcp

import (
	"bytes"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

const defaultMaxOutput = 5 * 1024 * 1024

// lz4BlockCompression implements the small hand-rolled LZ4 block variant
// used by pymax (protocol.tcp.compression.Lz4BlockCompression). It is not a
// full LZ4 implementation; it matches pymax's simplified encoder/decoder
// byte for byte.
type lz4BlockCompression struct{}

func (lz4BlockCompression) decompress(src []byte, maxOutput int) ([]byte, error) {
	dst := make([]byte, 0, len(src)*2)
	pos := 0

	for pos < len(src) {
		token := src[pos]
		pos++

		litLen := int(token >> 4)
		if litLen == 15 {
			for pos < len(src) {
				b := src[pos]
				pos++
				litLen += int(b)
				if b != 255 {
					break
				}
			}
		}

		if litLen > 0 {
			if pos+litLen > len(src) {
				return nil, fmt.Errorf("lz4: literal length out of bounds")
			}
			dst = append(dst, src[pos:pos+litLen]...)
			pos += litLen
			if len(dst) > maxOutput {
				return nil, fmt.Errorf("lz4: output too large")
			}
		}

		if pos >= len(src) {
			break
		}

		if pos+1 >= len(src) {
			return nil, fmt.Errorf("lz4: incomplete offset")
		}

		offset := int(src[pos]) | (int(src[pos+1]) << 8)
		pos += 2

		if offset == 0 {
			return nil, fmt.Errorf("lz4: zero offset")
		}

		matchLen := int(token&0x0F) + 4
		if token&0x0F == 0x0F {
			for pos < len(src) {
				b := src[pos]
				pos++
				matchLen += int(b)
				if b != 255 {
					break
				}
			}
		}

		matchPos := len(dst) - offset
		if matchPos < 0 {
			return nil, fmt.Errorf("lz4: match out of bounds")
		}

		for i := 0; i < matchLen; i++ {
			dst = append(dst, dst[matchPos+(i%offset)])
		}

		if len(dst) > maxOutput {
			return nil, fmt.Errorf("lz4: output too large")
		}
	}

	return dst, nil
}

func (lz4BlockCompression) compress(src []byte) []byte {
	dst := make([]byte, 0, len(src))
	pos := 0

	for pos < len(src) {
		litStart := pos
		for pos < len(src) && pos-litStart < 15 {
			pos++
		}

		litLen := pos - litStart
		token := byte((litLen << 4) & 0xF0)

		matchOffset := 0
		matchLen := 0

		lowerBound := litStart - 65535
		if lowerBound < 0 {
			lowerBound = 0
		}
		for i := lowerBound; i < litStart; i++ {
			j, k := i, litStart
			for j < i+65535 && k < len(src) && src[j] == src[k] {
				j++
				k++
			}
			if j-i > matchLen {
				matchOffset = litStart - i
				matchLen = j - i
			}
		}

		if matchLen >= 4 {
			token |= byte((matchLen - 4) & 0x0F)
			dst = append(dst, token)
			dst = append(dst, src[litStart:litStart+litLen]...)
			dst = append(dst, byte(matchOffset&0xFF), byte((matchOffset>>8)&0xFF))
			pos += matchLen
		} else {
			token |= byte(litLen & 0x0F)
			dst = append(dst, token)
			dst = append(dst, src[litStart:litStart+litLen]...)
		}
	}

	return dst
}

// zstdCompression decompresses zstd-compressed TCP payloads (compression
// factor flag 0xFF), a port of pymax's protocol.tcp.compression.ZstdCompression.
type zstdCompression struct{}

func (zstdCompression) decompress(src []byte, maxOutput int) ([]byte, error) {
	decoder, err := zstd.NewReader(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("zstd: failed to decompress payload: %w", err)
	}
	defer decoder.Close()

	limited := io.LimitReader(decoder, int64(maxOutput)+1)
	result, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("zstd: failed to decompress payload: %w", err)
	}
	if len(result) > maxOutput {
		return nil, fmt.Errorf("zstd: output too large")
	}
	return result, nil
}
