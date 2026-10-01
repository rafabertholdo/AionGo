package game

import "aionlightning/game/data"

// itemTwoReportQuestDialog ports the five item-started Java quests with two
// report conversations before the final item turn-in.
func (c *conn) itemTwoReportQuestDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if c.player == nil || o == nil || o.npc == nil || script == nil {
		return
	}
	q := c.player.quest(script.ID)
	if q == nil {
		return
	}
	switch o.npc.ID {
	case script.MiddleNPC:
		if q.Status != "START" || q.Vars != 0 {
			return
		}
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1352, script.ID))
		} else if dialogID == 10000 && c.customQuestProgress(script.ID, 1, "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
	case script.MiddleNPC2:
		if q.Status != "START" || q.Vars != 1 {
			return
		}
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1693, script.ID))
		} else if dialogID == 10001 && c.customQuestProgress(script.ID, 2, "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
	case script.EndNPC:
		if q.Status == "START" {
			if dialogID == 25 {
				c.send(dialogWindow(o.id, 2375, script.ID))
			} else if dialogID == 1009 && c.customQuestProgress(script.ID, 1, "REWARD") {
				removeItemID := script.ItemID
				if script.ID == 2846 {
					removeItemID = 182207048 // Java removes this, not its start item.
				}
				c.s.removeItemsByID(c.player, removeItemID, 1)
				c.send(dialogWindow(o.id, 5, script.ID))
			}
		} else if q.Status == "REWARD" {
			if dialogID == 1009 {
				c.send(dialogWindow(o.id, 5, script.ID))
			} else if dialogID >= 8 && dialogID <= 17 {
				c.finishQuestReward(script, o.id, dialogID, 0)
			}
		}
	}
}
