package game

import (
	"slices"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// handlers are the client packets the game server handles so far, by opcode.
var handlers = map[byte]func(*conn, *wire.Reader){
	cmVersionCheck:     (*conn).versionCheck,
	cmL2authLoginCheck: (*conn).loginCheck,
	cmCharacterList:    (*conn).characterList,
	cmCreateCharacter:  (*conn).createCharacter,
	cmDeleteCharacter:  (*conn).deleteCharacter,
	cmRestoreCharacter: (*conn).restoreCharacter,
	cmCheckNickname:    (*conn).checkNickname,
	cmMayLoginIntoGame: func(c *conn, _ *wire.Reader) { w := wire.Packet(smMayLoginIntoGame); w.D(0); c.send(w) },
	cmReconnectAuth:    func(c *conn, _ *wire.Reader) { c.s.login.requestReconnect(c) },
	cmMacAddress:       func(*conn, *wire.Reader) {},
	cmMayQuit:          func(*conn, *wire.Reader) {},
	cmTimeCheck:        (*conn).timeCheck,
	cmPing:             func(c *conn, _ *wire.Reader) { w := wire.Packet(smPong); w.C(0); w.C(0); c.send(w) },
	cmQuit:             (*conn).quit,
}

// Character creation results (SM_CREATE_CHARACTER).
const (
	createOK          = 0
	createFailed      = 1
	createDBError     = 2
	createInvalidName = 5
	createNameUsed    = 10
)

// deletionDelay is how long a deleted character can still be restored.
const deletionDelay = 5 * time.Minute

var classIDs = map[string]int32{"WARRIOR": 0, "GLADIATOR": 1, "TEMPLAR": 2, "SCOUT": 3, "ASSASSIN": 4, "RANGER": 5,
	"MAGE": 6, "SORCERER": 7, "SPIRIT_MASTER": 8, "PRIEST": 9, "CLERIC": 10, "CHANTER": 11}

func className(id int32) (string, bool) {
	for name, classID := range classIDs {
		if classID == id {
			return name, true
		}
	}
	return "", false
}

func (c *conn) versionCheck(r *wire.Reader) {
	chatAddress, chatPort := c.s.chat.clientAddress()
	config := c.s.config
	w := wire.Packet(smVersionCheck)
	w.C(0)
	w.C(config.ID)
	w.D(0x000188ad)
	w.D(0x000188a6)
	w.D(0)
	w.D(0x00018898)
	w.D(0x4c346d9d)
	w.C(0)
	w.C(config.CountryCode)
	w.C(0)
	w.C(config.Mode)
	w.D(int32(time.Now().Unix()))
	w.H(0x015e)
	w.H(0x0a01)
	w.H(0x0a01)
	w.H(0x020a)
	w.C(0)
	w.C(1)
	w.C(0)
	w.C(0)
	w.B(chatAddress[:])
	w.H(chatPort)
	c.send(w)
}

func (c *conn) timeCheck(r *wire.Reader) {
	w := wire.Packet(smTimeCheck)
	w.D(int32(time.Now().UnixMilli()))
	w.D(r.D())
	c.send(w)
}

// loginCheck is CM_L2AUTH_LOGIN_CHECK: the session keys the login server gave the player.
func (c *conn) loginCheck(r *wire.Reader) {
	playOK2, playOK1, accountID, loginOK := r.D(), r.D(), r.D(), r.D()
	c.s.login.authenticate(c, accountID, loginOK, playOK1, playOK2)
}

// accountAuthenticated is the login server's answer: load the account's characters and let the player in.
func (s *Server) accountAuthenticated(c *conn, accountID int32, ok bool, name string, accessLevel, membership byte) {
	fail := func() {
		w := wire.Packet(smL2authLoginCheck)
		w.D(1)
		w.S(name)
		c.close(w)
	}
	if !ok {
		fail()
		return
	}
	a := &account{id: accountID, name: name, accessLevel: accessLevel, membership: membership}
	characters, err := s.loadCharacters(accountID)
	if err != nil {
		s.log.Error("loading characters", "account", name, "err", err)
		fail()
		return
	}
	a.characters = characters
	c.account = a
	c.state = inAuthed
	s.mu.Lock()
	s.accounts[accountID] = c
	s.mu.Unlock()
	w := wire.Packet(smL2authLoginCheck)
	w.D(0)
	w.S(name)
	c.send(w)
	s.log.Info("account logged in", "account", name, "characters", len(characters))
}

// loadCharacters loads an account's characters for the select screen, deleting
// those whose deletion time has passed.
func (s *Server) loadCharacters(accountID int32) ([]*character, error) {
	rows, err := s.store.Characters(accountID)
	if err != nil {
		return nil, err
	}
	var characters []*character
	for _, row := range rows {
		if row.Deletion != nil && !row.Deletion.After(time.Now()) {
			if err := s.store.DeleteCharacter(row.ID); err != nil {
				return nil, err
			}
			s.log.Info("character deleted", "character", row.Name)
			continue
		}
		ch, err := s.loadCharacter(row)
		if err != nil {
			return nil, err
		}
		characters = append(characters, ch)
	}
	slices.SortStableFunc(characters, byLastOnline)
	return characters, nil
}

// byLastOnline is Account.getSortedAccountsList's order: characters never in
// the game first, then the most recently played.
func byLastOnline(a, b *character) int {
	switch {
	case a.LastOnline.IsZero() && b.LastOnline.IsZero():
		return 0
	case b.LastOnline.IsZero():
		return 1
	case a.LastOnline.IsZero():
		return -1
	}
	return b.LastOnline.Compare(a.LastOnline)
}

func (s *Server) loadCharacter(row *store.Character) (*character, error) {
	ch := &character{Character: row}
	var err error
	if ch.appearance, err = s.store.Appearance(row.ID); err != nil {
		return nil, err
	}
	if ch.appearance == nil {
		ch.appearance = &store.Appearance{}
	}
	if ch.equipment, err = s.store.Items(row.ID, 0, true); err != nil {
		return nil, err
	}
	ch.legionID, err = s.store.LegionID(row.ID)
	return ch, err
}

func (c *conn) characterList(r *wire.Reader) {
	playOK2 := r.D()
	w := wire.Packet(smCharacterList)
	w.D(playOK2)
	w.C(byte(len(c.account.characters)))
	for _, ch := range c.account.characters {
		c.s.writeCharacterInfo(w, ch)
		w.B(make([]byte, 14))
	}
	c.send(w)
}

func (c *conn) checkNickname(r *wire.Reader) {
	w := wire.Packet(smNicknameCheckResponse)
	w.C(byte(c.s.nameCheck(r.S())))
	c.send(w)
}

// nameCheck is createOK, createInvalidName or createNameUsed.
func (s *Server) nameCheck(name string) int32 {
	if !s.names.valid(name) {
		return createInvalidName
	}
	used, err := s.store.NameUsed(name)
	if err != nil {
		return createDBError
	}
	if used {
		return createNameUsed
	}
	return createOK
}

// createCharacter is CM_CREATE_CHARACTER: the name, race, class and appearance.
func (c *conn) createCharacter(r *wire.Reader) {
	r.D() // playOK2
	r.S()
	name := convertName(r.S())
	r.B(42 - len([]rune(name))*2)
	ch := &store.Character{AccountID: c.account.id, AccountName: c.account.name, Name: name, Created: time.Now()}
	ch.Gender = map[int32]string{0: "MALE"}[r.D()]
	if ch.Gender == "" {
		ch.Gender = "FEMALE"
	}
	ch.Race = map[int32]string{0: "ELYOS"}[r.D()]
	if ch.Race == "" {
		ch.Race = "ASMODIANS"
	}
	class, known := className(r.D())
	a := readAppearance(r)
	fail := func(code int32) {
		w := wire.Packet(smCreateCharacter)
		w.D(code)
		w.B(make([]byte, 448))
		c.send(w)
	}
	if code := c.s.nameCheck(name); code != createOK {
		fail(code)
		return
	}
	if !known || !startingClass(class) {
		fail(createFailed)
		return
	}
	ch.Class = class

	s := c.s
	start := s.data.Initial.Elyos
	if ch.Race == "ASMODIANS" {
		start = s.data.Initial.Asmodians
	}
	ch.WorldID, ch.X, ch.Y, ch.Z, ch.Heading = start.MapID, start.X, start.Y, start.Z, start.Heading
	ch.ID = s.ids.nextID()

	var skills []store.Skill
	// With AL-Game's skill.autolearn off (as configured), only autolearn skills.
	for _, learn := range s.data.SkillsAt(class, ch.Race, 1) {
		if learn.Autolearn {
			skills = append(skills, store.Skill{ID: learn.SkillID, Level: learn.SkillLevel})
		}
	}
	var items, equipment []*store.Item
	for _, start := range s.data.Initial.Classes {
		if start.Class != class {
			continue
		}
		for _, it := range start.Items {
			template := s.data.Items[it.ID]
			if template == nil {
				continue
			}
			item := &store.Item{UniqueID: s.ids.nextID(), ItemID: it.ID, Count: it.Count, Owner: ch.ID}
			if template.IsEquipment() {
				item.Equipped, item.Slot = true, data.FirstSlot(template.Slot)
				equipment = append(equipment, item)
			}
			items = append(items, item)
		}
	}
	if err := s.store.CreateCharacter(ch, a, skills, items); err != nil {
		s.log.Error("creating character", "character", name, "err", err)
		s.ids.release(ch.ID)
		fail(createDBError)
		return
	}
	created := &character{Character: ch, appearance: a, equipment: equipment}
	c.account.characters = append(c.account.characters, created)
	w := wire.Packet(smCreateCharacter)
	w.D(createOK)
	s.writeCharacterInfo(w, created)
	c.send(w)
	s.log.Info("character created", "account", c.account.name, "character", name, "class", class, "race", ch.Race)
}

// startingClass reports whether class is one a character can be created as.
func startingClass(class string) bool {
	return class == "WARRIOR" || class == "SCOUT" || class == "MAGE" || class == "PRIEST"
}

func (c *conn) findCharacter(id int32) *character {
	for _, ch := range c.account.characters {
		if ch.ID == id {
			return ch
		}
	}
	return nil
}

// deleteCharacter schedules the deletion; until it happens the character can be restored.
func (c *conn) deleteCharacter(r *wire.Reader) {
	r.D() // playOK2
	ch := c.findCharacter(r.D())
	if ch == nil || ch.legionID != 0 {
		c.send(systemMessage(msgDeleteCharacterInLegion))
		return
	}
	if ch.Deletion == nil {
		at := time.Now().Add(deletionDelay)
		if err := c.s.store.SetDeletion(ch.ID, &at); err != nil {
			c.s.log.Error("scheduling deletion", "character", ch.Name, "err", err)
			return
		}
		ch.Deletion = &at
	}
	w := wire.Packet(smDeleteCharacter)
	w.D(0)
	w.D(ch.ID)
	w.D(ch.deletionSeconds())
	c.send(w)
}

func (c *conn) restoreCharacter(r *wire.Reader) {
	r.D() // playOK2
	id := r.D()
	ch := c.findCharacter(id)
	ok := ch != nil && (ch.Deletion == nil || ch.Deletion.After(time.Now()))
	if ok && ch.Deletion != nil {
		if c.s.store.SetDeletion(ch.ID, nil) != nil {
			ok = false
		} else {
			ch.Deletion = nil
		}
	}
	w := wire.Packet(smRestoreCharacter)
	if ok {
		w.D(0)
	} else {
		w.D(0x10)
	}
	w.D(id)
	c.send(w)
}

// quit is CM_QUIT: back to the server list (logout) or out of the game.
func (c *conn) quit(r *wire.Reader) {
	logout := r.C() == 1
	c.leaveWorld()
	c.state = inAuthed
	w := wire.Packet(smQuitResponse)
	w.D(1)
	w.C(0)
	if logout {
		c.send(w)
		return
	}
	c.close(w)
}

func readAppearance(r *wire.Reader) *store.Appearance {
	a := &store.Appearance{}
	a.Voice, a.SkinRGB, a.HairRGB, a.EyeRGB, a.LipRGB = r.D(), r.D(), r.D(), r.D(), r.D()
	c := func() int32 { return int32(r.C()) }
	a.Face, a.Hair, a.Deco, a.Tattoo = c(), c(), c(), c()
	r.C() // always 4
	a.FaceShape, a.Forehead, a.EyeHeight, a.EyeSpace, a.EyeWidth, a.EyeSize, a.EyeShape = c(), c(), c(), c(), c(), c(), c()
	a.EyeAngle, a.BrowHeight, a.BrowAngle, a.BrowShape, a.Nose, a.NoseBridge, a.NoseWidth = c(), c(), c(), c(), c(), c(), c()
	a.NoseTip, a.Cheek, a.LipHeight, a.MouthSize, a.LipSize, a.Smile, a.LipShape, a.JawHeight = c(), c(), c(), c(), c(), c(), c(), c()
	a.ChinJut, a.EarShape, a.HeadSize, a.Neck, a.NeckLength, a.ShoulderSize = c(), c(), c(), c(), c(), c()
	a.Torso, a.Chest, a.Waist, a.Hips, a.ArmThickness, a.HandSize, a.LegThickness, a.FootSize, a.FacialRate = c(), c(), c(), c(), c(), c(), c(), c(), c()
	r.C() // always 0
	a.ArmLength, a.LegLength, a.Shoulders = c(), c(), c()
	r.C() // always 0
	r.C()
	a.Height = r.F()
	return a
}
