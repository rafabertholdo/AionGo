package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	archonOfStormsQuestID        int32  = 1059
	archonOfStormsStartNPCID     int32  = 204505
	archonOfStormsSecondNPCID    int32  = 204533
	archonOfStormsItemNPCID      int32  = 204535
	archonOfStormsGeyserObjectID int32  = 700282
	archonOfStormsItemID         int32  = 182201619
	archonOfStormsModelID        int32  = 212000
	archonOfStormsMovieID        uint16 = 193
	archonOfStormsItemMovieID    uint16 = 192
	archonOfStormsZoneName              = "PATEMA_GEYSER"
)

func (c *conn) archonOfStormsLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(archonOfStormsQuestID)
	prerequisite := c.player.quest(1500)
	template := c.s.data.Quests[archonOfStormsQuestID]
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking The Archon of Storms", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) archonOfStormsDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != archonOfStormsQuestID {
		return false
	}
	quest := c.player.quest(archonOfStormsQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != archonOfStormsStartNPCID {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, archonOfStormsQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuest(script, o.id, uint16(dialogID))
			return true
		default:
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch o.npc.ID {
	case archonOfStormsStartNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, archonOfStormsQuestID))
				return true
			}
			fallthrough
		case 10000:
			if variable == 0 {
				return c.archonOfStormsAdvance(o, quest, 1)
			}
			return false
		}
	case archonOfStormsSecondNPCID:
		switch dialogID {
		case 25:
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, archonOfStormsQuestID))
				return true
			}
			if variable == 3 {
				c.send(dialogWindow(o.id, 2034, archonOfStormsQuestID))
				return true
			}
			fallthrough
		case 10001:
			if variable == 1 {
				return c.archonOfStormsAdvance(o, quest, 2)
			}
			fallthrough
		case 10003:
			if variable == 3 {
				return c.archonOfStormsAdvance(o, quest, 4)
			}
			return false
		}
	case archonOfStormsItemNPCID:
		switch dialogID {
		case 25:
			if variable == 4 {
				c.send(dialogWindow(o.id, 2375, archonOfStormsQuestID))
				return true
			}
			fallthrough
		case 10004:
			if variable != 4 {
				return false
			}
			if !c.customQuestProgress(archonOfStormsQuestID, setQuestVar(quest.Vars, 0, 5), "") {
				return false
			}
			if c.s.questRewardsFit(c.player, []data.QuestItem{{ID: archonOfStormsItemID, Count: 1}}) {
				c.s.addItem(c.player, archonOfStormsItemID, 1)
			}
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case archonOfStormsGeyserObjectID:
		if variable == 2 && dialogID == -1 {
			c.archonOfStormsActivateGeyser(o)
		}
	}
	return false
}

func (c *conn) archonOfStormsAdvance(o *object, quest *store.Quest, variable int32) bool {
	if !c.customQuestProgress(archonOfStormsQuestID, setQuestVar(quest.Vars, 0, variable), "") {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}

func (c *conn) archonOfStormsActivateGeyser(o *object) {
	if c == nil || c.player == nil || o == nil || o.dead || o.useTask != nil || c.player.targetID != o.id {
		return
	}
	p := c.player
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		quest := p.quest(archonOfStormsQuestID)
		if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead || quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 2 {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		c.send(playMovie(archonOfStormsMovieID))
	})
}

func (c *conn) archonOfStormsMovieEnd(movieID uint16) bool {
	if c == nil || c.player == nil || movieID != archonOfStormsMovieID {
		return false
	}
	p := c.player
	quest := p.quest(archonOfStormsQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 2 {
		return false
	}
	if !c.customQuestProgress(archonOfStormsQuestID, setQuestVar(quest.Vars, 0, 3), "") {
		return false
	}
	p.fx.set(effectShapeChange)
	p.transformed = archonOfStormsModelID
	p.broadcast(transformPacket(p), true)
	c.s.later(15*time.Second, func() {
		if p.conn != c || !p.spawned || p.transformed != archonOfStormsModelID {
			return
		}
		p.fx.unset(effectShapeChange)
		p.transformed = 0
		p.broadcast(transformPacket(p), true)
	})
	return true
}

func (c *conn) archonOfStormsItemUse(item *store.Item) bool {
	if c == nil || c.player == nil || item == nil || item.ItemID != archonOfStormsItemID || c.player.cubeItem(item.UniqueID) != item {
		return false
	}
	p := c.player
	quest := p.quest(archonOfStormsQuestID)
	if p.zone == nil || p.zone.Name != archonOfStormsZoneName || quest == nil {
		return false
	}
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 3000, 0, 0), true)
	c.s.later(3*time.Second, func() {
		current := p.quest(archonOfStormsQuestID)
		if p.conn != c || p.cubeItem(item.UniqueID) != item || p.zone == nil || p.zone.Name != archonOfStormsZoneName || current == nil || current.Status != "START" {
			return
		}
		p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0), true)
		c.send(playMovie(archonOfStormsItemMovieID))
		next := *current
		next.Vars = setQuestVar(current.Vars, 0, 5)
		next.Status = "REWARD"
		if err := c.s.quests.SaveQuest(p.ID, next); err != nil {
			c.s.log.Error("finishing The Archon of Storms", "err", err)
			return
		}
		*current = next
		c.s.removeItemsByID(p, archonOfStormsItemID, 1)
		c.send(questAccepted(2, next))
	})
	return true
}
