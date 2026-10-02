package game

import (
	"bytes"
	"testing"

	"aionlightning/wire"
)

func TestClientPingRequest(t *testing.T) {
	var sent *wire.Writer
	c := &conn{tap: func(w *wire.Writer) { sent = w }}
	handlers[cmPingRequest](c, wire.NewReader(nil))
	if sent == nil || !bytes.Equal(sent.Data, []byte{smPingResponse, 4}) {
		t.Fatalf("ping response = %v, want opcode 0x7c and payload 0x04", sent)
	}
}
