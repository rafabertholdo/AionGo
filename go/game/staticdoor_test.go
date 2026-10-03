package game

import (
	"bytes"
	"testing"

	"aionlightning/game/store"
	"aionlightning/wire"
)

func TestOpenStaticDoorBroadcastsSwitchEmotion(t *testing.T) {
	s := testServer(nil)
	p := &player{character: &character{Character: &store.Character{ID: 1}}, known: map[int32]*player{}}
	observer := &player{}
	p.conn = &conn{s: s, player: p}
	observer.conn = &conn{s: s}
	p.known[2] = observer
	var playerPackets, observerPackets []*wire.Writer
	p.conn.tap = func(w *wire.Writer) { playerPackets = append(playerPackets, w) }
	observer.conn.tap = func(w *wire.Writer) { observerPackets = append(observerPackets, w) }

	request := wire.Packet(0)
	request.D(0x12345678)
	(&conn{s: s, player: p}).openStaticDoor(wire.NewReader(request.Data[1:]))
	want := wire.Packet(smEmotion)
	want.D(0x12345678)
	want.C(emoteSwitchDoor)
	want.H(9)
	want.D(0)
	if len(playerPackets) != 1 || !bytes.Equal(playerPackets[0].Data, want.Data) {
		t.Fatalf("player received %d packets; want switch-door packet %x", len(playerPackets), want.Data)
	}
	if len(observerPackets) != 1 || !bytes.Equal(observerPackets[0].Data, want.Data) {
		t.Fatalf("observer received %d packets; want switch-door packet %x", len(observerPackets), want.Data)
	}
}

func TestOpenStaticDoorIgnoresTruncatedRequest(t *testing.T) {
	s := testServer(nil)
	p := &player{character: &character{Character: &store.Character{ID: 1}}}
	p.conn = &conn{s: s, player: p}
	var packets int
	p.conn.tap = func(*wire.Writer) { packets++ }
	(&conn{s: s, player: p}).openStaticDoor(wire.NewReader([]byte{1, 2, 3}))
	if packets != 0 {
		t.Fatalf("truncated request sent %d packets", packets)
	}
}
