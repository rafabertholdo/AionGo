package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

const (
	dukakiOdiumQuestID  int32 = 1014
	dukakiOdiumItemID   int32 = 182200012
	dukakiCrystalItemID int32 = 182200011
)

func (c *conn) dukakiOdiumLevelUp() bool {
	if c == nil || c.player == nil || c.s.data.Quests[dukakiOdiumQuestID] == nil {
		return false
	}
	q := c.player.quest(dukakiOdiumQuestID)
	if q == nil || q.Status != "LOCKED" || c.player.level < c.s.data.Quests[dukakiOdiumQuestID].MinLevel {
		return false
	}
	next := *q
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Odium in the Dukaki Settlement", "err", err)
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

func dukakiOdiumMovie(id uint16) *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(0)
	w.D(0)
	w.D(0)
	w.H(id)
	w.D(0)
	return w
}

func (c *conn) dukakiOdiumDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != dukakiOdiumQuestID {
		return false
	}
	p := c.player
	q := p.quest(dukakiOdiumQuestID)
	if q == nil {
		return false
	}
	if q.Status == "REWARD" {
		if o.npc.ID != 203098 {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, script.ID))
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuest(script, o.id, uint16(dialogID))
		default:
			return false
		}
		return true
	}
	if q.Status != "START" {
		return false
	}
	variable := questVar(q.Vars, 0)
	advance := func(value int32, status string) bool {
		if !c.customQuestProgress(script.ID, setQuestVar(q.Vars, 0, value), status) {
			return false
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	}
	switch o.npc.ID {
	case 203129: // Pernos
		switch dialogID {
		case 25:
			page := uint16(0)
			switch variable {
			case 0:
				page = 1011
			case 10:
				page = 1352
			case 14:
				page = 1693
			default:
				return false
			}
			c.send(dialogWindow(o.id, page, script.ID))
			return true
		case 1013, 10000, 10001, 10002:
			// Java's cases fall through 1013 -> 10000 -> 10001 -> 10002 (only 1013 with var 0 and 10002 return).
			if dialogID == 1013 && variable == 0 {
				c.send(dukakiOdiumMovie(26))
				return false // Java: movie, then the echoed page
			}
			if dialogID <= 10000 && variable == 0 {
				return advance(1, "")
			}
			if dialogID <= 10001 && variable == 10 {
				return advance(11, "")
			}
			if variable == 14 {
				return advance(14, "REWARD")
			}
		}
	case 730020: // Hyan
		switch dialogID {
		case 25:
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, script.ID))
				return true
			}
		case 10001:
			if variable == 1 {
				return advance(2, "")
			}
		}
	case 700090: // Odium Refining Cauldron
		if dialogID != -1 || variable != 11 || o.useTask != nil || o.dead {
			return false
		}
		c.send(useObject(p.ID, o.id, 1))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
		o.useTask = c.s.later(3*time.Second, func() {
			o.useTask = nil
			current := p.quest(dukakiOdiumQuestID)
			if p.conn != c || !p.spawned || p.seen[o.id] != o || p.targetID != o.id || o.dead || current == nil || current.Status != "START" || questVar(current.Vars, 0) != 11 || c.s.countItems(p, dukakiCrystalItemID) == 0 {
				return
			}
			c.send(useObject(p.ID, o.id, 0))
			p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
			if template := c.s.data.Npcs[210739]; template != nil {
				spawn := &object{id: c.s.ids.nextID(), worldID: 210030000, x: 757.7, y: 2477.2, z: 217.4, homeX: 757.7, homeY: 2477.2, homeZ: 217.4, npc: template, interval: 60}
				c.s.initNpc(spawn)
				c.s.byID[spawn.id] = spawn
				c.s.addObject(spawn)
			}
		})
		return true
	}
	return false
}

func (c *conn) dukakiOdiumItemUse(item *store.Item, script *data.QuestScript) bool {
	if c == nil || c.player == nil || item == nil || script == nil || script.ID != dukakiOdiumQuestID || item.ItemID != dukakiOdiumItemID || c.player.cubeItem(item.UniqueID) != item {
		return false
	}
	p := c.player
	q := p.quest(dukakiOdiumQuestID)
	if q == nil || q.Status != "START" || questVar(q.Vars, 0) != 11 || p.zone == nil || p.zone.Name != "ODIUM_REFINING_CAULDRON" || c.s.countItems(p, dukakiCrystalItemID) == 0 {
		return false
	}
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 3000, 0, 0), true)
	c.s.later(3*time.Second, func() {
		current := p.quest(dukakiOdiumQuestID)
		if p.conn != c || current == nil || current.Status != "START" || questVar(current.Vars, 0) != 11 || p.cubeItem(item.UniqueID) != item || c.s.countItems(p, dukakiCrystalItemID) == 0 {
			return
		}
		if !c.customQuestProgress(dukakiOdiumQuestID, setQuestVar(current.Vars, 0, 14), "") {
			return
		}
		c.send(dukakiOdiumMovie(172))
		p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0), true)
		c.s.removeItemsByID(p, dukakiOdiumItemID, 1)
		c.s.removeItemsByID(p, dukakiCrystalItemID, 1)
	})
	return true
}
