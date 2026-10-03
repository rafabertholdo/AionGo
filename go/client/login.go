// Package client is the Aion 1.9 client's side of the protocol, without the
// game: it logs in, enters the world and plays by packets, for tests and bots.
package client

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"time"

	"aionlightning/crypt"
	"aionlightning/wire"
)

// Session is an account logged in to the login server, with the keys that let
// it into a game server.
type Session struct {
	Account          string
	AccountID        int32
	LoginOK          int32
	PlayOK1, PlayOK2 int32
	Server           Server   // the game server it plays on
	Servers          []Server // the login server's whole list, in list order
}

// Server is a game server of the login server's list.
type Server struct {
	ID      byte
	Address net.IP
	Port    int32
}

// protocolRevision is the login protocol SM_INIT announces.
const protocolRevision = 0x0000c621

// timeout bounds each wait for the server.
const timeout = 10 * time.Second

// loginConn is a connection to the login server.
type loginConn struct {
	conn      net.Conn
	cipher    *crypt.Blowfish
	sessionID int32
	modulus   *big.Int
}

// Login logs in to the login server at address and asks to play on the game
// server with serverID. The login server creates an account it doesn't know,
// if configured to.
func Login(address, name, password string, serverID byte) (*Session, error) {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	c := &loginConn{conn: conn}
	if err := c.init(); err != nil {
		return nil, err
	}

	w := wire.Packet(0x07) // CM_AUTH_GG
	w.D(c.sessionID)
	w.B(make([]byte, 16))
	if op, r, err := c.exchange(w); err != nil || op != 0x0b || r.D() != c.sessionID {
		return nil, fmt.Errorf("GameGuard step: opcode %#x, %v", op, err)
	}
	// CM_LOGIN: the name and password in an RSA block.
	block := make([]byte, 128)
	copy(block[64:], name)
	copy(block[96:], password)
	w = wire.Packet(0x0b)
	w.D(0)
	w.B(new(big.Int).Exp(new(big.Int).SetBytes(block), big.NewInt(65537), c.modulus).FillBytes(make([]byte, 128)))
	op, r, err := c.exchange(w)
	if err != nil {
		return nil, err
	}
	if op != 0x03 {
		return nil, fmt.Errorf("login refused: opcode %#x, reason %d", op, r.D())
	}
	s := &Session{Account: name, AccountID: r.D(), LoginOK: r.D()}

	w = wire.Packet(0x05) // CM_SERVER_LIST
	w.D(s.AccountID)
	w.D(s.LoginOK)
	w.D(0)
	if op, r, err = c.exchange(w); err != nil || op != 0x04 {
		return nil, fmt.Errorf("server list: opcode %#x, %v", op, err)
	}
	count := int(r.C())
	r.C() // last server
	for range count {
		server := Server{ID: r.C(), Address: net.IP(r.B(4)), Port: r.D()}
		r.B(2 + 2 + 2 + 1 + 4 + 1)
		s.Servers = append(s.Servers, server)
		if server.ID == serverID {
			s.Server = server
		}
	}
	if s.Server.ID != serverID {
		return nil, fmt.Errorf("no game server %d on the list", serverID)
	}

	w = wire.Packet(0x02) // CM_PLAY
	w.D(s.AccountID)
	w.D(s.LoginOK)
	w.C(serverID)
	if op, r, err = c.exchange(w); err != nil || op != 0x07 {
		return nil, fmt.Errorf("play refused: opcode %#x, %v", op, err)
	}
	s.PlayOK1, s.PlayOK2 = r.D(), r.D()
	return s, r.Err
}

// init reads SM_INIT: the connection's key and the RSA modulus.
func (c *loginConn) init() error {
	data, err := c.readFrame()
	if err != nil {
		return err
	}
	// The fixed initial key, then undo the XOR pass from the key stored 8 bytes from the end.
	crypt.NewBlowfish([]byte{0x6b, 0x60, 0xcb, 0x5b, 0x82, 0xce, 0x90, 0xb1, 0xcc, 0x2b, 0x6c, 0x55, 0x6c, 0x6c, 0x6c, 0x6c}).Decrypt(data)
	stop := len(data) - 8
	key := binary.LittleEndian.Uint32(data[stop:])
	for pos := stop - 4; pos >= 4; pos -= 4 {
		word := binary.LittleEndian.Uint32(data[pos:]) ^ key
		key -= word
		binary.LittleEndian.PutUint32(data[pos:], word)
	}
	r := wire.NewReader(data)
	if op := r.C(); op != 0x00 {
		return fmt.Errorf("first packet is %#x, not SM_INIT", op)
	}
	c.sessionID = r.D()
	if revision := r.D(); revision != protocolRevision {
		return fmt.Errorf("protocol revision %#x", revision)
	}
	c.modulus = new(big.Int).SetBytes(unscramble(r.B(128)))
	r.B(16)
	c.cipher = crypt.NewBlowfish(r.B(16))
	return r.Err
}

// unscramble undoes crypt.ScrambleModulus, as the client does.
func unscramble(m []byte) []byte {
	for i := range 0x40 {
		m[0x40+i] ^= m[i]
	}
	for i := range 4 {
		m[0x0d+i] ^= m[0x34+i]
	}
	for i := range 0x40 {
		m[i] ^= m[0x40+i]
	}
	for i := range 4 {
		m[i], m[0x4d+i] = m[0x4d+i], m[i]
	}
	return m
}

func (c *loginConn) readFrame() ([]byte, error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(timeout))
	var header [2]byte
	if _, err := io.ReadFull(c.conn, header[:]); err != nil {
		return nil, err
	}
	size := int(binary.LittleEndian.Uint16(header[:]))
	if size < 2 {
		return nil, errors.New("bad frame size")
	}
	data := make([]byte, size-2)
	_, err := io.ReadFull(c.conn, data)
	return data, err
}

// exchange sends a packet and reads the answer.
func (c *loginConn) exchange(w *wire.Writer) (byte, *wire.Reader, error) {
	if err := c.send(w); err != nil {
		return 0, nil, err
	}
	data, err := c.readFrame()
	if err != nil {
		return 0, nil, err
	}
	c.cipher.Decrypt(data)
	var sum uint32
	for i := 0; i+4 <= len(data); i += 4 {
		sum ^= binary.LittleEndian.Uint32(data[i:])
	}
	if sum != 0 || len(data) == 0 {
		return 0, nil, errors.New("bad checksum from the login server")
	}
	return data[0], wire.NewReader(data[1:]), nil
}

// send pads, checksums and encrypts a packet as the client does: the checksum
// word, then a last word of filler that the server's check leaves out.
func (c *loginConn) send(w *wire.Writer) error {
	if len(w.Data) > wire.MaxPayloadSize {
		return wire.ErrFrameTooLarge
	}
	size := len(w.Data) + 8
	size += 8 - size%8
	if size > wire.MaxPayloadSize {
		return wire.ErrFrameTooLarge
	}
	data := make([]byte, size)
	copy(data, w.Data)
	var sum uint32
	for i := 0; i < size-8; i += 4 {
		sum ^= binary.LittleEndian.Uint32(data[i:])
	}
	binary.LittleEndian.PutUint32(data[size-8:], sum)
	binary.LittleEndian.PutUint32(data[size-4:], 0xdeadbeef)
	c.cipher.Encrypt(data)
	if err := wire.WriteFrame(c.conn, data); err != nil {
		_ = c.conn.Close()
		return err
	}
	return nil
}
