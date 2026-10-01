package crypt

import "encoding/binary"

// gameStaticKey is mixed into every byte of every game packet.
var gameStaticKey = []byte("nKO/WctQ0AVLbpzfBkS6NevDYT8ourG5CRlmdjyJ72aswx4EPq1UgZhFMXH?3iI9")

// Every game packet starts with its opcode, the sender's code, and the opcode's complement.
const (
	ClientPacketCode = 0x55
	ServerPacketCode = 0x50
)

// GameCipher is one direction of a game connection's encryption: each byte
// XORed with the static key, the connection's key and the previous ciphertext
// byte, and the key advanced by each packet's size.
type GameCipher struct {
	key [8]byte
}

// NewGameCipher is a direction's cipher for the connection key SM_KEY agrees on.
func NewGameCipher(key uint32) *GameCipher {
	c := &GameCipher{}
	binary.LittleEndian.PutUint32(c.key[:], key)
	copy(c.key[4:], []byte{0xa1, 0x6c, 0x54, 0x87})
	return c
}

// GameKeySent is the value SM_KEY sends the client for a connection key.
func GameKeySent(key uint32) int32 {
	return int32((key ^ 0xcd92e451) + 0x3ff2cc87)
}

// GameKeyReceived is the connection key an SM_KEY value stands for.
func GameKeyReceived(sent int32) uint32 {
	return (uint32(sent) - 0x3ff2cc87) ^ 0xcd92e451
}

// Encrypt encrypts a packet in place.
func (c *GameCipher) Encrypt(data []byte) {
	if len(data) == 0 {
		return
	}
	data[0] ^= c.key[0]
	prev := data[0]
	for i := 1; i < len(data); i++ {
		data[i] ^= gameStaticKey[i&63] ^ c.key[i&7] ^ prev
		prev = data[i]
	}
	c.advance(len(data))
}

// Decrypt decrypts a packet in place.
func (c *GameCipher) Decrypt(data []byte) {
	if len(data) == 0 {
		return
	}
	prev := data[0]
	data[0] ^= c.key[0]
	for i := 1; i < len(data); i++ {
		curr := data[i]
		data[i] ^= gameStaticKey[i&63] ^ c.key[i&7] ^ prev
		prev = curr
	}
	c.advance(len(data))
}

func (c *GameCipher) advance(size int) {
	binary.LittleEndian.PutUint64(c.key[:], binary.LittleEndian.Uint64(c.key[:])+uint64(size))
}

// EncodeServerOpcode is how a server opcode appears on the wire, and
// DecodeServerOpcode the opcode from it.
func EncodeServerOpcode(op byte) byte { return (op + 0xae) ^ 0xee }

func DecodeServerOpcode(b byte) byte { return (b ^ 0xee) - 0xae }
