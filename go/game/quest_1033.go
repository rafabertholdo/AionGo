package game

import "aionlightning/game/data"

const (
	satalocasHeartQuestID int32 = 1033
	satalocasHeartNPCID   int32 = 203900
	satalocaKimeiaNPCID   int32 = 203996
	archonDrakeNPCID      int32 = 210799
)

func (c *conn) satalocasHeartLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(satalocasHeartQuestID)
	template := c.s.data.Quests[satalocasHeartQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Sataloca's Heart", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

// satalocasHeartKill matches Java's gated kill event: the ten Drakes only count
// after Kimeia's movie sets variable 10, then further kills re-send the same state.
func (c *conn) satalocasHeartKill(npcID int32) bool {
	if c == nil || c.player == nil || npcID != archonDrakeNPCID {
		return false
	}
	quest := c.player.quest(satalocasHeartQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if variable < 10 {
		return false
	}
	if variable == 10 {
		return c.customQuestProgress(satalocasHeartQuestID, setQuestVar(quest.Vars, 0, 11), "")
	}
	return c.customQuestProgress(satalocasHeartQuestID, quest.Vars, "")
}

func (c *conn) satalocasHeartDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != satalocasHeartQuestID {
		return false
	}
	quest := c.player.quest(satalocasHeartQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if o.npc.ID == satalocasHeartNPCID {
		if quest.Status == "REWARD" {
			switch {
			case dialogID == -1 || dialogID == 1009:
				c.send(dialogWindow(o.id, 5, satalocasHeartQuestID))
				return true
			case dialogID >= 8 && dialogID <= 11 || dialogID == 17:
				c.finishQuestReward(script, o.id, uint16(dialogID), 0)
				return true
			default:
				return false
			}
		}
		if quest.Status != "START" || variable != 0 {
			return false
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 1011, satalocasHeartQuestID))
			return true
		case 10000:
			if c.customQuestProgress(satalocasHeartQuestID, setQuestVar(quest.Vars, 0, 1), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		case 1007:
			c.send(dialogWindow(o.id, 4, satalocasHeartQuestID))
			return true
		case 1003:
			c.send(dialogWindow(o.id, 1004, satalocasHeartQuestID))
			return true
		}
		return false
	}
	if o.npc.ID != satalocaKimeiaNPCID || quest.Status != "START" {
		return false
	}
	switch {
	case variable == 1 && dialogID == 25:
		c.send(dialogWindow(o.id, 1693, satalocasHeartQuestID))
		return true
	case variable == 1 && dialogID == 10002:
		if c.customQuestProgress(satalocasHeartQuestID, setQuestVar(quest.Vars, 0, 10), "") {
			c.send(dialogWindow(o.id, 10, 0))
			c.send(ascensionMovie(42))
			return true
		}
	case variable == 11 && dialogID == 25:
		if c.customQuestProgress(satalocasHeartQuestID, setQuestVar(quest.Vars, 0, 12), "REWARD") {
			c.send(dialogWindow(o.id, 2205, satalocasHeartQuestID))
			return true
		}
	}
	return false
}
