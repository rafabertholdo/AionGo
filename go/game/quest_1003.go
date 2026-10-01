package game

import "aionlightning/game/data"

// illegalLoggingLevelUp unlocks the quest granted by Kalio's Call.
func (c *conn) illegalLoggingLevelUp() bool {
	if c.player == nil || c.player.level < 3 {
		return false
	}
	q := c.player.quest(1003)
	if q == nil || q.Status != "LOCKED" {
		return false
	}
	next := *q
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Illegal Logging", "err", err)
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) illegalLoggingKill(npcID int32) bool {
	if c.player == nil {
		return false
	}
	q := c.player.quest(1003)
	if q == nil || q.Status != "START" {
		return false
	}
	current := questVar(q.Vars, 0)
	switch npcID {
	case 210096, 210149, 210145, 210146, 210150, 210151, 210092, 210154:
		if current >= 1 && current <= 12 {
			return c.customQuestProgress(1003, setQuestVar(q.Vars, 0, current+1), "")
		}
	case 210160:
		if current >= 14 && current <= 15 {
			return c.customQuestProgress(1003, setQuestVar(q.Vars, 0, current+1), "")
		}
		if current == 16 {
			return c.customQuestProgress(1003, q.Vars, "REWARD")
		}
	}
	return false
}

func (c *conn) illegalLoggingDialog(o *object, script *data.QuestScript, dialogID int32) {
	if c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != 1003 || o.npc.ID != 203081 {
		return
	}
	q := c.player.quest(1003)
	if q == nil {
		return
	}
	if q.Status == "REWARD" {
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, 1003))
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuest(script, o.id, uint16(dialogID))
		}
		return
	}
	if q.Status != "START" {
		return
	}
	current := questVar(q.Vars, 0)
	switch dialogID {
	case 25:
		if current == 0 {
			c.send(dialogWindow(o.id, 1011, 1003))
		} else if current == 13 {
			c.send(dialogWindow(o.id, 1352, 1003))
		}
	case 10000, 10001:
		if (current == 0 || current == 13) && c.customQuestProgress(1003, setQuestVar(q.Vars, 0, current+1), "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
	}
}
