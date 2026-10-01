package crypt

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"testing"
)

// The expected values come from running AL-Login's own BlowfishCipher,
// CryptEngine and EncryptedRSAKeyPair (Java) on the same inputs.

func seq(n int, start byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = start + byte(i*7)
	}
	return b
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBlowfishMatchesJava(t *testing.T) {
	data := seq(32, 0)
	NewBlowfish(initialKey).Encrypt(data)
	if want := unhex(t, "e2366f5acc94df8643e9070a1db2bc94842db4ac3bdaef83ce8df8e643680517"); !bytes.Equal(data, want) {
		t.Fatalf("initial key: got %x, want %x", data, want)
	}

	key := seq(16, 3)
	data = seq(64, 11)
	NewBlowfish(key).Encrypt(data)
	if want := unhex(t, "123577d0b3aa665ef6b9ad2439156f3e41fa3b739e6d3eb10aef21f337040d1752d47abe530ff65065c333406517ab93412745b2dfd8832685976bb888340b2c"); !bytes.Equal(data, want) {
		t.Fatalf("connection key: got %x, want %x", data, want)
	}
	NewBlowfish(key).Decrypt(data)
	if !bytes.Equal(data, seq(64, 11)) {
		t.Fatalf("decrypt didn't restore the plaintext: %x", data)
	}
}

func TestEngineMatchesJavaAfterTheFirstPacket(t *testing.T) {
	engine := NewEngine(seq(16, 3))
	first := make([]byte, engine.EncryptedSize(5))
	engine.Encrypt(first, 5)

	second := make([]byte, engine.EncryptedSize(11))
	copy(second, seq(11, 40))
	size := engine.Encrypt(second, 11)
	if want := unhex(t, "bfed3f155ba30012b78f9802951a7aae"); size != 16 || !bytes.Equal(second[:size], want) {
		t.Fatalf("got %d %x, want 16 %x", size, second[:size], want)
	}
}

func TestEngineRoundTrip(t *testing.T) {
	key := NewBlowfishKey()
	server := NewEngine(key)
	first := make([]byte, server.EncryptedSize(3))
	server.Encrypt(first, 3)

	payload := []byte("CM_LOGIN payload")
	data := make([]byte, server.EncryptedSize(len(payload)))
	copy(data, payload)
	size := server.Encrypt(data, len(payload))

	client := NewBlowfish(key)
	client.Decrypt(data[:size])
	if !bytes.HasPrefix(data, payload) {
		t.Fatalf("round trip failed: %x", data[:size])
	}
}

func TestClientChecksumLeavesOutTheLastWord(t *testing.T) {
	// A client packet: payload, the checksum of what precedes it, then a word the check ignores.
	data := make([]byte, 24)
	copy(data, "CM_AUTH_GG")
	appendChecksum(data[:20])
	copy(data[20:], []byte{0xde, 0xad, 0xbe, 0xef})
	if !verifyChecksum(data) {
		t.Fatal("rejected a packet whose last word is filler")
	}
	data[0] ^= 1
	if verifyChecksum(data) {
		t.Fatal("a changed byte passed the checksum")
	}
}

func TestFirstPacketUnscrambles(t *testing.T) {
	payload := seq(40, 5)
	engine := NewEngine(seq(16, 3))
	data := make([]byte, engine.EncryptedSize(len(payload)))
	copy(data, payload)
	size := engine.Encrypt(data, len(payload))

	// What the client does: the initial key, then undo the XOR pass from the key stored after it.
	NewBlowfish(initialKey).Decrypt(data[:size])
	stop := size - 8
	key := uint32(data[stop]) | uint32(data[stop+1])<<8 | uint32(data[stop+2])<<16 | uint32(data[stop+3])<<24
	for pos := stop - 4; pos >= 4; pos -= 4 {
		word := uint32(data[pos]) | uint32(data[pos+1])<<8 | uint32(data[pos+2])<<16 | uint32(data[pos+3])<<24
		word ^= key
		key -= word
		data[pos], data[pos+1], data[pos+2], data[pos+3] = byte(word), byte(word>>8), byte(word>>16), byte(word>>24)
	}
	if !bytes.Equal(data[:len(payload)], payload) {
		t.Fatalf("got %x, want %x", data[:len(payload)], payload)
	}
}

func TestScrambleModulusMatchesJava(t *testing.T) {
	n := new(big.Int).SetBytes(unhex(t, "0081888f969da4abb2b9c0c7ced5dce3eaf1f8ff060d141b222930373e454c535a61686f767d848b9299a0a7aeb5bcc3cad1d8dfe6edf4fb020910171e252c333a41484f565d646b727980878e959ca3aab1b8bfc6cdd4dbe2e9f0f7fe050c131a21282f363d444b525960676e757c838a91989fa6adb4bbc2c9d0d7dee5ecf3fb"))
	if got, want := ScrambleModulus(n), unhex(t, "ddebe5e7c0c0c0c0c0404040401d2b25a74040c0c0c0c0c0c0c0c0c0404040404040404040c0c0c0c0c0c0c0c0c0404040404040404040c0c0c0c0c0c0c0c0c19ca3aab19da4abb2b9c0c7ced59ca3aa31f8ff060d141b222930373e454c535a61686f767d848b9299a0a7aeb5bcc3cad1d8dfe6edf4fb020910171e252c333a"); !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestRSADecryptInvertsEncrypt(t *testing.T) {
	pair, err := NewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	block := make([]byte, 128)
	copy(block[64:], "admin")
	copy(block[96:], "secret")
	m := new(big.Int).SetBytes(block)
	c := new(big.Int).Exp(m, big.NewInt(int64(pair.Private.E)), pair.Private.N).FillBytes(make([]byte, 128))
	if got := pair.Decrypt(c); !bytes.Equal(got, block) {
		t.Fatalf("got %x", got)
	}
}
