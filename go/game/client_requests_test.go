package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"testing"
	"testing/synctest"
	"time"

	"aionlightning/game/store"
	"aionlightning/wire"
)

func TestSmallClientRequestRegistration(t *testing.T) {
	for _, opcode := range []byte{cmClientCommandLoc, cmReportPlayer, cmDisconnect, cmShowMap, cmQuestionnaire, cmChangeChannel} {
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
	for _, opcode := range []byte{cmShowMap, cmQuestionnaire, cmChangeChannel} {
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

func TestChangeChannel(t *testing.T) {
	d := staticDataOrSkip(t)
	if m := d.WorldMaps[210010000]; m == nil || m.TwinCount != 2 {
		t.Skip("Poeta isn't a map with two channels in the static data")
	}
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, tap := fighter(t, s, 1000)
		o := monster(t, s, 1005)
		s.spawn(p)
		if p.seen[o.id] == nil {
			t.Fatal("the player doesn't see the monster in its channel")
		}
		request := func(channel int32) {
			w := wire.Packet(0)
			w.D(channel)
			handlers[cmChangeChannel](p.conn, wire.NewReader(w.Data[1:]))
		}
		for _, channel := range []int32{-1, 0, 2} {
			request(channel)
			if p.instance != 0 || tap.count(smChannelInfo) != 0 {
				t.Fatalf("channel %d moved the player to instance %d", channel, p.instance)
			}
		}
		handlers[cmChangeChannel](p.conn, wire.NewReader([]byte{1, 0}))
		if p.instance != 0 {
			t.Fatal("a truncated request changed the channel")
		}
		request(1)
		if p.instance != 1 || tap.count(smChannelInfo) != 1 || tap.count(smPlayerSpawn) != 1 {
			t.Fatalf("instance %d, channel infos %d, spawns %d", p.instance, tap.count(smChannelInfo), tap.count(smPlayerSpawn))
		}
		s.spawn(p)
		if p.seen[o.id] != nil || inRange3D(p, o, 100) {
			t.Fatal("the player still sees the monster of the other channel")
		}
	})
}

func TestCustomSettings(t *testing.T) {
	s := testServer(nil)
	p := &player{character: &character{Character: &store.Character{ID: 0x20000}}, settings: &store.Settings{}}
	var sent [][]byte
	p.conn = &conn{s: s, player: p, tap: func(w *wire.Writer) { sent = append(sent, w.Data) }}
	watcher := &player{}
	watcher.conn = &conn{tap: func(w *wire.Writer) { sent = append(sent, w.Data) }}
	p.known = map[int32]*player{1: watcher}
	handlers[cmCustomSettings](p.conn, wire.NewReader([]byte{5, 0}))
	if len(sent) != 0 || p.settings.Display != 0 {
		t.Fatal("a truncated request changed the settings")
	}
	handlers[cmCustomSettings](p.conn, wire.NewReader([]byte{5, 0, 0x22, 0}))
	want := []byte{smCustomSettings, 0, 0, 2, 0, 1, 5, 0, 0x22, 0}
	if p.settings.Display != 5 || p.settings.Deny != 0x22 || len(sent) != 2 || !bytes.Equal(sent[0], want) || !bytes.Equal(sent[1], want) {
		t.Fatalf("settings %+v, sent %x, want %x to the player and the watcher", p.settings, sent, want)
	}
}

func TestDroppedClientLingers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := testServer(nil)
		watched := 0
		watcher := &player{}
		watcher.conn = &conn{tap: func(w *wire.Writer) {
			if w.Data[0] == smMove {
				watched++
			}
		}}
		newClient := func(accountID int32) *conn {
			p := &player{character: &character{Character: &store.Character{ID: 0x20000 + accountID}}, spawned: true, known: map[int32]*player{1: watcher}}
			c := &conn{s: s, player: p, account: &account{id: accountID}}
			p.conn = c
			return c
		}
		c := newClient(1)
		if !c.lingerLogout() || watched != 1 || len(s.lingering) != 1 {
			t.Fatalf("the dropped player didn't stay and stop: %d stops, %d lingering", watched, len(s.lingering))
		}
		other := newClient(2)
		other.lingerLogout()
		// leaveWorld needs a store: the test only checks when the logout runs.
		for _, d := range []*conn{c, other} {
			d.worldMu.Lock()
			d.player = nil
			d.worldMu.Unlock()
		}
		lingering := func() map[int32]*lingering {
			s.mu.Lock()
			defer s.mu.Unlock()
			return maps.Clone(s.lingering)
		}
		s.finishAccountLogouts(2)
		if l := lingering(); len(l) != 1 || l[0x20001] == nil {
			t.Fatalf("the account's re-login finished the wrong logouts: %v", l)
		}
		time.Sleep(logoutDelay - time.Second)
		synctest.Wait()
		if len(lingering()) != 1 {
			t.Fatal("the logout ran before its delay")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if len(lingering()) != 0 {
			t.Fatal("the logout didn't run after its delay")
		}
		s.clientWG.Wait()

		s.shuttingDown.Store(true)
		if newClient(3).lingerLogout() || len(lingering()) != 0 {
			t.Fatal("a player lingered while the server shuts down")
		}
	})
}
