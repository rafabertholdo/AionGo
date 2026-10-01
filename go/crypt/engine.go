package crypt

import "encoding/binary"

// initialKey enciphers the first packet, SM_INIT, which carries the connection's own key.
var initialKey = []byte{0x6b, 0x60, 0xcb, 0x5b, 0x82, 0xce, 0x90, 0xb1, 0xcc, 0x2b, 0x6c, 0x55, 0x6c, 0x6c, 0x6c, 0x6c}

// Engine encrypts a connection's outgoing packets and decrypts its incoming
// ones. The first outgoing packet is scrambled with a random XOR key and
// enciphered with the fixed initial key; every packet after it, both ways,
// uses the connection's key and ends with a checksum.
type Engine struct {
	cipher  *Blowfish
	key     []byte
	started bool
}

// NewEngine returns the engine for a connection whose key, sent in SM_INIT, is key.
func NewEngine(key []byte) *Engine {
	return &Engine{cipher: NewBlowfish(initialKey), key: key}
}

// EncryptedSize is the size Encrypt makes a payload of n bytes, which the
// buffer passed to it must have room for.
func (e *Engine) EncryptedSize(n int) int {
	n += 4
	if !e.started {
		n += 4
	}
	return n + 8 - n%8
}

// Encrypt pads, checksums or scrambles, and enciphers the payload in
// data[:n], and returns its new size. data must be EncryptedSize(n) long, zeroed past n.
func (e *Engine) Encrypt(data []byte, n int) int {
	size := e.EncryptedSize(n)
	if !e.started {
		xorPass(data[:size], randomUint32())
		e.cipher.Encrypt(data[:size])
		e.cipher = NewBlowfish(e.key)
		e.started = true
		return size
	}
	appendChecksum(data[:size])
	e.cipher.Encrypt(data[:size])
	return size
}

// Decrypt deciphers a client packet in place and reports whether its checksum holds.
func (e *Engine) Decrypt(data []byte) bool {
	e.cipher.Decrypt(data)
	return verifyChecksum(data)
}

// verifyChecksum reports whether the 32-bit little-endian words of data, all
// but the last, XOR to zero. The client ends each packet with a word of padding
// the check leaves out, as AL-Login's does.
func verifyChecksum(data []byte) bool {
	if len(data)%4 != 0 || len(data) <= 4 {
		return false
	}
	var sum uint32
	for i := 0; i < len(data)-4; i += 4 {
		sum ^= binary.LittleEndian.Uint32(data[i:])
	}
	return sum == 0
}

// appendChecksum writes the XOR of data's words, all but the last, into its last word.
func appendChecksum(data []byte) {
	var sum uint32
	end := len(data) - 4
	for i := 0; i < end; i += 4 {
		sum ^= binary.LittleEndian.Uint32(data[i:])
	}
	binary.LittleEndian.PutUint32(data[end:], sum)
}

// xorPass scrambles the first packet's words from the fifth byte to 8 bytes
// before the end, each with the running sum of key and the words before it,
// and stores the final sum after them.
func xorPass(data []byte, key uint32) {
	stop := len(data) - 8
	pos := 4
	for ; pos < stop; pos += 4 {
		word := binary.LittleEndian.Uint32(data[pos:])
		key += word
		binary.LittleEndian.PutUint32(data[pos:], word^key)
	}
	binary.LittleEndian.PutUint32(data[pos:], key)
}
