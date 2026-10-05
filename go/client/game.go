package client

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
	"unicode/utf16"

	"aionlightning/crypt"
	"aionlightning/wire"
)

// Client packet opcodes this client sends.
const (
	cmCreateCharacter  = 0x0a
	cmL2authLoginCheck = 0x08
	cmCharacterList    = 0x09
	cmMove             = 0xa3
	cmAttack           = 0x93
	cmTargetSelect     = 0x92
	cmCastSpell        = 0x8c
	cmStartLoot        = 0x05
	cmLootItem         = 0x06
	cmChatMessage      = 0x86 // CM_CHAT_MESSAGE_PUBLIC
	cmFindGroup        = 0x40
	cmQuestionResponse = 0x9d
	cmWhisper          = 0x87 // CM_CHAT_MESSAGE_WHISPER
	cmChatWindow       = 0xb0
	cmQuit             = 0xee
	cmLevelReady       = 0xf4
	cmVersionCheck     = 0xf3
	cmEnterWorld       = 0xfb
)

// Server packet opcodes this client reads.
const (
	SmMessage          = 0x14
	SmQuestionWindow   = 0x50
	SmPlayerInfo       = 0x1c
	SmPlayerSpawn      = 0x2d
	SmNpcInfo          = 0x2a
	SmEmotion          = 0x43
	SmQuitResponse     = 0x5e
	SmKey              = 0x64
	SmCharacterList    = 0xe4
	SmL2authLoginCheck = 0xe5
	SmCreateCharacter  = 0xe7
)

// Packet is a server packet: its opcode and payload.
type Packet struct {
	Op   byte
	Data []byte
}

// Reader reads the packet's payload.
func (p Packet) Reader() *wire.Reader { return wire.NewReader(p.Data) }

// Game is a connection to a game server.
type Game struct {
	Session *Session
	conn    net.Conn
	in, out *crypt.GameCipher
	writeMu sync.Mutex

	position Position

	// Packets delivers every server packet, in order, until the connection closes.
	Packets chan Packet
}

// Character is a character on the select screen.
type Character struct {
	ID   int32
	Name string
}

// Dial connects to the session's game server at address (host:port; empty
// for the address the login server gave) and logs in to it.
func Dial(s *Session, address string) (*Game, error) {
	if address == "" {
		address = net.JoinHostPort(s.Server.Address.String(), fmt.Sprint(s.Server.Port))
	}
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	g := &Game{Session: s, conn: conn, Packets: make(chan Packet, 1024)}
	// SM_KEY, in the clear.
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	first, err := wire.ReadFrame(conn)
	if err != nil || len(first) < 7 || crypt.DecodeServerOpcode(first[0]) != SmKey {
		conn.Close()
		return nil, fmt.Errorf("no SM_KEY: %v", err)
	}
	key := crypt.GameKeyReceived(int32(binary.LittleEndian.Uint32(first[3:])))
	g.in, g.out = crypt.NewGameCipher(key), crypt.NewGameCipher(key)
	_ = conn.SetReadDeadline(time.Time{})
	go g.read()

	// CM_VERSION_CHECK, as the 1.9 client sends it.
	w := wire.Packet(cmVersionCheck)
	w.B([]byte{0xbb, 0x00, 0x27, 0x00, 0xe4, 0x04, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00})
	g.Send(w)
	w = wire.Packet(cmL2authLoginCheck)
	w.D(s.PlayOK2)
	w.D(s.PlayOK1)
	w.D(s.AccountID)
	w.D(s.LoginOK)
	g.Send(w)
	p, err := g.Await(SmL2authLoginCheck)
	if err != nil {
		g.Close()
		return nil, err
	}
	if result := p.Reader().D(); result != 0 {
		g.Close()
		return nil, fmt.Errorf("the game server refused the session: %d", result)
	}
	return g, nil
}

