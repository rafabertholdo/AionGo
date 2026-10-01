package game

import "aionlightning/game/data"

func (c *conn) lostAxeDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if c.player == nil || o == nil || o.npc == nil || o.npc.ID != 203075 || script == nil || script.ID != 1107 {
		return
	}
	q := c.player.quest(1107)
	if q == nil {
		return
	}
	if q.Status == "START" {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 2375, 1107))
		} else if dialogID == 1009 && c.customQuestProgress(1107, 1, "REWARD") {
			c.s.removeItemsByID(c.player, script.ItemID, 1)
			c.send(dialogWindow(o.id, 5, 1107))
		}
		return
	}
	if q.Status == "REWARD" {
		if dialogID == 1009 {
			c.send(dialogWindow(o.id, 5, 1107))
		} else if dialogID == 17 {
			c.finishQuest(script, o.id, dialogID)
		}
	}
}
