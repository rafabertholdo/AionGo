package game

import "aionlightning/game/data"

func (c *conn) encroachersDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if script == nil || script.ID != encroachersQuestID || o == nil || o.npc == nil || o.npc.ID != encroachersReportNPCID {
		return false
	}
	quest := c.player.quest(encroachersQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		return c.finishAltgardStartupQuest(o, script, encroachersQuestID, dialogID)
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch dialogID {
	case 25:
		if variable == 0 {
			c.send(dialogWindow(o.id, 1011, encroachersQuestID))
			return true
		}
		if variable <= 5 {
			c.send(dialogWindow(o.id, 1352, encroachersQuestID))
			return true
		}
		if variable >= 5 {
			c.beginAltgardStartupQuest(encroachersQuestID, quest.Vars, "REWARD")
		}
	case 10000, 10001:
		if variable == 0 || variable == 5 {
			if c.beginAltgardStartupQuest(encroachersQuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	}
	return false
}

func (c *conn) encroachersKill(npcID int32) bool {
	if c == nil || c.player == nil || npcID != encroachersBruteNPCID {
		return false
	}
	quest := c.player.quest(encroachersQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if variable > 0 && variable < 4 {
		return c.beginAltgardStartupQuest(encroachersQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
	}
	if variable == 4 {
		return c.beginAltgardStartupQuest(encroachersQuestID, quest.Vars, "REWARD")
	}
	return false
}
