package game

import (
	"math"
	"slices"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// Mail: letters with kinah or an item attached (AL-Game's MailService).
// ponytail: express mail isn't ported, as in AL-Game; the item details of a letter are those of the cube.

func init() {
	handlers[cmSendMail] = (*conn).sendMail
	handlers[cmReadMail] = (*conn).readMail
	handlers[cmGetMailAttachment] = (*conn).getMailAttachment
	handlers[cmDeleteMail] = (*conn).deleteMail
}

// mailSaver keeps letters; store.Store does.
type mailSaver interface {
	InsertLetter(*store.Letter) error
	UpdateLetter(*store.Letter) error
	DeleteLetter(int32) error
	CharacterByName(string) (*store.Character, error)
	SetMailboxLetters(playerID int32, n int) error
}

// MailMessage ids.
const (
	mailSent           = 0
	mailNoSuchName     = 1
	mailRecipientFull  = 2
	mailOneRaceOnly    = 3
	mailboxSlots       = 65536
	mailboxCapacity    = 100
	mailMaxTitle       = 20
	mailMaxMessage     = 1000
	mailBaseCommission = 10
)

// letter is a Letter of a player's mailbox.
type letter struct {
	store.Letter
	item *store.Item // what is attached, if anything
}

func (p *player) letter(id int32) *letter {
	for _, l := range p.mailbox {
		if l.ID == id {
			return l
		}
	}
	return nil
}

func (p *player) haveUnread() bool {
	return slices.ContainsFunc(p.mailbox, func(l *letter) bool { return l.Unread })
}

// sortedLetters is Mailbox.getLetters: by the time they came, then by id.
func (p *player) sortedLetters() []*letter {
	list := slices.Clone(p.mailbox)
	slices.SortFunc(list, func(a, b *letter) int {
		if c := a.Received.Compare(b.Received); c != 0 {
			return c
		}
		return int(a.ID - b.ID)
	})
	return list
}

// loadMailbox is MailDAO.loadPlayerMailbox.
func (s *Server) loadMailbox(p *player) error {
	letters, err := s.store.Letters(p.ID)
	if err != nil {
		return err
	}
	items, err := s.store.Items(p.ID, store.MailboxLocation, false)
	if err != nil {
		return err
	}
	p.mailbox = nil
	for _, l := range letters {
		item := &letter{Letter: l}
		for _, it := range items {
			if l.ItemID != 0 && it.UniqueID == l.ItemID {
				item.item = it
			}
		}
		p.mailbox = append(p.mailbox, item)
	}
	return nil
}

// mailLetters is SM_MAIL_SERVICE's letters of the mailbox (2).
func (s *Server) mailLetters(p *player) *wire.Writer {
	letters := p.sortedLetters()
	if len(letters) == 0 {
		return emptyMailbox(p)
	}
	w := wire.Packet(smMailService)
	w.C(2)
	w.D(p.ID)
	w.C(0)
	w.H(uint16(mailboxSlots - len(p.mailbox)))
	for _, l := range letters {
		w.D(l.ID)
		w.S(l.Sender)
		w.S(l.Title)
		w.Bool(!l.Unread)
		if l.item != nil {
			w.D(l.item.UniqueID)
			w.D(l.item.ItemID)
		} else {
			w.D(0)
			w.D(0)
		}
		w.D(int32(l.Kinah))
		w.D(0)
		w.C(0)
	}
	return w
}

func mailState(newMail, unread bool) *wire.Writer {
	w := wire.Packet(smMailService)
	w.C(0)
	w.Bool(newMail)
	w.C(0)
	w.Bool(unread)
	w.D(0)
	w.C(0)
	return w
}

func mailMessage(id byte) *wire.Writer {
	w := wire.Packet(smMailService)
	w.C(1)
	w.C(id)
	return w
}

func (s *Server) mailRead(p *player, l *letter) *wire.Writer {
	w := wire.Packet(smMailService)
	w.C(3)
	w.D(p.ID)
	w.D(1)
	w.D(0)
	w.D(l.ID)
	w.D(p.ID)
	w.S(l.Sender)
	w.S(l.Title)
	w.S(l.Message)
	if l.item != nil {
		t := s.template(l.item)
		if t == nil {
			t = &data.ItemTemplate{ID: l.item.ItemID}
		}
		w.D(l.item.UniqueID)
		w.D(t.ID)
		w.D(1)
		w.D(0)
		w.H(0x24)
		w.D(t.NameID)
		w.H(0)
		s.writeItemDetails(w, p, l.item, t)
	} else {
		for range 5 {
			w.D(0)
		}
	}
	w.D(int32(l.Kinah))
	w.D(0)
	w.C(0)
	w.Q(l.Received.Unix())
	w.C(0)
	return w
}

// qualityPriceRate is the share of an attached item's price the post office asks, by the item's quality.
func qualityPriceRate(quality string) float32 {
	switch quality {
	case "RARE":
		return 0.03
	case "LEGEND", "UNIQUE":
		return 0.04
	case "MYTHIC", "EPIC":
		return 0.05
	}
	return 0.02
}

func mailRound(f float32) int64 { return int64(math.Floor(float64(f) + 0.5)) }

// sendMail is CM_SEND_MAIL.
func (c *conn) sendMail(r *wire.Reader) {
	name, title, message := r.S(), r.S(), r.S()
	itemID, itemCount := r.D(), int64(r.D())
	r.D()
	kinah := int64(r.D())
	r.D()
	express := r.C()
	if r.Err != nil || express != 0 {
		return
	}
	c.withPlayer(func(s *Server, p *player) { s.sendLetter(p, name, title, message, itemID, itemCount, kinah) })
}

// sendLetter is MailService.sendMail.
func (s *Server) sendLetter(sender *player, name, title, message string, itemID int32, itemCount, kinah int64) {
	if len(name) > 16 {
		return
	}
	title = truncate(title, mailMaxTitle)
	message = truncate(message, mailMaxMessage)
	recipient := s.playerNamed(name)
	var cd *store.Character
	if recipient != nil {
		cd = recipient.Character
	} else {
		var err error
		if cd, err = s.mailDB.CharacterByName(convertName(name)); err != nil || cd == nil {
			sender.conn.send(mailMessage(mailNoSuchName))
			return
		}
	}
	if cd.Race != sender.Race {
		sender.conn.send(mailMessage(mailOneRaceOnly))
		return
	}
	if (recipient != nil && len(recipient.mailbox) >= mailboxCapacity) || (recipient == nil && cd.Letters >= mailboxCapacity) {
		sender.conn.send(mailMessage(mailRecipientFull))
		return
	}
	item := sender.cubeItem(itemID)
	price := int64(mailBaseCommission) + mailRound(float32(kinah)*0.01)
	if itemID != 0 {
		if item == nil || itemCount < 1 || itemCount > item.Count || item.SoulBound {
			return
		}
		t := s.template(item)
		if t == nil || t.Mask&itemTradeable == 0 {
			return
		}
		price += mailRound(float32(int64(t.Price)*itemCount) * qualityPriceRate(t.Quality))
	}
	if sender.kinah.Count < price || sender.kinah.Count < kinah {
		return
	}
	l := &letter{Letter: store.Letter{ID: s.ids.nextID(), Recipient: cd.ID, Sender: sender.Name, Title: title, Message: message,
		Unread: true, Kinah: max(kinah, 0), Received: time.Now()}}
	if item != nil {
		if item.Count == itemCount {
			sender.cube = removeFromCube(sender.cube, item)
			s.sendDeleted(sender, storageCube, item.UniqueID)
			l.item = item
		} else {
			l.item = s.newItem(sender, item.ItemID, itemCount)
			s.decreaseItemCount(sender, item, itemCount)
			if err := s.items.InsertItem(l.item); err != nil {
				s.log.Error("saving item", "item", l.item.ItemID, "err", err)
			}
		}
		l.item.Equipped, l.item.Slot, l.item.Location, l.item.Owner = false, 0, store.MailboxLocation, cd.ID
		l.ItemID = l.item.UniqueID
		s.saveItem(l.item)
	}
	if err := s.mailDB.InsertLetter(&l.Letter); err != nil {
		s.log.Error("saving a letter", "err", err)
		return
	}
	s.decreaseKinah(sender, l.Kinah+price)
	if recipient != nil {
		recipient.mailbox = append(recipient.mailbox, l)
		recipient.conn.send(s.mailLetters(recipient))
		recipient.conn.send(mailState(false, false))
		recipient.conn.send(mailState(true, true))
	} else {
		cd.Letters++
		if err := s.mailDB.SetMailboxLetters(cd.ID, cd.Letters); err != nil {
			s.log.Error("counting a letter", "err", err)
		}
	}
	sender.conn.send(mailMessage(mailSent))
}

func truncate(text string, n int) string {
	if r := []rune(text); len(r) > n {
		return string(r[:n])
	}
	return text
}

// readMail is CM_READ_MAIL.
func (c *conn) readMail(r *wire.Reader) {
	id := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		l := p.letter(id)
		if l == nil {
			return
		}
		p.conn.send(s.mailRead(p, l))
		if l.Unread {
			l.Unread = false
			if err := s.mailDB.UpdateLetter(&l.Letter); err != nil {
				s.log.Error("saving a letter", "err", err)
			}
		}
	})
}

