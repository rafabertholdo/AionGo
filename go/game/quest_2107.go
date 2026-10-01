package game

import "aionlightning/game/data"

func (c *conn) returnToSenderDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != 2107 {
		return
	}
	q := c.player.quest(2107)
	if q == nil {
		return
	}
	if q.Status == "START" && o.npc.ID == 203516 && q.Vars == 0 {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1352, 2107))
		} else if dialogID == 10000 && c.customQuestProgress(2107, 1, "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
		return
	}
	if o.npc.ID != 203512 {
		return
	}
	if q.Status == "START" {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 2375, 2107))
		} else if dialogID == 1009 && c.customQuestProgress(2107, 2, "REWARD") {
			c.s.removeItemsByID(c.player, script.ItemID, 1)
			c.send(dialogWindow(o.id, 5, 2107))
		}
		return
	}
	if q.Status == "REWARD" {
		if dialogID == 1009 {
			c.send(dialogWindow(o.id, 5, 2107))
		} else if dialogID == 17 {
			c.finishQuest(script, o.id, dialogID)
		}
	}
}
