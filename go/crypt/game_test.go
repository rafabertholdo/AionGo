package crypt

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// The expected values come from AL-Game's Crypt (Java) on the same inputs.

func keyed() *GameCipher {
	return &GameCipher{key: [8]byte{0x11, 0x22, 0x33, 0x44, 0xa1, 0x6c, 0x54, 0x87}}
}

func TestGameEncryptMatchesJava(t *testing.T) {
	c := keyed()
	for round, want := range []string{"1175135f9dd3bd30796f880f50e57d5fdc48c247ac", "27402764a7ea850f7065831b45f36a4ffa6de66c86"} {
		data := make([]byte, 21)
		for i := range data {
			data[i] = byte(i*13 + round)
		}
		c.Encrypt(data)
		if hex.EncodeToString(data) != want {
			t.Fatalf("round %d: got %x, want %s", round, data, want)
		}
	}
}

func TestGameDecryptMatchesJava(t *testing.T) {
	c := keyed()
	for round, want := range []string{"124a610cdbec05a90480486fde3fcbce36", "264c630ed5e207ab31864a6de001c9cc03"} {
		data := make([]byte, 17)
		for i := range data {
			data[i] = byte(i*29 + 3 + round)
		}
		c.Decrypt(data)
		if hex.EncodeToString(data) != want {
			t.Fatalf("round %d: got %x, want %s", round, data, want)
		}
	}
}

func TestGameEncryptThenDecryptRoundTrips(t *testing.T) {
	sender, receiver := keyed(), keyed()
	packet := []byte{0x09, ClientPacketCode, ^byte(0x09), 1, 2, 3, 4, 5}
	for range 3 {
		data := bytes.Clone(packet)
		sender.Encrypt(data)
		receiver.Decrypt(data)
		if !bytes.Equal(data, packet) {
			t.Fatalf("got %x", data)
		}
	}
}

func TestGameKeyExchangeMatchesJava(t *testing.T) {
	key := binary.LittleEndian.Uint32([]byte{0xc2, 0xfb, 0x50, 0x0f})
	c := NewGameCipher(key)
	if hex.EncodeToString(c.key[:]) != "c2fb500fa16c5487" || GameKeySent(key) != 45411354 {
		t.Fatalf("key %x, sent %d", c.key, GameKeySent(key))
	}
	if GameKeyReceived(GameKeySent(key)) != key {
		t.Fatal("the key doesn't come back from what SM_KEY sends")
	}
}

func TestServerOpcodeEncodingMatchesJava(t *testing.T) {
	got := []byte{EncodeServerOpcode(0x64), EncodeServerOpcode(0x00), EncodeServerOpcode(0xe4)}
	if hex.EncodeToString(got) != "fc407c" {
		t.Fatalf("got %x", got)
	}
	for op := range 256 {
		if DecodeServerOpcode(EncodeServerOpcode(byte(op))) != byte(op) {
			t.Fatalf("opcode %#x doesn't decode", op)
		}
	}
}
