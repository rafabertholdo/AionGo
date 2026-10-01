package game

import "aionlightning/game/data"

// simpleThreeNPCQuestDialog ports the Java start, report, and final NPC
// conversations shared by thirteen quests. Quest 2651 advances to variable 3;
// the other twelve advance to variable 2 at the final NPC.
func (c *conn) simpleThreeNPCQuestDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if c.player == nil || o == nil || o.npc == nil || script == nil {
		return
	}
	q := c.player.quest(script.ID)
	switch o.npc.ID {
	case script.StartNPC:
		if q == nil || (script.ID >= 1324 && script.ID <= 1909 && q.Status == "NONE") {
			c.customQuestStart(o, script, dialogID, 0)
		}
	case script.MiddleNPC:
		if q == nil || q.Status != "START" || q.Vars != 0 {
			return
		}
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1352, script.ID))
		} else if dialogID == 10000 && c.customQuestProgress(script.ID, 1, "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
	case script.EndNPC:
		if q == nil {
			return
		}
		if q.Status == "START" {
			if dialogID == 25 {
				c.send(dialogWindow(o.id, 2375, script.ID))
			} else if dialogID == 1009 {
				vars := int32(2)
				if script.ID == 2651 {
					vars = 3
				}
				if c.customQuestProgress(script.ID, vars, "REWARD") {
					c.send(dialogWindow(o.id, 5, script.ID))
				}
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
