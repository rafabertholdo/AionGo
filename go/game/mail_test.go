package game

import (
	"testing"

	"aionlightning/game/store"
	"aionlightning/wire"
)

// memoryMail is a mail store that keeps letters and characters in memory, for tests.
type memoryMail struct {
	letters    map[int32]store.Letter
	characters map[string]*store.Character
}

func (m *memoryMail) InsertLetter(l *store.Letter) error {
	if m.letters == nil {
		m.letters = map[int32]store.Letter{}
	}
	m.letters[l.ID] = *l
	return nil
}
func (m *memoryMail) UpdateLetter(l *store.Letter) error { m.letters[l.ID] = *l; return nil }
func (m *memoryMail) DeleteLetter(id int32) error        { delete(m.letters, id); return nil }
func (m *memoryMail) CharacterByName(name string) (*store.Character, error) {
	return m.characters[name], nil
}
func (m *memoryMail) SetMailboxLetters(int32, int) error { return nil }

// TestMail has Ann mail Bob some potions and kinah, and Bob read the letter, take what came with it, and delete it.
func TestMail(t *testing.T) {
	d := staticDataOrSkip(t)
	const potion = 160000001
	if it := d.Items[potion]; it == nil || it.Mask&itemTradeable == 0 {
		t.Skip("no tradeable item")
	}
	s := testServer(d)
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	s.visMu.Lock()
	s.addItem(ann, potion, 10)
	ann.kinah.Count = 1000
	s.visMu.Unlock()
	ann.conn.sendMail(packet(cmSendMail, func(w *wire.Writer) {
		w.S("Bob")
		w.S("gift")
		w.S("here you go")
		w.D(ann.cube[0].UniqueID)
		w.D(4)
		w.D(0)
		w.D(100)
		w.D(0)
		w.C(0)
	}))
	if len(bob.mailbox) != 1 || bobTap.count(smMailService) != 3 || annTap.count(smMailService) != 1 {
		t.Fatalf("the letter didn't arrive: %d letters, %v %v", len(bob.mailbox), annTap.counts, bobTap.counts)
	}
	if ann.kinah.Count >= 900 || ann.cube[0].Count != 6 {
		t.Errorf("Ann has %d kinah and %d potions", ann.kinah.Count, ann.cube[0].Count)
	}
	id := bob.mailbox[0].ID
	bob.conn.readMail(packet(cmReadMail, func(w *wire.Writer) { w.D(id) }))
	if bob.mailbox[0].Unread {
		t.Errorf("the letter is still unread")
	}
	bob.conn.getMailAttachment(packet(cmGetMailAttachment, func(w *wire.Writer) { w.D(id); w.C(0) }))
	bob.conn.getMailAttachment(packet(cmGetMailAttachment, func(w *wire.Writer) { w.D(id); w.C(1) }))
	if len(bob.cube) != 1 || bob.cube[0].Count != 4 || bob.kinah.Count != 100 {
		t.Errorf("Bob has %d items and %d kinah", len(bob.cube), bob.kinah.Count)
	}
	bob.conn.deleteMail(packet(cmDeleteMail, func(w *wire.Writer) { w.D(id) }))
	if len(bob.mailbox) != 0 {
		t.Errorf("the letter wasn't deleted")
	}
}

// TestBroker has Ann put potions up for sale, Bob buy them, and Ann settle her account.
func TestBroker(t *testing.T) {
	d := staticDataOrSkip(t)
	const potion = 160000001 // group 1600: food, in the consumables
	if it := d.Items[potion]; it == nil || it.Mask&itemTradeable == 0 {
		t.Skip("no tradeable item")
	}
	s := testServer(d)
	ann, bob, annTap, bobTap := twoPlayers(t, s)
	s.visMu.Lock()
	s.addItem(ann, potion, 5)
	ann.kinah.Count = 100
	bob.kinah.Count = 1000
	s.visMu.Unlock()
	ann.conn.registerBrokerItem(packet(cmRegisterBrokerItem, func(w *wire.Writer) {
		w.D(0)
		w.D(ann.cube[0].UniqueID)
		w.D(500)
		w.D(0)
		w.H(5)
	}))
	if annTap.count(smBrokerRegistrationService) != 1 || len(ann.cube) != 0 || ann.kinah.Count != 90 {
		t.Fatalf("no registration: %v, %d kinah", annTap.counts, ann.kinah.Count)
	}
	list := func() {
		bob.conn.brokerList(packet(cmBrokerList, func(w *wire.Writer) { w.D(0); w.C(4); w.H(0); w.H(9060) }))
	}
	list()
	if bobTap.count(smBrokerItems) != 1 {
		t.Fatalf("no list")
	}
	var id int32
	for k := range s.boardOf("ELYOS").items {
		id = k
	}
	bob.conn.buyBrokerItem(packet(cmBuyBrokerItem, func(w *wire.Writer) { w.D(0); w.D(id); w.H(5) }))
	if len(bob.cube) != 1 || bob.kinah.Count != 500 || len(s.boardOf("ELYOS").items) != 0 || annTap.count(smBrokerSettledList) != 1 {
		t.Fatalf("no purchase: %d items, %d kinah, %v", len(bob.cube), bob.kinah.Count, annTap.counts)
	}
	ann.conn.brokerSettleAccountForTest()
	if ann.kinah.Count != 590 {
		t.Errorf("Ann has %d kinah after settling", ann.kinah.Count)
	}
}

func (c *conn) brokerSettleAccountForTest() {
	c.withPlayer(func(s *Server, p *player) { s.settleAccount(p) })
}
