package game

import "aionlightning/game/data"

func (c *conn) teachingALessonLevelUp() bool {
	if c.player == nil || c.player.level < 4 {
		return false
	}
	q := c.player.quest(2005)
	if q == nil || q.Status != "LOCKED" {
		return false
	}
	next := *q
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) teachingALessonEvent(o *object, script *data.QuestScript, dialogID int32) bool {
	if c.player == nil || o == nil || o.npc == nil || o.npc.ID != 203540 || script == nil || script.ID != 2005 {
		return false
	}
	q := c.player.quest(2005)
	if q == nil {
		return false
	}
	if q.Status == "REWARD" {
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, 2005))
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
	switch dialogID {
	case 25:
		if variable == 0 {
			c.send(dialogWindow(o.id, 1011, 2005))
		} else if variable == 1 {
			c.send(dialogWindow(o.id, 1352, 2005))
		} else {
			return false
		}
		return true
	case 1012:
		c.send(wheresRaeMovie(54))
		return false
	case 10000:
		if variable != 0 || !c.customQuestProgress(2005, setQuestVar(q.Vars, 0, 1), "") {
			return false
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	case 33:
		if variable != 1 {
			return false
		}
		if !c.s.hasQuestItems(c.player, c.s.data.Quests[2005]) {
			c.send(dialogWindow(o.id, 1693, 2005))
			return true
		}
		if !c.customQuestProgress(2005, q.Vars, "REWARD") {
			return false
		}
		c.s.removeItemsByID(c.player, 182203006, 5)
		c.send(dialogWindow(o.id, 5, 2005))
		return true
	}
	return false
}
