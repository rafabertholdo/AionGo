package game

import (
	"math/rand/v2"

	"aionlightning/wire"
)

func init() {
	handlers[cmStartLoot] = (*conn).startLoot
	handlers[cmLootItem] = (*conn).lootItem
}

// dropItem is a drop that came out for a corpse: what, how many, and where it is on the list.
type dropItem struct {
	index int32
	item  int32
	count int64
	roll  *lootRoll
	free  bool // every eligible player passed
}

// lootState is what a monster left when it died (DropService's maps and DropNpc).
type lootState struct {
	items    []*dropItem
	allowed  map[int32]bool // who may loot; nil once anyone may
	looting  int32          // the player who has the list open
	group    *group
	eligible []*player // nearby group members at death, separate from corpse-looting rights
	active   *lootRoll
}

// The rate at which regular players find drops (rate.regular.drop).
const dropRate = 10

// dropPercent is DropRewardEnum: monsters of a much lower level than the player drop less.
func dropPercent(difference int32) int32 {
	table := map[int32]int32{-11: 0, -10: 1, -9: 10, -8: 20, -7: 30, -6: 40, -5: 50, -4: 60, -3: 90, -2: 100, -1: 100}
	if difference > 0 {
		return 100
	}
	if difference < -11 {
		return 0
	}
	if percent, ok := table[difference]; ok {
		return percent
	}
	return 100
}

// registerDrop is DropService.registerDrop: what the monster drops, chosen now, and the killer told there is loot.
// ponytail: alliances share drops with milestone 10.
func (s *Server) registerDrop(o *object, p *player) {
	s.registerDropFor(o, p, int32(p.level), []*player{p})
}

// registerDropFor is registerDrop for the players who may take the loot, with the level the drop rate goes by.
func (s *Server) registerDropFor(o *object, p *player, level int32, recipients []*player) {
	rate := float32(dropRate) * float32(dropPercent(o.npc.Level-level)) / 100
	loot := &lootState{allowed: map[int32]bool{}}
	for _, m := range recipients {
		loot.allowed[m.ID] = true
	}
	index := int32(1)
	for _, drop := range s.drops[o.npc.ID] {
		if rand.Float32()*100 < drop.Chance*rate {
			loot.items = append(loot.items, &dropItem{index: index, item: drop.ItemID, count: int64(rnd(drop.Min, drop.Max))})
			index++
		}
	}
	for _, script := range s.data.QuestDropsByNPC[o.npc.ID] {
		q := p.quest(script.ID)
		template := s.data.Quests[script.ID]
		if q == nil || q.Status != "START" || template == nil || s.hasQuestItems(p, template) {
			continue
		}
		for _, drop := range template.QuestDrops {
			if drop.NPCID == o.npc.ID && rand.Float32()*100 < drop.Chance {
				loot.items = append(loot.items, &dropItem{index: index, item: drop.ItemID, count: 1})
				index++
			}
		}
	}
	o.loot = loot
	for _, m := range recipients {
		m.conn.send(lootStatus(o.id, 0))
	}
}

func lootStatus(id int32, state byte) *wire.Writer {
	w := wire.Packet(smLootStatus)
	w.D(id)
	w.C(state)
	return w
}

func lootItemList(id int32, items []*dropItem) *wire.Writer {
	w := wire.Packet(smLootItemlist)
	w.D(id)
	w.C(byte(len(items)))
	for _, item := range items {
		w.C(byte(item.index))
		w.D(item.item)
		w.H(uint16(item.count))
		w.D(0)
	}
	return w
}

const (
	msgLootNoRight = 901338  // "You are not authorized to examine the corpse."
	msgLooting     = 1300829 // "Someone is already looting that."
)

// startLoot is CM_START_LOOT: the player opens or closes a corpse's list.
func (c *conn) startLoot(r *wire.Reader) {
	p := c.player
	id, action := r.D(), r.C()
	if p == nil || r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	switch action {
	case 0:
		s.openLoot(p, id)
	case 1:
		s.closeLoot(p, id)
	}
}

// openLoot is DropService.requestDropList.
func (s *Server) openLoot(p *player, id int32) {
	o := s.byID[id]
	if o == nil || o.loot == nil {
		return
	}
	loot := o.loot
	if loot.allowed != nil && !loot.allowed[p.ID] {
		p.conn.send(systemMessage(msgLootNoRight))
		return
	}
	if loot.looting != 0 {
		p.conn.send(systemMessage(msgLooting))
		return
	}
	loot.looting = p.ID
	p.conn.send(lootItemList(id, loot.items))
	p.conn.send(lootStatus(id, 2))
	p.state &^= stateActive
	p.state |= stateLooting
	p.broadcast(s.playerEmotionTo(p, emoteStartLoot, 0, id, 0, 0, 0, 0), true)
}

// closeLoot is DropService.requestDropList with close: the player stops looting, and what is left is free to all.
func (s *Server) closeLoot(p *player, id int32) {
	o := s.byID[id]
	if o == nil || o.loot == nil {
		return
	}
	if o.loot.looting != p.ID {
		return
	}
	o.loot.looting = 0
	p.state &^= stateLooting
	p.state |= stateActive
	p.broadcast(s.playerEmotionTo(p, emoteEndLoot, 0, id, 0, 0, 0, 0), true)
	if len(o.loot.items) == 0 {
		s.despawnNpc(o, true)
		return
	}
	o.broadcast(lootStatus(id, 0), true)
	o.loot.allowed = nil
}

// lootItem is CM_LOOT_ITEM: the player takes one of the corpse's drops.
func (c *conn) lootItem(r *wire.Reader) {
	p := c.player
	id, index := r.D(), r.C()
	if p == nil || r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.takeLoot(p, id, int32(index))
}

// takeLoot is DropService.requestDropItem.
func (s *Server) takeLoot(p *player, id, index int32) {
	o := s.byID[id]
	if o == nil || o.loot == nil || o.loot.looting != p.ID {
		return
	}
	loot := o.loot
	var taken *dropItem
	var at int
	for i, item := range loot.items {
		if index == item.index {
			taken, at = item, i
			break
		}
	}
	if taken == nil || loot.active != nil {
		return
	}
	if taken.roll != nil {
		if taken.roll.winner != p {
			p.conn.send(systemMessage(msgLootOtherOwner))
			return
		}
		s.awardLootRoll(o, taken.roll)
		return
	}
	if !taken.free && loot.group != nil && loot.group == p.group && s.beginLootRoll(o, taken) {
		return
	}
	if !s.addItem(p, taken.item, taken.count) {
		return
	}
	loot.items = append(loot.items[:at], loot.items[at+1:]...)
	if len(loot.items) != 0 {
		p.conn.send(lootItemList(id, loot.items))
		return
	}
	// resendDropList with nothing left: the loot window closes and the corpse goes.
	p.conn.send(lootStatus(id, 3))
	p.state &^= stateLooting
	p.state |= stateActive
	p.broadcast(s.playerEmotionTo(p, emoteEndLoot, 0, id, 0, 0, 0, 0), true)
	s.despawnNpc(o, true)
}
