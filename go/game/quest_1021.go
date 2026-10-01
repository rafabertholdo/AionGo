package game

import "aionlightning/game/data"

const (
	trandilasEggsQuestID int32 = 1021
	trandilasEggsNPCID   int32 = 203129
	trandilasEggsMobID   int32 = 210202
)

// trandilasEggsLevelUp unlocks the quest at its minimum level after Frillneck Hunt.
func (c *conn) trandilasEggsLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(trandilasEggsQuestID)
	previous := c.player.quest(1015)
	template := c.s.data.Quests[trandilasEggsQuestID]
	if quest == nil || previous == nil || template == nil || quest.Status != "LOCKED" || previous.Status != "COMPLETE" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Trandila's Eggs", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

// trandilasEggsKill completes the quest when the required Rakeclaw dies.
func (c *conn) trandilasEggsKill(o *object) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.npc.ID != trandilasEggsMobID {
		return false
	}
	quest := c.player.quest(trandilasEggsQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 1 {
		return false
	}
	if !c.customQuestProgress(trandilasEggsQuestID, quest.Vars, "REWARD") {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}

// trandilasEggsDialog ports Pernos's opening conversation and reward turn-in.
func (c *conn) trandilasEggsDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != trandilasEggsNPCID || script == nil || script.ID != trandilasEggsQuestID {
		return false
	}
	quest := c.player.quest(trandilasEggsQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, trandilasEggsQuestID))
		case dialogID >= 8 && dialogID <= 17:
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
		if variable == 0 {
			c.send(dialogWindow(o.id, 1011, trandilasEggsQuestID))
			return true
		}
		c.send(playMovie(27))
		return true
	case 1012:
		c.send(playMovie(27))
		return true
	case 10000, 10001:
		if variable != 0 {
			return false
		}
		if !c.customQuestProgress(trandilasEggsQuestID, setQuestVar(quest.Vars, 0, 1), "") {
			return false
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	default:
		return false
	}
}

func (c *conn) trandilasEggsShowDialog(o *object, script *data.QuestScript) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != trandilasEggsNPCID || script == nil || script.ID != trandilasEggsQuestID {
		return false
	}
	quest := c.player.quest(trandilasEggsQuestID)
	if quest == nil || quest.Status != "REWARD" {
		return false
	}
	c.send(dialogWindow(o.id, 5, trandilasEggsQuestID))
	return true
}
