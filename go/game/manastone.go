package game

import (
	"strconv"

	"aionlightning/game/store"
	"aionlightning/wire"
)

func init() {
	handlers[cmManastone] = (*conn).manastone
}

const manastoneRemovalPrice int64 = 500

// manastone handles CM_MANASTONE: use an enchant stone/manastone or remove a socket at an NPC.
func (c *conn) manastone(r *wire.Reader) {
	action := r.C()
	r.C()
	targetID := r.D()
	var stoneID, supplementID int32
	var slot byte
	var npcID int32
	switch action {
	case 1, 2:
		stoneID, supplementID = r.D(), r.D()
	case 3:
		slot = r.C()
		r.C()
		r.H()
		npcID = r.D()
	}
	p := c.player
	if p == nil || r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if p.dead || s.restrictedInPrison(p, "manastone socketing") {
		return
	}
	switch action {
	case 1, 2:
		stone := p.cubeItem(stoneID)
		target := s.itemOf(p, targetID)
		if stone == nil || target == nil {
			p.conn.send(systemMessage(msgItemError))
			return
		}
		var supplement *store.Item
		if supplementID != 0 {
			supplement = p.cubeItem(supplementID)
			if supplement == nil {
				return
			}
		}
		if action == 1 {
			t := s.template(stone)
			if t == nil || stone.ItemID <= 166000000 || stone.ItemID >= 167000000 {
				return
			}
			s.startEnchanting(p, stone, target, supplement)
		} else {
			t := s.template(stone)
			if t == nil || stone.ItemID < 167000000 || stone.ItemID >= 168000000 {
				return
			}
			s.startEnchanting(p, stone, target, supplement)
		}
	case 3:
		object := s.byID[npcID]
		target := p.cubeItem(targetID)
		if object == nil || object.npc == nil || target == nil || int(slot) >= len(p.stones[targetID]) {
			return
		}
		if !s.decreaseKinah(p, manastoneRemovalPrice) {
			p.conn.send(systemMessage(msgNotEnoughKinah, strconv.FormatInt(manastoneRemovalPrice, 10)))
			return
		}
		s.removeManastone(p, target, int32(slot))
	}
}

func (s *Server) removeManastone(p *player, item *store.Item, slot int32) {
	stones := p.stones[item.UniqueID]
	if slot < 0 || int(slot) >= len(stones) {
		return
	}
	index := int(slot)
	databaseSlot := stones[index].Slot
	stones = append(stones[:index], stones[index+1:]...)
	if len(stones) == 0 {
		delete(p.stones, item.UniqueID)
	} else {
		p.stones[item.UniqueID] = stones
	}
	if err := s.items.DeleteStone(item.UniqueID, databaseSlot); err != nil {
		s.log.Error("removing a manastone", "item", item.ItemID, "slot", databaseSlot, "err", err)
	}
	s.itemChanged(p, item)
}
