package game

import "aionlightning/game/data"

func (c *conn) observatoryDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != observatoryQuestID {
		return false
	}
	quest := c.player.quest(observatoryQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != observatoryReportNPCID {
			return false
		}
		if dialogID == -1 {
			c.send(dialogWindow(o.id, 2034, observatoryQuestID))
			return true
		}
		return c.finishAltgardStartupQuest(o, script, observatoryQuestID, dialogID)
	}
	if quest.Status != "START" || o.npc.ID != observatoryTalkNPCID {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch dialogID {
	case 25:
		pages := map[int32]uint16{0: 1011, 6: 1352, 7: 1693}
		if page, ok := pages[variable]; ok {
			c.send(dialogWindow(o.id, page, observatoryQuestID))
			return true
		}
	case 10000, 10001:
		if variable == 0 || variable == 6 {
			if c.beginAltgardStartupQuest(observatoryQuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case 33:
		if variable == 7 {
			template := c.s.data.Quests[observatoryQuestID]
			if !c.s.hasQuestItems(c.player, template) {
				c.send(dialogWindow(o.id, 1779, observatoryQuestID))
				return true
			}
			for _, item := range template.CollectItems {
				c.s.removeItemsByID(c.player, item.ID, item.Count)
			}
			if !c.beginAltgardStartupQuest(observatoryQuestID, quest.Vars, "REWARD") {
				return false
			}
			c.send(dialogWindow(o.id, 1694, observatoryQuestID))
			return true
		}
	}
	return false
}

func (c *conn) observatoryKill(npcID int32) bool {
	if c == nil || c.player == nil || (npcID != 210528 && npcID != 210721) {
		return false
	}
	quest := c.player.quest(observatoryQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if variable >= 6 {
		return false
	}
	return c.beginAltgardStartupQuest(observatoryQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
}