func (g *Game) read() {
	defer close(g.Packets)
	for {
		payload, err := wire.ReadFrame(g.conn)
		if err != nil || len(payload) < 3 {
			return
		}
		g.in.Decrypt(payload)
		g.Packets <- Packet{Op: crypt.DecodeServerOpcode(payload[0]), Data: payload[3:]}
	}
}

// Send sends a client packet: its opcode, the client's code and the opcode's complement, encrypted.
func (g *Game) Send(w *wire.Writer) {
	if w == nil || len(w.Data) == 0 {
		return
	}
	if len(w.Data) > wire.MaxPayloadSize-2 {
		_ = g.conn.Close()
		return
	}
	op := w.Data[0]
	payload := append([]byte{op, crypt.ClientPacketCode, ^op}, w.Data[1:]...)
	g.writeMu.Lock()
	defer g.writeMu.Unlock()
	g.out.Encrypt(payload)
	if err := wire.WriteFrame(g.conn, payload); err != nil {
		_ = g.conn.Close()
	}
}

// Await returns the next packet with opcode op, dropping those before it.
func (g *Game) Await(op byte) (Packet, error) {
	deadline := time.After(timeout)
	for {
		select {
		case p, ok := <-g.Packets:
			if !ok {
				return Packet{}, errors.New("the game server closed the connection")
			}
			if p.Op == op {
				return p, nil
			}
		case <-deadline:
			return Packet{}, fmt.Errorf("no packet %#x from the game server", op)
		}
	}
}

// Close closes the connection.
func (g *Game) Close() error { return g.conn.Close() }

// characterInfoSize is a character's PlayerInfo on the select screen, and the 14 bytes after it.
const characterInfoSize = 456 + 14

// Characters is the account's characters.
func (g *Game) Characters() ([]Character, error) {
	w := wire.Packet(cmCharacterList)
	w.D(g.Session.PlayOK2)
	g.Send(w)
	p, err := g.Await(SmCharacterList)
	if err != nil {
		return nil, err
	}
	r := p.Reader()
	r.D()
	characters := make([]Character, r.C())
	for i := range characters {
		info := r.B(characterInfoSize)
		characters[i] = Character{ID: int32(binary.LittleEndian.Uint32(info)), Name: utf16String(info[4:48])}
	}
	return characters, r.Err
}

// Races, genders and classes for Create.
const (
	Elyos, Asmodian = 0, 1
	Male, Female    = 0, 1
	Warrior, Scout  = 0, 3
	Mage, Priest    = 6, 9
)

// Create makes a character with the default appearance and returns its id.
func (g *Game) Create(name string, race, gender, class int32) (int32, error) {
	w := wire.Packet(cmCreateCharacter)
	w.D(g.Session.PlayOK2)
	w.S(g.Session.Account)
	w.S(name)
	w.B(make([]byte, 42-len(utf16.Encode([]rune(name)))*2))
	w.D(gender)
	w.D(race)
	w.D(class)
	w.D(0) // voice
	for range 4 {
		w.D(0xc0c0c0) // skin, hair, eye, lip colours
	}
	for _, v := range []byte{1, 1, 0, 0, 4} { // face, hair, deco, tattoo, always 4
		w.C(v)
	}
	w.B(make([]byte, 37)) // face shape … facial rate
	w.C(0)
	w.B(make([]byte, 3)) // arm length, leg length, shoulders
	w.C(0)
	w.C(0)
	w.F(1) // height
	g.Send(w)
	p, err := g.Await(SmCreateCharacter)
	if err != nil {
		return 0, err
	}
	r := p.Reader()
	if code := r.D(); code != 0 {
		return 0, fmt.Errorf("character not created: code %d", code)
	}
	return r.D(), r.Err
}

// EnterWorld enters the world with a character and waits until it stands in it.
func (g *Game) EnterWorld(id int32) error {
	w := wire.Packet(cmEnterWorld)
	w.D(id)
	g.Send(w)
	if _, err := g.Await(SmPlayerSpawn); err != nil {
		return err
	}
	g.Send(wire.Packet(cmLevelReady))
	p, err := g.Await(SmPlayerInfo)
	if err != nil {
		return err
	}
	r := p.Reader()
	g.position = Position{X: r.F(), Y: r.F(), Z: r.F()}
	return r.Err
}

