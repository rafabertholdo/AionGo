package game

import (
	"slices"
	"strconv"
	"time"
	"unicode/utf16"

	"aionlightning/game/store"
	"aionlightning/wire"
)

// The small controllers and services: BindpointController, PostboxController, ClassChangeService, HTMLService,
// AnnouncementService and ExtractAction. StaticObjectController is empty in AL-Game and a static object sends the
// client nothing, so it has no port; ActionitemController is the quests' (its dialog starts with the quest engine).

func init() {
	// A bind stone and a postbox answer a dialog request; the rest is the quests'.
	next := handlers[cmShowDialog]
	handlers[cmShowDialog] = func(c *conn, r *wire.Reader) {
		peek := wire.NewReader(append([]byte(nil), r.Data...))
		id := peek.D()
		if peek.Err == nil && c.smallDialog(id) {
			return
		}
		next(c, r)
	}
	// With no npc, a dialog select is the class change's, if the quests didn't want it (CM_DIALOG_SELECT).
	selected := handlers[cmDialogSelect]
	handlers[cmDialogSelect] = func(c *conn, r *wire.Reader) {
		peek := wire.NewReader(slices.Clone(r.Data))
		id, dialog := peek.D(), peek.H()
		if peek.Err == nil && id == 0 && c.classChange(dialog) {
			return
		}
		selected(c, r)
	}
}

const (
	dialogPostbox = 18

	questionBindToLocation = 160012
	msgBindRegistered      = 1300670
	msgBindNoFee           = 1300686
	msgBindAlready         = 1300688
	chatPeriodNotice       = 0x1c
	chatSystemNoticeAuto   = 0x21
	levelUpdateBindPoint   = 2
	htmlChunk              = 32767 - 8 // HTMLService: UTF-16 units of html in a packet
	extractDelay           = 5 * time.Second
	enchantStoneBase       = 166000000
)

