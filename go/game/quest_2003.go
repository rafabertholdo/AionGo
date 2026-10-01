package game

import (
	"aionlightning/game/data"
)

// treasureOfTheDeceasedLevelUp unlocks the quest after Order of the Captain.
func (c *conn) treasureOfTheDeceasedLevelUp() bool {
	if c.player == nil || c.player.level < 2 {
		return false
	}
	q := c.player.quest(2003)
	previous := c.player.quest(2100)
	if q == nil || q.Status != "LOCKED" || previous == nil || previous.Status != "COMPLETE" {
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

func (c *conn) treasureOfTheDeceasedEvent(o *object, script *data.QuestScript, dialogID int32) bool {
	if c.player == nil || o == nil || o.npc == nil || o.npc.ID != 203539 || script == nil || script.ID != 2003 {
		return false
	}
	q := c.player.quest(2003)
	if q == nil {
		return false
	}
	if q.Status == "REWARD" {
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, 2003))
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
		if variable == 0 || variable == 1 {
			page := uint16(1011)
			if variable == 1 {
				page = 1352
			}
			c.send(dialogWindow(o.id, page, 2003))
			return true
		}
		fallthrough
	case 1012:
		c.send(wheresRaeMovie(53))
		return false
	case 10000:
		if variable == 0 {
			if !c.customQuestProgress(2003, setQuestVar(q.Vars, 0, 1), "") {
				return false
			}
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
		fallthrough
	case 33:
		if variable != 1 {
			return false
		}
		if !c.s.hasQuestItems(c.player, c.s.data.Quests[2003]) {
			c.send(dialogWindow(o.id, 1693, 2003))
			return true
		}
		if !c.customQuestProgress(2003, q.Vars, "REWARD") {
			return false
		}
		c.s.removeItemsByID(c.player, 182203004, 4)
		c.send(dialogWindow(o.id, 5, 2003))
		return true
	}
	return false
}