// Position is where a character stands.
type Position struct{ X, Y, Z float32 }

// Position is where the character entered the world.
func (g *Game) Position() (Position, error) {
	return g.position, nil
}

// Movement types for Move.
const (
	MoveStop          = 0x00
	MoveStartKeyboard = 0xc0
)

// Move tells the server where the character is, heading to (toX, toY, toZ)
// when it starts moving.
func (g *Game) Move(x, y, z float32, heading, kind byte, toX, toY, toZ float32) {
	w := wire.Packet(cmMove)
	w.F(x)
	w.F(y)
	w.F(z)
	w.C(heading)
	w.C(kind)
	if kind == MoveStartKeyboard {
		w.F(toX)
		w.F(toY)
		w.F(toZ)
	}
	g.Send(w)
}

// Target selects the object with id, or nothing with 0.
func (g *Game) Target(id int32) {
	w := wire.Packet(cmTargetSelect)
	w.D(id)
	w.C(0)
	g.Send(w)
}

// Attack hits the target with a normal attack.
func (g *Game) Attack(id int32, number byte) {
	w := wire.Packet(cmAttack)
	w.D(id)
	w.C(number)
	w.H(0)
	w.C(1)
	g.Send(w)
}

// Cast uses a skill on the target.
func (g *Game) Cast(skill uint16, target int32) {
	w := wire.Packet(cmCastSpell)
	w.H(skill)
	w.C(1)
	w.C(0)
	w.D(target)
	w.H(0)
	g.Send(w)
}

// OpenLoot opens (or, closing, closes) the loot of a corpse.
func (g *Game) OpenLoot(id int32, closing bool) {
	w := wire.Packet(cmStartLoot)
	w.D(id)
	w.Bool(closing)
	g.Send(w)
}

// TakeLoot takes the item at index of an open loot.
func (g *Game) TakeLoot(id int32, index byte) {
	w := wire.Packet(cmLootItem)
	w.D(id)
	w.C(index)
	g.Send(w)
}

// Say says text in normal chat, to those around.
func (g *Game) Say(text string) {
	w := wire.Packet(cmChatMessage)
	w.C(0) // ChatType NORMAL
	w.S(text)
	g.Send(w)
}

// AnswerQuestion answers a question window (SM_QUESTION_WINDOW) by its code.
func (g *Game) AnswerQuestion(code int32, yes bool) {
	w := wire.Packet(cmQuestionResponse)
	w.D(code)
	w.Bool(yes)
	g.Send(w)
}

// ApplyToGroup applies to a Find Group recruit post as the window does: it opens the private chat
// window with the post's owner (CM_CHAT_WINDOW) and whispers the application.
func (g *Game) ApplyToGroup(owner, text string) {
	w := wire.Packet(cmChatWindow)
	w.S(owner)
	w.D(0)
	g.Send(w)
	w = wire.Packet(cmWhisper)
	w.S(owner)
	w.S(text)
	g.Send(w)
}

// FindGroupApply posts the character on Find Group's apply list, as the window's
// "Apply for Group" does: object id, message, group type 0, then class and level.
func (g *Game) FindGroupApply(id int32, message string, class, level byte) {
	w := wire.Packet(cmFindGroup)
	w.C(6)
	w.D(id)
	w.S(message)
	w.C(0)
	w.C(class)
	w.C(level)
	g.Send(w)
}

// Quit leaves the world; with logout, back to the select screen, else it disconnects.
func (g *Game) Quit(logout bool) error {
	w := wire.Packet(cmQuit)
	w.Bool(logout)
	g.Send(w)
	_, err := g.Await(SmQuitResponse)
	return err
}

func utf16String(b []byte) string {
	var units []uint16
	for i := 0; i+1 < len(b); i += 2 {
		u := binary.LittleEndian.Uint16(b[i:])
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return string(utf16.Decode(units))
}
