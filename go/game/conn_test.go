package game

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"aionlightning/crypt"
	"aionlightning/wire"
)

type packetConn struct {
	bytes.Buffer
	writes      int
	closed      bool
	writeErr    error
	deadlineErr error
	short       bool
}

func (c *packetConn) Read([]byte) (int, error) { return 0, io.EOF }
func (c *packetConn) Write(p []byte) (int, error) {
	c.writes++
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	if c.short {
		return len(p) - 1, nil
	}
	return c.Buffer.Write(p)
}
func (c *packetConn) Close() error                     { c.closed = true; return nil }
func (c *packetConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (c *packetConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (c *packetConn) SetDeadline(time.Time) error      { return nil }
func (c *packetConn) SetReadDeadline(time.Time) error  { return nil }
func (c *packetConn) SetWriteDeadline(time.Time) error { return c.deadlineErr }

func TestGameSendClosesOnWriteFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		peer packetConn
	}{
		{"write_error", packetConn{writeErr: errors.New("broken connection")}},
		{"short_write", packetConn{short: true}},
		{"deadline_error", packetConn{deadlineErr: errors.New("deadline failed")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cipher, _ := newGameCrypt(1234)
			c := &conn{netConn: &tc.peer, crypt: cipher}
			c.send(wire.Packet(smPong))
			if !c.closed || !tc.peer.closed {
				t.Fatal("failed write left the connection open")
			}
			writes := tc.peer.writes
			c.send(wire.Packet(smPong))
			if tc.peer.writes != writes {
				t.Fatal("sent another packet after a failed write")
			}
		})
	}
}

func TestGameSendFrameSizeBoundary(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
		want int
	}{
		{"maximum", wire.MaxPayloadSize - 2, 1},
		{"oversized", wire.MaxPayloadSize - 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := &packetConn{}
			cipher, _ := newGameCrypt(1234)
			c := &conn{netConn: peer, crypt: cipher}
			w := &wire.Writer{Data: make([]byte, tc.size)}
			w.Data[0] = smPong
			c.send(w)
			if peer.writes != tc.want {
				t.Fatalf("writes %d, want %d", peer.writes, tc.want)
			}
			if tc.want == 0 {
				if !c.closed || !peer.closed || cipher.enabled {
					t.Fatal("oversized packet was not rejected before encryption")
				}
				return
			}
			payload, err := wire.ReadFrame(&peer.Buffer)
			if err != nil || len(payload) != wire.MaxPayloadSize {
				t.Fatalf("maximum frame length %d, err %v", len(payload), err)
			}
		})
	}
}

func TestGameSendConcurrentPacketsRemainDecryptable(t *testing.T) {
	const key = 1234
	peer := &packetConn{}
	cipher, sent := newGameCrypt(key)
	c := &conn{netConn: peer, crypt: cipher}
	w := wire.Packet(smKey)
	w.D(sent)
	c.send(w)
	const count = 32
	var workers sync.WaitGroup
	for i := range count {
		workers.Go(func() {
			w := wire.Packet(smPong)
			w.D(int32(i))
			c.send(w)
		})
	}
	workers.Wait()
	first, err := wire.ReadFrame(&peer.Buffer)
	if err != nil || len(first) != 7 || crypt.DecodeServerOpcode(first[0]) != smKey {
		t.Fatalf("initial packet was not the clear key: %x, %v", first, err)
	}
	decrypt := crypt.NewGameCipher(key)
	seen := map[int32]bool{}
	for range count {
		payload, err := wire.ReadFrame(&peer.Buffer)
		if err != nil || len(payload) != 7 {
			t.Fatalf("invalid frame: %x, %v", payload, err)
		}
		decrypt.Decrypt(payload)
		if crypt.DecodeServerOpcode(payload[0]) != smPong || payload[1] != crypt.ServerPacketCode || payload[2] != ^payload[0] {
			t.Fatalf("cipher stream lost packet ordering: %x", payload)
		}
		id := wire.NewReader(payload[3:]).D()
		if id < 0 || id >= count || seen[id] {
			t.Fatalf("invalid or duplicate packet id %d", id)
		}
		seen[id] = true
	}
	if peer.Buffer.Len() != 0 {
		t.Fatal("unexpected trailing frame bytes")
	}
}
