package game

import "aionlightning/game/data"

// twoReportsQuestDialog handles the five Java quests that visit two report
// NPCs before the final reward NPC. Both reports use dialog 10000.
func (c *conn) twoReportsQuestDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if c.player == nil || o == nil || o.npc == nil || script == nil {
		return
	}
	var first, second, final int32
	switch script.ID {
	case 1553:
		first, second, final = 730051, 204500, 204584
	case 1620:
		first, second, final = 790000, 730001, 203125
	case 1578:
		first, second, final = 730024, 204560, 204579
	case 1605:
		first, second, final = 204530, 204501, 204577
	case 1483:
		first, second, final = 203940, 203944, 798127
	default:
		return
	}
	q := c.player.quest(script.ID)
	if o.npc.ID == script.StartNPC {
		if q == nil || q.Status == "NONE" {
			c.customQuestStart(o, script, dialogID, 0)
		}
		return
	}
	if q == nil {
		return
	}
	if o.npc.ID == first && q.Status == "START" && q.Vars == 0 {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1352, script.ID))
		} else if dialogID == 10000 && c.customQuestProgress(script.ID, 1, "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
		return
	}
	if o.npc.ID == second && q.Status == "START" && q.Vars == 1 {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1352, script.ID))
		} else if dialogID == 10000 && c.customQuestProgress(script.ID, 2, "") {
			page := uint16(1693)
			if script.ID == 1553 {
				page = 10
			}
			c.send(dialogWindow(o.id, page, 0))
		}
		return
	}
	if o.npc.ID != final {
		return
	}
	if q.Status == "START" {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 2375, script.ID))
		} else if dialogID == 1009 && q.Vars == 2 {
			vars := int32(3)
			if script.ID == 1553 {
				vars = 2
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
