package game

import "aionlightning/crypt"

// gameCrypt is the server's side of a game connection's encryption. The
// first packet, SM_KEY, goes out in the clear.
type gameCrypt struct {
	in, out *crypt.GameCipher
	enabled bool
}

// newGameCrypt keys the connection from key, and returns the value SM_KEY sends the client for it.
func newGameCrypt(key uint32) (*gameCrypt, int32) {
	return &gameCrypt{in: crypt.NewGameCipher(key), out: crypt.NewGameCipher(key)}, crypt.GameKeySent(key)
}

// encrypt encrypts an outgoing packet in place; the first one stays in the clear.
func (c *gameCrypt) encrypt(data []byte) {
	if !c.enabled {
		c.enabled = true
		return
	}
	c.out.Encrypt(data)
}

// decrypt decrypts an incoming packet in place and reports whether it has a valid header.
func (c *gameCrypt) decrypt(data []byte) bool {
	if !c.enabled || len(data) < 3 {
		return false
	}
	c.in.Decrypt(data)
	return data[0] == ^data[2] && data[1] == crypt.ClientPacketCode
}
