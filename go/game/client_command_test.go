package game

import (
	"bytes"
	"encoding/hex"
	"math"
	"strconv"
	"testing"

	"aionlightning/game/store"
	"aionlightning/wire"
)

func TestClientCommandLoc(t *testing.T) {
	s := testServer(nil)
	p := &player{character: &character{Character: &store.Character{
		WorldID: 1, X: 2, Y: -3.5, Z: 0,
	}}}
	c := &conn{s: s, player: p}
	p.conn = c
	var packets []*wire.Writer
	c.tap = func(w *wire.Writer) { packets = append(packets, w) }
	observer := &player{conn: &conn{}}
	observer.conn.tap = func(*wire.Writer) { t.Fatal("location must be private") }
	p.known = map[int32]*player{2: observer}
	handlers[cmClientCommandLoc](c, wire.NewReader(nil))
	// SM_SYSTEM_MESSAGE, CURRENT_LOCATION, four Java string parameters and final zero.
	want, err := hex.DecodeString("13000000000096820300043100000032002e00300000002d0033002e003500000030002e003000000000")
	if err != nil {
		t.Fatal(err)
	}
	if len(packets) != 1 || packets[0].Data[0] != smSystemMessage || !bytes.Equal(packets[0].Data[1:], want) {
		t.Fatalf("unexpected location packets: %v", packets)
	}
	c.player = nil
	c.clientCommandLoc(wire.NewReader(nil))
	if len(packets) != 1 {
		t.Fatal("absent player received location")
	}
}

func TestLocationCoordinate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value float32
		want  string
	}{
		{name: "zero", want: "0.0"},
		{name: "negative_zero", value: float32(math.Copysign(0, -1)), want: "-0.0"},
		{name: "integer", value: 1234, want: "1234.0"},
		{name: "fraction", value: -1234.125, want: "-1234.125"},
		{name: "small_decimal", value: 0.001, want: "0.001"},
		{name: "small_exponent", value: 0.0001, want: "1.0E-4"},
		{name: "smallest_float", value: math.SmallestNonzeroFloat32, want: "1.4E-45"},
		{name: "large_decimal", value: 9999999, want: "9999999.0"},
		{name: "large_exponent", value: 10000000, want: "1.0E7"},
		{name: "nan", value: float32(math.NaN()), want: "NaN"},
		{name: "infinity", value: float32(math.Inf(1)), want: "Infinity"},
		{name: "negative_infinity", value: float32(math.Inf(-1)), want: "-Infinity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := locationCoordinate(tc.value); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClientCommandRollRepliesAndBroadcasts(t *testing.T) {
	s := testServer(nil)
	p := &player{character: &character{Character: &store.Character{Name: "Aion"}}, known: map[int32]*player{}}
	observer := &player{}
	p.conn = &conn{s: s, player: p}
	observer.conn = &conn{s: s}
	p.known[2] = observer
	var selfPackets, observerPackets []*wire.Writer
	p.conn.tap = func(w *wire.Writer) { selfPackets = append(selfPackets, w) }
	observer.conn.tap = func(w *wire.Writer) { observerPackets = append(observerPackets, w) }

	request := wire.Packet(0)
	request.D(20)
	(&conn{s: s, player: p}).clientCommandRoll(wire.NewReader(request.Data[1:]))
	if len(selfPackets) != 1 || len(observerPackets) != 1 {
		t.Fatalf("got %d player replies and %d observer broadcasts", len(selfPackets), len(observerPackets))
	}
	readRoll := func(w *wire.Writer, wantCode int32) (string, string) {
		t.Helper()
		if w.Data[0] != smSystemMessage {
			t.Fatalf("packet opcode %#x, want system message", w.Data[0])
		}
		r := wire.NewReader(w.Data[1:])
		r.H()
		r.D()
		if code := r.D(); code != wantCode {
			t.Fatalf("message code %d, want %d", code, wantCode)
		}
		count := r.C()
		values := make([]string, count)
		for i := range values {
			values[i] = r.S()
		}
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		if count == 2 {
			return values[0], values[1]
		}
		return values[1], values[2]
	}
	value, maximum := readRoll(selfPackets[0], 1400126)
	if n, err := strconv.Atoi(value); err != nil || n < 1 || n > 20 || maximum != "20" {
		t.Fatalf("player roll (%q, %q) outside expected range", value, maximum)
	}
	value, maximum = readRoll(observerPackets[0], 1400127)
	if n, err := strconv.Atoi(value); err != nil || n < 1 || n > 20 || maximum != "20" {
		t.Fatalf("broadcast roll (%q, %q) outside expected range", value, maximum)
	}
}

func TestClientCommandRollHandlesBoundaryAndTruncation(t *testing.T) {
	s := testServer(nil)
	p := &player{character: &character{Character: &store.Character{Name: "Aion"}}}
	p.conn = &conn{s: s, player: p}
	var packets []*wire.Writer
	p.conn.tap = func(w *wire.Writer) { packets = append(packets, w) }
	c := &conn{s: s, player: p}
	for _, maximum := range []int32{0, -1} {
		request := wire.Packet(0)
		request.D(maximum)
		c.clientCommandRoll(wire.NewReader(request.Data[1:]))
	}
	if len(packets) != 2 {
		t.Fatalf("nonpositive bounds should produce safe minimum-roll replies; got %d", len(packets))
	}
	request := wire.Packet(0)
	request.D(math.MaxInt32)
	c.clientCommandRoll(wire.NewReader(request.Data[1:]))
	if len(packets) != 3 {
		t.Fatal("maximum int32 request did not produce a response")
	}
	c.clientCommandRoll(wire.NewReader(nil))
	if len(packets) != 3 {
		t.Fatal("truncated request produced a response")
	}
}
