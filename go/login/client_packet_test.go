package login

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"aionlightning/crypt"
	"aionlightning/wire"
)

func TestLoginSendRejectsOversizeBeforeEncryption(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
	}{
		{"payload_oversized", wire.MaxPayloadSize + 1},
		{"padding_oversized", wire.MaxPayloadSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, peer := net.Pipe()
			defer server.Close()
			defer peer.Close()
			if err := server.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			engine := crypt.NewEngine(make([]byte, 16))
			before := engine.EncryptedSize(1)
			c := &client{conn: server, engine: engine}
			c.send(&wire.Writer{Data: make([]byte, tc.size)})
			if engine.EncryptedSize(1) != before {
				t.Fatal("oversized packet changed the encryption phase")
			}
			if _, err := peer.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
				t.Fatalf("oversized packet did not close the connection: %v", err)
			}
		})
	}
}
