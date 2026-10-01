// Package wire is the Aion servers' packet format: little-endian fields,
// UTF-16LE zero-terminated strings, and frames that start with their own
// 2-byte size.
package wire

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"unicode/utf16"
)

// ErrShortPacket is a packet that ends before a field it should have.
var ErrShortPacket = errors.New("packet ends early")

// Reader reads a packet's fields in order, remembering the first error.
type Reader struct {
	Data []byte
	Err  error
}

// NewReader reads data.
func NewReader(data []byte) *Reader {
	return &Reader{Data: data}
}

func (r *Reader) take(n int) []byte {
	if r.Err != nil || n < 0 || len(r.Data) < n {
		r.Err = ErrShortPacket
		return make([]byte, max(n, 0))
	}
	b := r.Data[:n]
	r.Data = r.Data[n:]
	return b
}

// C reads a byte.
func (r *Reader) C() byte { return r.take(1)[0] }

// H reads a 16-bit value.
func (r *Reader) H() uint16 { return binary.LittleEndian.Uint16(r.take(2)) }

// D reads a 32-bit value.
func (r *Reader) D() int32 { return int32(binary.LittleEndian.Uint32(r.take(4))) }

// Q reads a 64-bit value.
func (r *Reader) Q() int64 { return int64(binary.LittleEndian.Uint64(r.take(8))) }

// F reads a 32-bit float.
func (r *Reader) F() float32 { return math.Float32frombits(binary.LittleEndian.Uint32(r.take(4))) }

// B reads n bytes into a new slice.
func (r *Reader) B(n int) []byte { return append([]byte(nil), r.take(n)...) }

// S reads a UTF-16LE string ending with a zero character.
func (r *Reader) S() string {
	var units []uint16
	for {
		u := r.H()
		if u == 0 || r.Err != nil {
			return string(utf16.Decode(units))
		}
		units = append(units, u)
	}
}

// Remaining is how many bytes are left.
func (r *Reader) Remaining() int { return len(r.Data) }

// Writer builds a packet's fields.
type Writer struct{ Data []byte }

// Packet starts a packet with its opcode.
func Packet(opcode byte) *Writer { return &Writer{Data: []byte{opcode}} }

// C writes a byte.
func (w *Writer) C(v byte) { w.Data = append(w.Data, v) }

// H writes a 16-bit value.
func (w *Writer) H(v uint16) { w.Data = binary.LittleEndian.AppendUint16(w.Data, v) }

// D writes a 32-bit value.
func (w *Writer) D(v int32) { w.Data = binary.LittleEndian.AppendUint32(w.Data, uint32(v)) }

// Q writes a 64-bit value.
func (w *Writer) Q(v int64) { w.Data = binary.LittleEndian.AppendUint64(w.Data, uint64(v)) }

// F writes a 32-bit float.
func (w *Writer) F(v float32) { w.Data = binary.LittleEndian.AppendUint32(w.Data, math.Float32bits(v)) }

// B writes bytes.
func (w *Writer) B(v []byte) { w.Data = append(w.Data, v...) }

// Bool writes 1 or 0.
func (w *Writer) Bool(v bool) {
	if v {
		w.C(1)
	} else {
		w.C(0)
	}
}

// S writes a UTF-16LE string ending with a zero character.
func (w *Writer) S(v string) {
	for _, u := range utf16.Encode([]rune(v)) {
		w.H(u)
	}
	w.H(0)
}

// ReadFrame reads one frame: a 2-byte size that counts itself, then the payload.
func ReadFrame(r io.Reader) ([]byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	size := int(binary.LittleEndian.Uint16(header[:]))
	if size < 2 {
		return nil, io.ErrUnexpectedEOF
	}
	payload := make([]byte, size-2)
	_, err := io.ReadFull(r, payload)
	return payload, err
}

// Frame prefixes payload with its size, for an unencrypted link.
func Frame(payload []byte) []byte {
	return append(binary.LittleEndian.AppendUint16(nil, uint16(len(payload)+2)), payload...)
}
