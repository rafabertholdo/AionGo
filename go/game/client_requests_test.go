package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"aionlightning/game/store"
	"aionlightning/wire"
)

func TestSmallClientRequestRegistration(t *testing.T) {
	for _, opcode := range []byte{cmClientCommandLoc, cmReportPlayer, cmDisconnect, cmShowMap, cmQuestionnaire} {
		t.Run(fmt.Sprintf("%02x", opcode), func(t *testing.T) {
			if handlers[opcode] == nil || clientPacketStates[opcode] != inGame {
				t.Fatal("request must be registered for in-game clients")
			}
		})
	}
}

func TestReportPlayerAudit(t *testing.T) {
	for _, name := range []string{"Reported", "", "Élyos😀\nforged audit"} {
		t.Run(name, func(t *testing.T) {
			var logs bytes.Buffer
			s := testServer(nil)
			s.log = slog.New(slog.NewJSONHandler(&logs, nil))
			p := &player{character: &character{Character: &store.Character{Name: "Reporter"}}}
			c := &conn{s: s, player: p}
			p.conn = c
			c.tap = func(*wire.Writer) { t.Fatal("report must not send a reply") }
			request := wire.Packet(0)
			request.C(255)
			request.S(name)
			handlers[cmReportPlayer](c, wire.NewReader(request.Data[1:]))
			var record map[string]any
			if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			if record["reporter"] != "Reporter" || record["reported"] != name || record["msg"] != "[AUDIT] player report" {
				t.Fatalf("unexpected audit: %v", record)
			}
		})
	}
}

func TestReportPlayerRejectsTruncationAndAbsentPlayer(t *testing.T) {
	var logs bytes.Buffer
	s := testServer(nil)
	s.log = slog.New(slog.NewJSONHandler(&logs, nil))
	p := &player{character: &character{Character: &store.Character{Name: "Reporter"}}}
	c := &conn{s: s, player: p}
	request := wire.Packet(0)
	request.C(0)
	request.S("Target")
	for size := range len(request.Data) - 1 {
		t.Run(fmt.Sprintf("length_%d", size), func(t *testing.T) {
			r := wire.NewReader(request.Data[1 : 1+size])
			c.reportPlayer(r)
			if r.Err == nil || logs.Len() != 0 {
				t.Fatal("truncated report must fail parsing without an audit")
			}
		})
	}
	c.player = nil
	c.reportPlayer(wire.NewReader(request.Data[1:]))
	if logs.Len() != 0 {
		t.Fatal("absent player generated an audit")
	}
}

func TestClientDisconnect(t *testing.T) {
	for _, tc := range []struct {
		name       string
		payload    []byte
		wantClosed bool
	}{
		{name: "zero", payload: []byte{0}, wantClosed: true},
		{name: "one", payload: []byte{1}},
		{name: "maximum", payload: []byte{255}},
		{name: "truncated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := &packetConn{}
			p := &player{}
			c := &conn{netConn: peer, player: p}
			c.tap = func(*wire.Writer) { t.Fatal("disconnect must not send a final packet") }
			c.clientDisconnect(wire.NewReader(tc.payload))
			if peer.closed != tc.wantClosed || c.closed != tc.wantClosed || peer.writes != 0 {
				t.Fatalf("connection closed=%v, peer closed=%v, writes=%d", c.closed, peer.closed, peer.writes)
			}
			if c.player != p {
				t.Fatal("handler must leave player cleanup to the reader's deferred logout")
			}
		})
	}
}

func TestInertClientRequests(t *testing.T) {
	for _, opcode := range []byte{cmShowMap, cmQuestionnaire} {
		t.Run(fmt.Sprintf("%02x", opcode), func(t *testing.T) {
			c := &conn{}
			c.tap = func(*wire.Writer) { t.Fatal("inert request sent a packet") }
			for size := range 13 {
				handlers[opcode](c, wire.NewReader(make([]byte, size)))
			}
			if c.closed {
				t.Fatal("inert request closed the connection")
			}
		})
	}
}
