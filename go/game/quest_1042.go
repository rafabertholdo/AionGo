package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	keeperKaidanKeyQuestID  int32 = 1042
	keeperKaidanKeyStartNPC int32 = 203989
	keeperKaidanKeyEndNPC   int32 = 203901
	keeperKaidanKeyItemID   int32 = 182201018
)

func (c *conn) keeperKaidanKeyLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(keeperKaidanKeyQuestID)
	prerequisite := c.player.quest(1040)
	template := c.s.data.Quests[keeperKaidanKeyQuestID]
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Keeper of the Kaidan Key", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) keeperKaidanKeyDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != keeperKaidanKeyQuestID {
		return false
	}
	quest := c.player.quest(keeperKaidanKeyQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != keeperKaidanKeyEndNPC {
			return false
		}
		switch dialogID {
		case -1:
			c.send(dialogWindow(o.id, 10002, keeperKaidanKeyQuestID))
			return true
		case 1009:
			c.send(dialogWindow(o.id, 5, keeperKaidanKeyQuestID))
			return true
		default:
			if dialogID >= 8 && dialogID <= 17 {
				c.finishQuest(script, o.id, uint16(dialogID))
				return true
			}
		}
		return false
	}
	if quest.Status != "START" {
		return false
	}
	switch o.npc.ID {
	case keeperKaidanKeyStartNPC:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, keeperKaidanKeyQuestID))
				return true
			}
			// Java falls through from case 25 into case 1012 when variable is not zero.
			c.send(ascensionMovie(185))
			return false
		case 1012:
			c.send(ascensionMovie(185))
			return false
		case 10000:
			if variable == 0 && c.customQuestProgress(keeperKaidanKeyQuestID, setQuestVar(quest.Vars, 0, 1), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case keeperKaidanKeyEndNPC:
		switch dialogID {
		case 25:
			if variable == 2 {
				c.send(dialogWindow(o.id, 1352, keeperKaidanKeyQuestID))
				return true
			}
			// Java falls through from case 25 to the item turn-in branch when the variable differs.
			return c.keeperKaidanKeyTurnIn(o, script)
		case 33:
			return c.keeperKaidanKeyTurnIn(o, script)
		}
	}
	return false
}

func (c *conn) keeperKaidanKeyTurnIn(o *object, script *data.QuestScript) bool {
	if c.s.countItems(c.player, keeperKaidanKeyItemID) < 1 {
		c.send(dialogWindow(o.id, 1438, keeperKaidanKeyQuestID))
		return true
	}
	if !c.customQuestProgress(keeperKaidanKeyQuestID, c.player.quest(keeperKaidanKeyQuestID).Vars, "REWARD") {
		return false
	}
	c.s.removeItemsByID(c.player, keeperKaidanKeyItemID, 1)
	c.send(dialogWindow(o.id, 5, keeperKaidanKeyQuestID))
	return true
}

func (c *conn) keeperKaidanKeyItemUse(item *store.Item) {
	if c == nil || c.player == nil || item == nil || item.ItemID != keeperKaidanKeyItemID || c.player.cubeItem(item.UniqueID) != item {
		return
	}
	p := c.player
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 20, 1, 0), true)
	quest := p.quest(keeperKaidanKeyQuestID)
	c.keeperKaidanKeySetProgress(quest)
}

func (c *conn) keeperKaidanKeyKill(npcID int32) bool {
	if c == nil || c.player == nil || npcID != 212029 && npcID != 212033 {
		return false
	}
	quest := c.player.quest(keeperKaidanKeyQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) >= 2 {
		return false
	}
	return c.keeperKaidanKeySetProgress(quest)
}

func (c *conn) keeperKaidanKeySetProgress(quest *store.Quest) bool {
	if c == nil || c.player == nil || quest == nil {
		return false
	}
	next := *quest
	next.Vars = setQuestVar(next.Vars, 0, 2)
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("updating Keeper of the Kaidan Key", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}
