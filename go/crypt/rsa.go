package crypt

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"math/big"
)

// KeyPair is a 1024-bit RSA key and its modulus scrambled the way the client
// expects to find it in SM_INIT.
type KeyPair struct {
	Private          *rsa.PrivateKey
	ScrambledModulus []byte
}

// NewKeyPair generates a key with the public exponent 65537.
func NewKeyPair() (*KeyPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}
	return &KeyPair{Private: key, ScrambledModulus: ScrambleModulus(key.N)}, nil
}

// ScrambleModulus is the 128-byte big-endian modulus with bytes 0-3 and
// 0x4d-0x50 swapped, then three XOR passes between its halves and two small spans.
func ScrambleModulus(n *big.Int) []byte {
	m := n.FillBytes(make([]byte, 128))
	for i := range 4 {
		m[i], m[0x4d+i] = m[0x4d+i], m[i]
	}
	for i := range 0x40 {
		m[i] ^= m[0x40+i]
	}
	for i := range 4 {
		m[0x0d+i] ^= m[0x34+i]
	}
	for i := range 0x40 {
		m[0x40+i] ^= m[i]
	}
	return m
}

// Decrypt is raw RSA without padding, as the client encrypts its 128-byte
// credentials block, returned as 128 big-endian bytes.
func (k *KeyPair) Decrypt(block []byte) []byte {
	c := new(big.Int).SetBytes(block)
	return new(big.Int).Exp(c, k.Private.D, k.Private.N).FillBytes(make([]byte, 128))
}

// NewBlowfishKey is a random 16-byte key, what Java's default Blowfish KeyGenerator makes.
func NewBlowfishKey() []byte {
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	return key
}

func randomUint32() uint32 {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return binary.LittleEndian.Uint32(b[:])
}

// RandomInt32 is a random 32-bit value, for session and play keys.
func RandomInt32() int32 {
	return int32(randomUint32())
}
