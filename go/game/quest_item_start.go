package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
	"time"
)

// delayedItemQuestUse is the shared three-second start-item animation for
// the eleven matching Sanctum and Pandaemonium Java handlers.
func (c *conn) delayedItemQuestUse(item *store.Item, script *data.QuestScript) bool {
	if c.player == nil || item == nil || script == nil || script.ItemUseDelay <= 0 || item.ItemID != script.ItemID || c.player.cubeItem(item.UniqueID) != item {
		return false
	}
	p := c.player
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, script.ItemUseDelay, 0, 0), true)
	c.s.later(time.Duration(script.ItemUseDelay)*time.Millisecond, func() {
		if p.conn != c || p.cubeItem(item.UniqueID) != item {
			return
		}
		p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0), true)
		c.send(dialogWindow(0, 4, script.ID))
	})
	return true
}

func (c *conn) delayedItemQuestDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if c.player == nil || o == nil || o.npc == nil || script == nil || script.ItemUseDelay <= 0 || o.npc.ID != script.EndNPC {
		return
	}
	q := c.player.quest(script.ID)
	if q == nil {
		return
	}
	if q.Status == "START" {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 2375, script.ID))
		} else if dialogID == 1009 && c.customQuestProgress(script.ID, 1, "REWARD") {
			c.s.removeItemsByID(c.player, script.ItemID, 1)
			c.send(dialogWindow(o.id, 5, script.ID))
		}
		return
	}
	if q.Status == "REWARD" {
		if dialogID == 1009 {
			c.send(dialogWindow(o.id, 5, script.ID))
		} else if dialogID >= 8 && dialogID <= 17 {
			c.finishQuestReward(script, o.id, 17, 0)
		}
	}
}

// questStartItemUse is Java's onItemUseEvent shared by the three item-started
// custom handlers. The item stays in the cube until that quest's NPC takes it.
func (c *conn) questStartItemUse(item *store.Item, script *data.QuestScript) bool {
	if c.player == nil || item == nil || script == nil || item.ItemID != script.ItemID || c.player.cubeItem(item.UniqueID) != item {
		return false
	}
	p := c.player
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 20, 1, 0), true)
	if q := p.quest(script.ID); q == nil || q.Status == "NONE" {
		c.send(dialogWindow(0, 4, script.ID))
	}
	return true
}

func (c *conn) itemStartedQuestDialog(script *data.QuestScript, dialogID uint16) bool {
	if script == nil || c.player == nil {
		return false
	}
	switch dialogID {
	case 1002:
		if c.s.countItems(c.player, script.ItemID) == 0 || !c.s.canStartQuest(c.player, script) {
			return false
		}
		c.startQuest(script, 0)
		c.send(dialogWindow(0, 0, 0))
		return true
	case 1003:
		c.send(dialogWindow(0, 0, 0))
		return true
	}
	return false
}
