package game

import "aionlightning/game/data"

func (c *conn) dangerFromAboveLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	q := c.player.quest(1011)
	previous := c.player.quest(1130)
	if q == nil || q.Status != "LOCKED" || previous == nil || previous.Status != "COMPLETE" {
		return false
	}
	next := *q
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Danger From Above", "err", err)
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) dangerFromAboveKill(o *object) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.npc.ID != 700091 {
		return false
	}
	q := c.player.quest(1011)
	if q == nil || q.Status != "START" {
		return false
	}
	if q.Vars > 0 && q.Vars < 4 {
		return c.customQuestProgress(1011, q.Vars+1, "")
	}
	if q.Vars == 4 && c.customQuestProgress(1011, q.Vars, "REWARD") {
		c.send(dialogWindow(o.id, 10, 0))
		return true
	}
	return false
}

func (c *conn) dangerFromAboveDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != 1011 {
		return false
	}
	q := c.player.quest(1011)
	if q == nil {
		return false
	}
	if q.Status == "REWARD" {
		if o.npc.ID != 203109 {
			return false
		}
		switch {
		case dialogID == -1:
			c.send(dialogWindow(o.id, 1693, 1011))
		case dialogID == 1009:
			c.send(dialogWindow(o.id, 5, 1011))
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuest(script, o.id, uint16(dialogID))
		default:
			return false
		}
		return true
	}
	if q.Status != "START" {
		return false
	}
	variable := questVar(q.Vars, 0)
	switch o.npc.ID {
	case 203109:
		switch dialogID {
		case 25:
			if variable != 0 {
				return false
			}
			c.send(dialogWindow(o.id, 1011, 1011))
			return true
		case 10000:
			if variable != 0 || !c.customQuestProgress(1011, setQuestVar(q.Vars, 0, 1), "") {
				return false
			}
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case 203122:
		switch dialogID {
		case 25:
			if variable != 1 {
				return false
			}
			c.send(dialogWindow(o.id, 1352, 1011))
			return true
		case 1353:
			if variable != 1 {
				return false
			}
			c.send(playMovie(24))
			return false
		case 10001:
			if variable != 1 || !c.customQuestProgress(1011, setQuestVar(q.Vars, 0, 2), "") {
				return false
			}
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	}
	return false
}
