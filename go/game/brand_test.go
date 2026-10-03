package game

import (
	"bytes"
	"testing"

	"aionlightning/wire"
)

func TestShowBrandRecipients(t *testing.T) {
	for _, kind := range []string{"group", "alliance", "both", "solo"} {
		t.Run(kind, func(t *testing.T) {
			s := testServer(nil)
			players := make([]*player, 4)
			packets := make([][][]byte, len(players))
			for i := range players {
				players[i] = &player{}
				players[i].conn = &conn{s: s, player: players[i]}
				players[i].conn.tap = func(w *wire.Writer) {
					packets[i] = append(packets[i], bytes.Clone(w.Data))
				}
			}
			// The sender is an ordinary member; the leader/captain is someone else.
			p := players[0]
			if kind == "group" || kind == "both" {
				p.group = &group{leader: players[1], members: players[:2]}
			}
			if kind == "alliance" || kind == "both" {
				p.alliance = &alliance{captain: players[2], members: []*player{p, players[2], {}}}
			}
			handler := handlers[cmShowBrand]
			if handler == nil || clientPacketStates[cmShowBrand] != inGame {
				t.Fatal("brand request must be registered for in-game clients")
			}
			request := wire.Packet(0)
			request.D(7)
			request.D(0x12345678)
			handler(p.conn, wire.NewReader(request.Data[1:]))
			// Java SM_SHOW_BRAND: opcode F7, H(1), D(brand), D(target).
			want := []byte{0xf7, 1, 0, 7, 0, 0, 0, 0x78, 0x56, 0x34, 0x12}
			counts := []int{0, 0, 0, 0}
			switch kind {
			case "group":
				counts = []int{1, 1, 0, 0}
			case "alliance":
				counts = []int{1, 0, 1, 0}
			case "both":
				counts = []int{2, 1, 1, 0}
			}
			for i, received := range packets {
				if len(received) != counts[i] {
					t.Fatalf("player %d received %d packets, want %d", i, len(received), counts[i])
				}
				for _, packet := range received {
					if !bytes.Equal(packet, want) {
						t.Fatalf("player %d received %x, want %x", i, packet, want)
					}
				}
			}
		})
	}
}

func TestShowBrandClearAndUnrestrictedIDs(t *testing.T) {
	s := testServer(nil)
	p := &player{}
	p.conn = &conn{s: s, player: p}
	p.group = &group{members: []*player{p}}
	var packets [][]byte
	p.conn.tap = func(w *wire.Writer) { packets = append(packets, bytes.Clone(w.Data)) }
	// Java passes IDs through without resolving the target or checking brand bounds.
	for _, ids := range [][2]int32{{0, 0}, {-1, -2147483648}, {2147483647, 2147483647}} {
		request := wire.Packet(0)
		request.D(ids[0])
		request.D(ids[1])
		p.conn.showBrand(wire.NewReader(request.Data[1:]))
		if len(packets) != 1 {
			t.Fatalf("IDs %v sent %d packets, want 1", ids, len(packets))
		}
		r := wire.NewReader(packets[0][1:])
		if header, brand, target := r.H(), r.D(), r.D(); header != 1 || brand != ids[0] || target != ids[1] {
			t.Fatalf("IDs %v produced header %d, brand %d, target %d", ids, header, brand, target)
		}
		packets = nil
	}
}

func TestShowBrandIgnoresTruncatedAndAbsentPlayer(t *testing.T) {
	s := testServer(nil)
	p := &player{}
	p.conn = &conn{s: s, player: p}
	p.group = &group{members: []*player{p}}
	var sent int
	p.conn.tap = func(*wire.Writer) { sent++ }
	for size := range 8 {
		p.conn.showBrand(wire.NewReader(make([]byte, size)))
	}
	p.conn.player = nil
	p.conn.showBrand(wire.NewReader(make([]byte, 8)))
	if sent != 0 {
		t.Fatalf("truncated or absent-player request sent %d packets", sent)
	}
}