// smallDialog is BindpointController and PostboxController.onDialogRequest; it says whether the object was one.
func (c *conn) smallDialog(id int32) bool {
	p := c.player
	if p == nil {
		return false
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	o := p.seen[id]
	if o == nil || o.npc == nil {
		return false
	}
	switch o.npc.Type {
	case "POSTBOX":
		c.send(dialogWindow(o.id, dialogPostbox, 0))
	case "RESURRECT":
		s.bindDialog(p, o)
	default:
		return false
	}
	return true
}

// bindDialog is BindpointController.onDialogRequest: the player is asked to bind to the stone, for its price.
func (s *Server) bindDialog(p *player, o *object) {
	stone, ok := s.data.BindStones[o.npc.ID]
	if !ok {
		s.log.Info("There is no bind point template for npc", "npc", o.npc.ID)
		return
	}
	if p.BindPoint == stone.ID {
		p.conn.send(systemMessage(msgBindAlready))
		return
	}
	world := s.data.WorldMaps[p.WorldID].WorldType
	if !s.config.CrossFactionBinding {
		switch {
		case world == "ASMODAE" && p.Race == "ELYOS", world == "ABYSS" && p.Race == "ELYOS" && o.npc.ID == 700401:
			s.tell(p, "Elyos cannot bind in Asmodian territory.")
			return
		case world == "ELYSEA" && p.Race == "ASMODIANS", world == "ABYSS" && p.Race == "ASMODIANS" && o.npc.ID == 730071:
			s.tell(p, "Asmodians cannot bind in Elyos territory.")
			return
		}
	}
	if p.WorldID == prisonMap {
		s.tell(p, "You cannot bind here.")
		return
	}
	asked := p.putRequest(questionBindToLocation, func(accepted bool) {
		if !accepted || p.BindPoint == stone.ID {
			return
		}
		if !s.decreaseKinah(p, int64(stone.Price)) {
			p.conn.send(systemMessage(msgBindNoFee))
			return
		}
		p.BindPoint = stone.ID
		p.conn.send(s.bindPoint(p))
		update := wire.Packet(smLevelUpdate)
		update.D(p.ID)
		update.H(levelUpdateBindPoint)
		update.H(uint16(p.level))
		update.H(0)
		p.broadcast(update, true)
		p.conn.send(systemMessage(msgBindRegistered))
	})
	if asked {
		p.conn.send(questionWindow(questionBindToLocation, 0, strconv.Itoa(int(stone.Price))))
	}
}

// classNames are the classes by PlayerClass.ordinal(); 0, 3, 6 and 9 are the ones that pick a second class at level 9.
var classNames = []string{"WARRIOR", "GLADIATOR", "TEMPLAR", "SCOUT", "ASSASSIN", "RANGER", "MAGE", "SORCERER", "SPIRIT_MASTER",
	"PRIEST", "CLERIC", "CHANTER"}

// classChoices are the dialogs of ClassChangeService.changeClassToSelection: the class ordinal each one picks, by race.
var classChoices = map[string]map[uint16]int{
	"ELYOS":     {2376: 1, 2461: 2, 2717: 4, 2802: 5, 3058: 7, 3143: 8, 3399: 10, 3484: 11},
	"ASMODIANS": {3058: 1, 3143: 2, 3399: 4, 3484: 5, 3740: 7, 3825: 8, 4081: 10, 4166: 11},
}

// classQuests are the two quests the choice completes, by race: what the class change quest chain would have done.
var classQuests = map[string][2]int32{"ELYOS": {1006, 1007}, "ASMODIANS": {2008, 2009}}

// classChangeDialog is ClassChangeService.showClassChangeDialog: at level 9 a player of a first class is asked to pick.
func (s *Server) classChangeDialog(p *player) {
	if !s.config.SimpleSecondClass || p.level < 9 {
		return
	}
	first := slices.Index(classNames, p.Class)
	if first < 0 || first%3 != 0 {
		return
	}
	// Each class's dialog is 341 after the last; the Asmodians' start two classes later.
	dialog, quest := 2375+341*first/3, classQuests["ELYOS"][0]
	if p.Race == "ASMODIANS" {
		dialog, quest = dialog+682, classQuests["ASMODIANS"][0]
	}
	p.conn.send(dialogWindow(0, uint16(dialog), quest))
}

// classChange is ClassChangeService.changeClassToSelection, and says whether the dialog was a choice of class.
// AL-Game accepts any player's choice at any time (and always completes the quests); this one wants a player of the
// first class the choice comes from, at level 9, so nobody changes class twice by sending packets.
func (c *conn) classChange(dialog uint16) bool {
	p := c.player
	s := c.s
	if p == nil || !s.config.SimpleSecondClass {
		return false
	}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	choice, ok := classChoices[p.Race][dialog]
	if !ok {
		return false
	}
	first := slices.Index(classNames, p.Class)
	if p.level < 9 || first < 0 || (choice-1)/3*3 != first {
		return false // not this player's to make: the quests may want the dialog
	}
	s.setClass(p, classNames[choice])
	for _, id := range classQuests[p.Race] {
		s.completeQuest(p, id)
	}
	return true
}

// setClass is ClassChangeService.setClass: the new class's stats and skills, and the dialog closes.
func (s *Server) setClass(p *player, class string) {
	p.Class = class
	s.levelUp(p)
	p.conn.send(dialogWindow(0, 0, 0))
}

// completeQuest is ClassChangeService.addCompliteQuest: the quest is complete, whatever it was.
func (s *Server) completeQuest(p *player, id int32) {
	q := p.quest(id)
	action := byte(2)
	if q == nil {
		p.quests = append(p.quests, store.Quest{ID: id})
		q = &p.quests[len(p.quests)-1]
		action = 1
	}
	q.Status = "COMPLETE"
	if err := s.quests.SaveQuest(p.ID, *q); err != nil {
		s.log.Error("completing a quest", "quest", id, "err", err)
	}
	p.conn.send(questAccepted(action, *q))
}

// htmlPackets is HTMLService.sendData: SM_QUESTIONNAIRE in chunks of at most 32759 UTF-16 units, no more than 255.
func htmlPackets(id int32, html string) []*wire.Writer {
	units := utf16.Encode([]rune(html))
	count := len(units)/htmlChunk + 1
	if count > 255 {
		return nil
	}
	var packets []*wire.Writer
	for i := range count {
		part := units[min(i*htmlChunk, len(units)):min((i+1)*htmlChunk, len(units))]
		w := wire.Packet(smQuestionnaire)
		w.D(id)
		w.C(byte(i))
		w.C(byte(count))
		w.H(uint16(len(part) * 2))
		w.S(string(utf16.Decode(part)))
		packets = append(packets, w)
	}
	return packets
}

// showHTML is HTMLService.showHTML.
func (s *Server) showHTML(p *player, html string) {
	id := s.ids.nextID()
	defer s.ids.release(id)
	for _, w := range htmlPackets(id, html) {
		p.conn.send(w)
	}
}

// autoAnnouncement is SM_MESSAGE of AnnouncementService: a message sent by sender 1, in the chat type its row asks for.
func autoAnnouncement(chatType byte, sender, text string) *wire.Writer {
	w := wire.Packet(smMessage)
	w.C(chatType)
	w.C(0)
	w.D(1)
	switch chatType {
	case chatShout:
		w.S(sender)
		w.S(text)
		w.F(0)
		w.F(0)
		w.F(0)
	case chatGroupLeader:
		w.S(sender)
		w.S(text)
	default:
		w.H(0)
		w.S(text)
	}
	return w
}

// announcementType is Announcement.getChatType.
func announcementType(kind string) byte {
	switch kind {
	case "NORMAL":
		return chatPeriodNotice
	case "YELLOW":
		return chatAnnouncement
	case "SHOUT":
		return chatShout
	case "ORANGE":
		return chatGroupLeader
	}
	return chatSystemNoticeAuto
}

// sendAutoAnnouncement is one run of AnnouncementService's task: the message goes to every player it is meant for.
func (s *Server) sendAutoAnnouncement(a store.AutoAnnouncement) {
	chatType := announcementType(a.Type)
	// Shouts and orange messages carry the sender's name; the others put it in the text.
	named := chatType == chatShout || chatType == chatGroupLeader
	for _, p := range s.spawned {
		sender, text := "Automatic Announce", a.Text
		if a.Faction == "ELYOS" || a.Faction == "ASMODIANS" {
			if a.Faction != p.Race {
				continue
			}
			sender = "Asmodian Automatic Announce"
			if a.Faction == "ELYOS" {
				sender = "Elyos Automatic Announce"
			}
		}
		if !named {
			text = sender + ": " + a.Text
		}
		p.conn.send(autoAnnouncement(chatType, sender, text))
	}
}

// startAnnouncements is AnnouncementService.load: each row is repeated every delay seconds, the first one after it too.
func (s *Server) startAnnouncements(list []store.AutoAnnouncement) []*task {
	var tasks []*task
	for _, a := range list {
		if a.Delay <= 0 {
			continue
		}
		delay := time.Duration(a.Delay) * time.Second
		tasks = append(tasks, s.every(delay, delay, func() { s.sendAutoAnnouncement(a) }))
	}
	s.log.Info("Loaded announcements", "count", len(tasks))
	return tasks
}

// extractItem is ExtractAction.act: five seconds later the item is broken into enchantment stones.
func (s *Server) extractItem(p *player, extract, target *store.Item) {
	p.conn.send(itemUsageAnimation(p.ID, extract.UniqueID, extract.ItemID, int32(extractDelay/time.Millisecond), 0, 0))
	p.itemUse.cancel()
	p.itemUse = s.later(extractDelay, func() {
		if p.dead || itemIn(p.cube, extract.UniqueID) == nil || itemIn(p.cube, target.UniqueID) == nil {
			return
		}
		p.conn.send(itemUsageAnimation(p.ID, extract.UniqueID, extract.ItemID, 0, 1, 0))
		s.breakItem(p, target, extract)
	})
}

// breakItem is EnchantService.breakItem: the item and one extractor go, and stones come, by the item's quality and level.
func (s *Server) breakItem(p *player, target, extract *store.Item) {
	t := s.template(target)
	if t == nil {
		return
	}
	var number, spread int32
	switch t.Quality {
	case "COMMON", "JUNK":
		number, spread = rnd(1, 2), 5
	case "RARE":
		number, spread = rnd(1, 3), 10
	case "LEGEND", "MYTHIC":
		number, spread = rnd(1, 3), 15
	case "EPIC", "UNIQUE":
		number, spread = rnd(1, 3), 20
	}
	stone := enchantStoneBase + t.Level + rnd(0, spread)
	s.removeItem(p, target)
	s.decreaseItemCount(p, extract, 1)
	s.addItem(p, stone, int64(number))
}
