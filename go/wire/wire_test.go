package wire

import (
	"bytes"
	"errors"
	"io"
	"math"
	"testing"
)

func TestReaderRoundTrip(t *testing.T) {
	w := &Writer{}
	w.C(123)
	w.H(456)
	w.D(-789)
	w.Q(math.MinInt64)
	w.F(1.25)
	w.B([]byte{1, 2, 3})
	w.S("Aion 🐉")
	r := NewReader(w.Data)
	if r.C() != 123 || r.H() != 456 || r.D() != -789 || r.Q() != math.MinInt64 || r.F() != 1.25 {
		t.Fatal("numeric fields did not round trip")
	}
	if !bytes.Equal(r.B(3), []byte{1, 2, 3}) || r.S() != "Aion 🐉" || r.Err != nil || r.Remaining() != 0 {
		t.Fatalf("byte/string fields did not round trip: %v, remaining %d", r.Err, r.Remaining())
	}
}

func TestReaderRejectsInvalidByteCounts(t *testing.T) {
	for _, tc := range []struct {
		name string
		n    int
	}{
		{"negative", -1},
		{"outside_frame", MaxPayloadSize + 1},
		{"maximum_integer", math.MaxInt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewReader([]byte{1, 2, 3})
			if got := r.B(tc.n); len(got) != 0 || !errors.Is(r.Err, ErrShortPacket) || r.Remaining() != 3 {
				t.Fatalf("invalid byte count returned %d bytes, err %v, remaining %d", len(got), r.Err, r.Remaining())
			}
		})
	}
}

func TestReaderTruncatedFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		read func(*Reader)
		size int
	}{
		{"byte", func(r *Reader) { r.C() }, 1},
		{"short", func(r *Reader) { r.H() }, 2},
		{"integer", func(r *Reader) { r.D() }, 4},
		{"long", func(r *Reader) { r.Q() }, 8},
		{"float", func(r *Reader) { r.F() }, 4},
		{"bytes", func(r *Reader) { r.B(5) }, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for length := 0; length < tc.size; length++ {
				r := NewReader(make([]byte, length))
				tc.read(r)
				if !errors.Is(r.Err, ErrShortPacket) || r.Remaining() != length {
					t.Fatalf("length %d: err %v, remaining %d", length, r.Err, r.Remaining())
				}
				// Reading again must remain safe after the first failure.
				tc.read(r)
			}
		})
	}
}

func TestReaderTruncatedString(t *testing.T) {
	for _, input := range [][]byte{{}, {'A'}, {'A', 0}, {'A', 0, 0}} {
		r := NewReader(input)
		r.S()
		if !errors.Is(r.Err, ErrShortPacket) {
			t.Fatalf("unterminated string %x: err %v", input, r.Err)
		}
	}
}

func TestReaderPreservesFirstError(t *testing.T) {
	want := errors.New("first error")
	r := &Reader{Err: want}
	r.B(math.MaxInt)
	r.D()
	if !errors.Is(r.Err, want) {
		t.Fatalf("first error replaced: %v", r.Err)
	}
}

func TestFrameBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
	}{
		{"empty", 0},
		{"ordinary", 16},
		{"maximum", MaxPayloadSize},
		{"oversized", MaxPayloadSize + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := bytes.Repeat([]byte{0x55}, tc.size)
			frame, err := Frame(payload)
			if tc.size > MaxPayloadSize {
				if !errors.Is(err, ErrFrameTooLarge) || frame != nil {
					t.Fatalf("oversized payload: frame length %d, err %v", len(frame), err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := ReadFrame(bytes.NewReader(frame))
			if err != nil || !bytes.Equal(got, payload) || len(frame) != tc.size+2 {
				t.Fatalf("frame round trip: payload length %d, err %v", len(got), err)
			}
		})
	}
}

func TestReadFrameRejectsMalformedLengths(t *testing.T) {
	for _, input := range [][]byte{{}, {2}, {0, 0}, {1, 0}, {4, 0, 1}} {
		if _, err := ReadFrame(bytes.NewReader(input)); err == nil {
			t.Fatalf("accepted malformed frame %x", input)
		}
	}
}

type failingWriter struct {
	err   error
	short bool
	calls int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.short {
		return len(p) - 1, nil
	}
	return 0, w.err
}

func TestWriteFrameErrors(t *testing.T) {
	want := errors.New("write failed")
	for _, tc := range []struct {
		name    string
		writer  failingWriter
		payload []byte
		want    error
		calls   int
	}{
		{"write_failure", failingWriter{err: want}, []byte{1}, want, 1},
		{"short_write", failingWriter{short: true}, []byte{1}, io.ErrShortWrite, 1},
		{"oversized", failingWriter{}, make([]byte, MaxPayloadSize+1), ErrFrameTooLarge, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := WriteFrame(&tc.writer, tc.payload)
			if !errors.Is(err, tc.want) || tc.writer.calls != tc.calls {
				t.Fatalf("err %v, calls %d", err, tc.writer.calls)
			}
		})
	}
}

func FuzzFrameRoundTrip(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1, 2, 3})
	f.Add(bytes.Repeat([]byte{0xff}, MaxPayloadSize))
	f.Fuzz(func(t *testing.T, payload []byte) {
		frame, err := Frame(payload)
		if len(payload) > MaxPayloadSize {
			if !errors.Is(err, ErrFrameTooLarge) || frame != nil {
				t.Fatal("oversized frame accepted")
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		got, err := ReadFrame(bytes.NewReader(frame))
		if err != nil || !bytes.Equal(payload, got) {
			t.Fatalf("round trip failed: %v", err)
		}
	})
}

func FuzzReader(f *testing.F) {
	f.Add([]byte{}, int64(-1))
	f.Add([]byte{1, 2, 3}, int64(math.MaxInt64))
	f.Add([]byte{0x41, 0, 0, 0}, int64(2))
	f.Fuzz(func(t *testing.T, input []byte, count int64) {
		r := NewReader(input)
		r.B(int(count))
		r.C()
		r.H()
		r.D()
		r.Q()
		r.F()
		r.S()
		if r.Remaining() > len(input) {
			t.Fatal("reader grew its input")
		}
	})
}
