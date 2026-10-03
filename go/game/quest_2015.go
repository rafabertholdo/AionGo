package game

import "aionlightning/game/data"

func (c *conn) takeTheInitiativeDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != takeTheInitiativeQuestID || o.npc.ID != takeInitiativeReportNPCID {
		return false
	}
	quest := c.player.quest(takeTheInitiativeQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		return c.finishAltgardStartupQuest(o, script, takeTheInitiativeQuestID, dialogID)
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch dialogID {
	case -1:
		if questVar(quest.Vars, 1) >= 1 && questVar(quest.Vars, 2) >= 5 && questVar(quest.Vars, 3) >= 5 {
			if !c.beginAltgardStartupQuest(takeTheInitiativeQuestID, setQuestVar(quest.Vars, 0, variable+1), "REWARD") {
				return false
			}
			c.send(dialogWindow(o.id, 1352, takeTheInitiativeQuestID))
			return true
		}
	case 25:
		if variable == 0 {
			c.send(dialogWindow(o.id, 1011, takeTheInitiativeQuestID))
			return true
		}
	case 10000:
		if c.beginAltgardStartupQuest(takeTheInitiativeQuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	}
	return false
}

func (c *conn) takeTheInitiativeKill(npcID int32) bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(takeTheInitiativeQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	varID, max := 0, int32(0)
	switch npcID {
	case 210510:
		varID, max = 1, 1
	case 210504:
		varID, max = 2, 5
	case 210506:
		varID, max = 3, 5
	default:
		return false
	}
	current := questVar(quest.Vars, varID)
	if current >= max {
		return false
	}
	return c.beginAltgardStartupQuest(takeTheInitiativeQuestID, setQuestVar(quest.Vars, varID, current+1), "")
}
