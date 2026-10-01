// Package crypt is the Aion 1.9 login protocol's cryptography: its Blowfish
// variant, the packet checksum and first-packet scrambling, and the RSA key
// the client encrypts its credentials with.
package crypt

import "encoding/binary"

// Blowfish is the login server's Blowfish. The key schedule is standard, but
// every 32-bit half-block is read and written little-endian, so it doesn't
// match standard (big-endian) Blowfish implementations.
type Blowfish struct {
	p [18]uint32
	s [4][256]uint32
}

// NewBlowfish returns the cipher keyed with key (any length from 1 byte).
func NewBlowfish(key []byte) *Blowfish {
	c := &Blowfish{}
	c.s = [4][256]uint32{sInit0, sInit1, sInit2, sInit3}
	k := 0
	for i := range c.p {
		var data uint32
		for range 4 {
			data = data<<8 | uint32(key[k])
			k = (k + 1) % len(key)
		}
		c.p[i] = pInit[i] ^ data
	}
	var block [8]byte
	for i := 0; i < 18; i += 2 {
		c.Encrypt(block[:])
		c.p[i] = binary.LittleEndian.Uint32(block[0:])
		c.p[i+1] = binary.LittleEndian.Uint32(block[4:])
	}
	for b := range c.s {
		for j := 0; j < 256; j += 2 {
			c.Encrypt(block[:])
			c.s[b][j] = binary.LittleEndian.Uint32(block[0:])
			c.s[b][j+1] = binary.LittleEndian.Uint32(block[4:])
		}
	}
	return c
}

func (c *Blowfish) f(x uint32) uint32 {
	return ((c.s[0][x>>24] + c.s[1][x>>16&0xff]) ^ c.s[2][x>>8&0xff]) + c.s[3][x&0xff]
}

// Encrypt enciphers data in place, 8 bytes at a time; a trailing partial block is left as it is.
func (c *Blowfish) Encrypt(data []byte) {
	for p := 0; p+8 <= len(data); p += 8 {
		l := binary.LittleEndian.Uint32(data[p:])
		r := binary.LittleEndian.Uint32(data[p+4:])
		for i := range 16 {
			l ^= c.p[i]
			r ^= c.f(l)
			l, r = r, l
		}
		l, r = r, l
		r ^= c.p[16]
		l ^= c.p[17]
		binary.LittleEndian.PutUint32(data[p:], l)
		binary.LittleEndian.PutUint32(data[p+4:], r)
	}
}

// Decrypt deciphers data in place, 8 bytes at a time.
func (c *Blowfish) Decrypt(data []byte) {
	for p := 0; p+8 <= len(data); p += 8 {
		l := binary.LittleEndian.Uint32(data[p:])
		r := binary.LittleEndian.Uint32(data[p+4:])
		for i := 17; i > 1; i-- {
			l ^= c.p[i]
			r ^= c.f(l)
			l, r = r, l
		}
		l, r = r, l
		r ^= c.p[1]
		l ^= c.p[0]
		binary.LittleEndian.PutUint32(data[p:], l)
		binary.LittleEndian.PutUint32(data[p+4:], r)
	}
}
