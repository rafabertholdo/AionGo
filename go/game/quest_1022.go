package game

import "aionlightning/game/data"

const krallDesecrationQuestID int32 = 1022

// krallDesecrationLevelUp unlocks the quest after level 14 and completion of
// Held Sacred, matching the Java level-up handler and quest prerequisite.
func (c *conn) krallDesecrationLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(krallDesecrationQuestID)
	previous := c.player.quest(1017)
	template := c.s.data.Quests[krallDesecrationQuestID]
	if quest == nil || previous == nil || template == nil || quest.Status != "LOCKED" ||
		previous.Status != "COMPLETE" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Krall Desecration", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

// krallDesecrationKill advances the five ordered kills, then opens the reward.
func (c *conn) krallDesecrationKill(npcID int32) bool {
	if c == nil || c.player == nil || npcID != 210178 {
		return false
	}
	quest := c.player.quest(krallDesecrationQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if variable < 1 || variable > 5 {
		return false
	}
	status := ""
	nextVars := setQuestVar(quest.Vars, 0, variable+1)
	if variable == 5 {
		status = "REWARD"
		nextVars = quest.Vars
	}
	return c.customQuestProgress(krallDesecrationQuestID, nextVars, status)
}

// krallDesecrationDialog handles the captain's opening, report progress, and
// fixed reward turn-in.
func (c *conn) krallDesecrationDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != 203178 ||
		script == nil || script.ID != krallDesecrationQuestID {
		return false
	}
	quest := c.player.quest(krallDesecrationQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, krallDesecrationQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			// Java accepts every default reward-dialog index for a fixed reward.
			// finishQuest's Go helper uses 17 to select the fixed reward branch.
			c.finishQuest(script, o.id, 17)
			return quest.Status == "COMPLETE"
		default:
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	if questVar(quest.Vars, 0) != 0 {
		return false
	}
	switch dialogID {
	case 25:
		c.send(dialogWindow(o.id, 1011, krallDesecrationQuestID))
		return true
	case 10000, 10001:
		if c.customQuestProgress(krallDesecrationQuestID, setQuestVar(quest.Vars, 0, 1), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	}
	return false
}

// krallDesecrationShowDialog is the captain's default reward preview.
func (c *conn) krallDesecrationShowDialog(o *object, script *data.QuestScript) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != 203178 ||
		script == nil || script.ID != krallDesecrationQuestID {
		return false
	}
	quest := c.player.quest(krallDesecrationQuestID)
	if quest == nil || quest.Status != "REWARD" {
		return false
	}
	c.send(dialogWindow(o.id, 5, krallDesecrationQuestID))
	return true
}
