package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	nymphsDiaryItemID  = 182200214
	nymphsLetterItemID = 182200226
	nymphsDressItemID  = 182200217
)

// nymphsGownItemUse is the diary's onItemUseEvent. The quest is accepted from
// the item dialog so the item can be consumed only after the player confirms.
func (c *conn) nymphsGownItemUse(item *store.Item, script *data.QuestScript) {
	if c.player == nil || item == nil || script == nil || item.ItemID != nymphsDiaryItemID || c.player.cubeItem(item.UniqueID) != item {
		return
	}
	c.player.broadcast(itemUsageAnimation(c.player.ID, item.UniqueID, item.ItemID, 20, 1, 0), true)
	if q := c.player.quest(script.ID); q == nil || q.Status == "NONE" {
		c.send(dialogWindow(0, 4, script.ID))
	}
}

// nymphsGownDialog ports the diary, Namus, Seirenia's clothes, and Asteros
// events from Java's _1114TheNymphsGown handler.
func (c *conn) nymphsGownDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c.player == nil || script == nil || script.ID != 1114 {
		return false
	}
	p := c.player
	if o == nil {
		if dialogID != 1002 || c.s.countItems(p, nymphsDiaryItemID) == 0 || !c.s.canStartQuest(p, script) {
			return false
		}
		c.startQuest(script, 0)
		q := p.quest(script.ID)
		if q == nil || q.Status != "START" {
			return true
		}
		c.s.removeItemsByID(p, nymphsDiaryItemID, 1)
		if !c.s.addItem(p, nymphsLetterItemID, 1) {
			c.send(systemMessage(msgInventoryFull))
			return true
		}
		c.send(dialogWindow(0, 0, 0))
		return true
	}
	if o.npc == nil {
		return false
	}
	q := p.quest(script.ID)
	if q == nil {
		return false
	}
	npcID := o.npc.ID
	if q.Status == "REWARD" {
		if npcID != 203075 && npcID != 203058 {
			return false
		}
		// Java: Namus answers in REWARD at var 4, Asteros at var 3 (the latter with defaultQuestEndDialog only).
		rewardIndex := 0
		if npcID == 203058 {
			rewardIndex = 1
		}
		if want := int32(4 - rewardIndex); questVar(q.Vars, 0) != want {
			return false
		}
		switch dialogID {
		case -1:
			window := uint16(2375)
			if npcID == 203058 {
				window = 5
			}
			c.send(dialogWindow(o.id, window, script.ID))
		case 1009: // Java: Namus sends window 6, Asteros (defaultQuestEndDialog) window 5
			window := uint16(5)
			if npcID == 203075 {
				window = 6
			}
			c.send(dialogWindow(o.id, window, script.ID))
		case 8, 9, 10, 11, 12, 13, 14, 15, 16, 17:
			c.finishQuestReward(script, o.id, uint16(dialogID), rewardIndex)
		default:
			return false
		}
		return true
	}
	if q.Status != "START" {
		return false
	}
	variable := questVar(q.Vars, 0)
	switch npcID {
	case 203075: // Namus
		if dialogID == -1 {
			return false
		}
		if dialogID == 25 {
			switch variable {
			case 0:
				c.send(dialogWindow(o.id, 1011, script.ID))
			case 2:
				c.send(dialogWindow(o.id, 1693, script.ID))
			case 3:
				c.send(dialogWindow(o.id, 2375, script.ID))
			default:
				return false
			}
			return true
		}
		if dialogID == 10000 && variable == 0 {
			if c.customQuestProgress(script.ID, setQuestVar(q.Vars, 0, 1), "") {
				c.s.removeItemsByID(p, nymphsLetterItemID, c.s.countItems(p, nymphsLetterItemID))
				c.send(dialogWindow(o.id, 10, 0))
			}
			return true
		}
		if dialogID == 10001 && variable == 2 {
			if c.customQuestProgress(script.ID, setQuestVar(q.Vars, 0, 3), "") {
				c.send(dialogWindow(o.id, 10, 0))
			}
			return true
		}
		if dialogID == 1009 && (variable == 2 || variable == 3) {
			if c.customQuestProgress(script.ID, setQuestVar(q.Vars, 0, 4), "REWARD") {
				c.s.removeItemsByID(p, nymphsDressItemID, c.s.countItems(p, nymphsDressItemID))
				c.send(dialogWindow(o.id, 6, script.ID))
			}
			return true
		}
	case 700008: // Seirenia's clothes
		if dialogID != -1 || variable != 1 || o.useTask != nil || o.dead {
			return false
		}
		c.send(useObject(p.ID, o.id, 1))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
		o.useTask = c.s.later(3*time.Second, func() {
			o.useTask = nil
			if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead {
				return
			}
			current := p.quest(script.ID)
			if current == nil || current.Status != "START" || questVar(current.Vars, 0) != 1 {
				return
			}
			c.send(useObject(p.ID, o.id, 0))
			p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
			if !c.s.addItem(p, nymphsDressItemID, 1) {
				c.send(systemMessage(msgInventoryFull))
				return
			}
			for _, known := range p.seen {
				if known.npc != nil && known.npc.ID == 203175 {
					c.s.addDamage(known, p, 50)
				}
			}
			c.customQuestProgress(script.ID, setQuestVar(current.Vars, 0, 2), "")
		})
		return true
	case 203058: // Asteros
		if variable != 3 {
			return false
		}
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 2034, script.ID))
			return true
		}
		if dialogID == 10001 {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
		if dialogID == 10002 {
			if c.customQuestProgress(script.ID, q.Vars, "REWARD") {
				c.s.removeItemsByID(p, nymphsDressItemID, c.s.countItems(p, nymphsDressItemID))
				c.send(dialogWindow(o.id, 5, script.ID))
			}
			return true
		}
	}
	return false
}
