package game

import "aionlightning/game/data"

const forestOutlawQuestID int32 = 1139

// forestOutlawLevelUp unlocks the level 11 quest.
func (c *conn) forestOutlawLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(forestOutlawQuestID)
	template := c.s.data.Quests[forestOutlawQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking The Forest Outlaw", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

// forestOutlawKill implements both Java kill routes. The 210138 route needs
// nine kills, while the 210140 route can finish on its fifth kill.
func (c *conn) forestOutlawKill(npcID int32) bool {
	if c == nil || c.player == nil || (npcID != 210138 && npcID != 210140) {
		return false
	}
	quest := c.player.quest(forestOutlawQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch npcID {
	case 210138:
		if variable >= 1 && variable <= 8 {
			return c.customQuestProgress(forestOutlawQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
		}
		if variable == 9 {
			return c.customQuestProgress(forestOutlawQuestID, quest.Vars, "REWARD")
		}
	case 210140:
		if variable >= 1 && variable <= 4 {
			return c.customQuestProgress(forestOutlawQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
		}
		if variable == 5 {
			return c.customQuestProgress(forestOutlawQuestID, quest.Vars, "REWARD")
		}
	}
	return false
}

// forestOutlawDialog handles the militia report and fixed reward.
func (c *conn) forestOutlawDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != 203124 ||
		script == nil || script.ID != forestOutlawQuestID {
		return false
	}
	quest := c.player.quest(forestOutlawQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, forestOutlawQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			// The source default end handler accepts all dialog ids for this fixed reward.
			c.finishQuest(script, o.id, 17)
			return true
		default:
			return false
		}
	}
	if quest.Status != "START" || questVar(quest.Vars, 0) != 0 {
		return false
	}
	switch dialogID {
	case 25:
		c.send(dialogWindow(o.id, 1011, forestOutlawQuestID))
		return true
	case 10000, 10001:
		if c.customQuestProgress(forestOutlawQuestID, setQuestVar(quest.Vars, 0, 1), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	}
	return false
}

// forestOutlawShowDialog handles the standard reward click preview.
func (c *conn) forestOutlawShowDialog(o *object, script *data.QuestScript) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != 203124 ||
		script == nil || script.ID != forestOutlawQuestID {
		return false
	}
	quest := c.player.quest(forestOutlawQuestID)
	if quest == nil || quest.Status != "REWARD" {
		return false
	}
	c.send(dialogWindow(o.id, 5, forestOutlawQuestID))
	return true
}
