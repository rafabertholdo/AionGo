package game

import (
	"bytes"
	"net"
	"testing"
	"time"

	"aionlightning/crypt"
	"aionlightning/wire"
)

func TestShutdownKicksAllConnectionsBeforeWaitingForSaves(t *testing.T) {
	s := testServer(nil)
	s.clients = map[net.Conn]*conn{}
	kicked := make(chan struct{}, 3)
	var peers []*packetConn
	for _, state := range []stateSet{inConnected, inAuthed, inGame} {
		peer := &packetConn{}
		cipher, _ := newGameCrypt(1234)
		cipher.enabled = true
		s.clients[peer] = &conn{s: s, netConn: peer, crypt: cipher, state: state, tap: func(w *wire.Writer) {
			if w.Data[0] == smQuitResponse {
				kicked <- struct{}{}
			}
		}}
		peers = append(peers, peer)
	}
	// Reader cleanup owns this wait group until its character saves finish.
	s.clientWG.Add(1)
	done := make(chan struct{})
	go func() { s.CloseClients(); close(done) }()
	for range peers {
		select {
		case <-kicked:
		case <-time.After(time.Second):
			t.Fatal("shutdown did not kick all connected clients")
		}
	}
	select {
	case <-done:
		t.Fatal("shutdown returned with a pending character save")
	default:
	}
	s.clientWG.Done()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish after reader cleanup")
	}
	for _, peer := range peers {
		if !peer.closed {
			t.Fatal("shutdown left a client connected")
		}
		payload, err := wire.ReadFrame(&peer.Buffer)
		if err != nil {
			t.Fatal(err)
		}
		crypt.NewGameCipher(1234).Decrypt(payload)
		op := crypt.EncodeServerOpcode(smQuitResponse)
		want := []byte{op, crypt.ServerPacketCode, ^op, 1, 0, 0, 0, 0}
		if !bytes.Equal(payload, want) {
			t.Fatalf("final packet %x, want Aion 1.9 kick %x", payload, want)
		}
		if peer.Buffer.Len() != 0 {
			t.Fatal("packets followed the final kick")
		}
	}
}
