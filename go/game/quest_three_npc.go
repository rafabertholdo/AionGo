package game

import "aionlightning/game/data"

// threeNPCQuestDialog ports thirty-four Java handlers with the same
// start, one to three reports, and optional final-NPC branches. Several repeat
// the start NPC as the final branch; that branch is unreachable there, too.
func (c *conn) threeNPCQuestDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if c.player == nil || o == nil || o.npc == nil || script == nil || script.MiddleNPC == 0 {
		return
	}
	q := c.player.quest(script.ID)
	if o.npc.ID == script.StartNPC {
		if q == nil || q.Status == "NONE" || q.Status == "COMPLETE" {
			c.customQuestStart(o, script, dialogID, 0)
			return
		}
		if q.Status == "START" {
			if dialogID == 25 {
				c.send(dialogWindow(o.id, 2375, script.ID))
			} else if dialogID == 1009 && c.customQuestProgress(script.ID, q.Vars, "REWARD") {
				c.send(dialogWindow(o.id, 10, 0))
			}
			return
		}
		if q.Status == "REWARD" {
			c.threeNPCQuestEnd(o, script, dialogID)
		}
		return
	}
	if q == nil {
		return
	}
	if o.npc.ID == script.MiddleNPC && q.Status == "START" && q.Vars == 0 {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1352, script.ID))
		} else if dialogID == 10000 && c.customQuestProgress(script.ID, 1, "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
		return
	}
	if o.npc.ID == script.MiddleNPC2 && q.Status == "START" && q.Vars == 1 {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1693, script.ID))
		} else if dialogID == 10001 && c.customQuestProgress(script.ID, 2, "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
		return
	}
	if o.npc.ID == script.MiddleNPC3 && q.Status == "START" && q.Vars == 2 {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 2034, script.ID))
		} else if dialogID == 10002 && c.customQuestProgress(script.ID, 3, "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
		return
	}
	if script.FinalNPC != 0 && o.npc.ID == script.FinalNPC {
		if q.Status == "START" {
			if dialogID == 25 {
				c.send(dialogWindow(o.id, 2375, script.ID))
			} else if dialogID == 1009 && c.customQuestProgress(script.ID, 3, "REWARD") {
				c.send(dialogWindow(o.id, 5, script.ID))
			}
		} else if q.Status == "REWARD" {
			c.threeNPCQuestEnd(o, script, dialogID)
		}
	}
}

func (c *conn) threeNPCQuestEnd(o *object, script *data.QuestScript, dialogID uint16) {
	if dialogID == 1009 {
		c.send(dialogWindow(o.id, 5, script.ID))
	} else if dialogID >= 8 && dialogID <= 17 {
		c.finishQuestReward(script, o.id, 17, 0)
	}
}
