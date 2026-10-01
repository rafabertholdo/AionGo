package game

import "aionlightning/game/data"

const huntingLepharistRevolutionariesQuestID int32 = 1013

// huntingLepharistRevolutionariesLevelUp unlocks the level-10 Elyos quest.
func (c *conn) huntingLepharistRevolutionariesLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(huntingLepharistRevolutionariesQuestID)
	template := c.s.data.Quests[huntingLepharistRevolutionariesQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Hunting Lepharist Revolutionaries", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

// huntingLepharistRevolutionariesKill advances the two ordered kill stages.
func (c *conn) huntingLepharistRevolutionariesKill(npcID int32) bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(huntingLepharistRevolutionariesQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch npcID {
	case 210688:
		if variable < 1 || variable > 11 {
			return false
		}
		return c.customQuestProgress(huntingLepharistRevolutionariesQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
	case 210316:
		if variable != 12 {
			return false
		}
		return c.customQuestProgress(huntingLepharistRevolutionariesQuestID, quest.Vars, "REWARD")
	default:
		return false
	}
}

// huntingLepharistRevolutionariesDialog ports Erytes's conversation and reward turn-in.
func (c *conn) huntingLepharistRevolutionariesDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.npc.ID != 203126 || script == nil || script.ID != huntingLepharistRevolutionariesQuestID {
		return false
	}
	quest := c.player.quest(huntingLepharistRevolutionariesQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		switch dialogID {
		case -1, 1009:
			c.send(dialogWindow(o.id, 5, huntingLepharistRevolutionariesQuestID))
		case 8, 9, 10, 11, 12, 13, 14, 15, 16, 17:
			c.finishQuest(script, o.id, uint16(dialogID))
		default:
			return false
		}
		return true
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch dialogID {
	case 25:
		switch {
		case variable == 0:
			c.send(dialogWindow(o.id, 1011, huntingLepharistRevolutionariesQuestID))
		case variable == 11:
			c.send(dialogWindow(o.id, 1352, huntingLepharistRevolutionariesQuestID))
		case variable >= 12:
			if c.customQuestProgress(huntingLepharistRevolutionariesQuestID, quest.Vars, "REWARD") {
				c.send(dialogWindow(o.id, 2375, huntingLepharistRevolutionariesQuestID))
				return true
			}
			return false
		default:
			return false
		}
		return true
	case 1012:
		c.send(playMovie(25))
		c.send(dialogWindow(o.id, 1012, huntingLepharistRevolutionariesQuestID))
		return true
	case 10000, 10001:
		if dialogID == 10000 && variable == 0 || dialogID == 10001 && variable == 11 {
			if !c.customQuestProgress(huntingLepharistRevolutionariesQuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
				return false
			}
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	}
	return false
}