// getMailAttachment is CM_GET_MAIL_ATTACHMENT: the player takes the item (0) or the kinah (1) of a letter.
func (c *conn) getMailAttachment(r *wire.Reader) {
	id, kind := r.D(), r.C()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		l := p.letter(id)
		if l == nil {
			return
		}
		state := func() *wire.Writer {
			w := wire.Packet(smMailService)
			w.C(5)
			w.D(id)
			w.C(kind)
			w.C(1)
			return w
		}
		switch kind {
		case 0:
			if l.item == nil {
				return
			}
			if p.cubeFull() {
				p.conn.send(systemMessage(msgInventoryFull))
				return
			}
			item := l.item
			item.Owner, item.Location, item.Slot = p.ID, storageCube, firstAvailableSlot
			p.cube = append(p.cube, item)
			s.saveItem(item)
			p.conn.send(s.addItemsPacket(p, item))
			p.conn.send(state())
			l.item, l.ItemID = nil, 0
		case 1:
			s.increaseKinah(p, l.Kinah)
			p.conn.send(state())
			l.Kinah = 0
		default:
			return
		}
		if err := s.mailDB.UpdateLetter(&l.Letter); err != nil {
			s.log.Error("saving a letter", "err", err)
		}
	})
}

// deleteMail is CM_DELETE_MAIL.
func (c *conn) deleteMail(r *wire.Reader) {
	id := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		if p.letter(id) == nil {
			return
		}
		p.mailbox = slices.DeleteFunc(p.mailbox, func(l *letter) bool { return l.ID == id })
		if err := s.mailDB.DeleteLetter(id); err != nil {
			s.log.Error("deleting a letter", "err", err)
		}
		w := wire.Packet(smMailService)
		w.C(6)
		w.D(0)
		w.D(0)
		w.D(id)
		p.conn.send(w)
	})
}
