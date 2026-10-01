package game

import "aionlightning/game/data"

// hitThemWhereItHurtsLevelUp unlocks the quest once its level is met.
func (c *conn) hitThemWhereItHurtsLevelUp() bool {
	if c.player == nil || c.player.level < c.s.data.Quests[2006].MinLevel {
		return false
	}
	q := c.player.quest(2006)
	if q == nil || q.Status != "LOCKED" {
		return false
	}
	next := *q
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Hit Them Where it Hurts", "err", err)
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) hitThemWhereItHurtsDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != 2006 {
		return false
	}
	q := c.player.quest(2006)
	if q == nil {
		return false
	}
	if q.Status == "REWARD" && o.npc.ID == 203516 {
		switch {
		case dialogID == -1:
			c.send(dialogWindow(o.id, 1693, 2006))
		case dialogID == 1009:
			c.send(dialogWindow(o.id, 5, 2006))
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
	if o.npc.ID == 700095 {
		return q.Vars == 1 && dialogID == -1
	}
	if o.npc.ID != 203540 {
		return false
	}
	switch dialogID {
	case 25:
		if q.Vars == 0 || q.Vars == 1 {
			page := uint16(1011)
			if q.Vars == 1 {
				page = 1352
			}
			c.send(dialogWindow(o.id, page, 2006))
			return true
		}
	case 10000:
		if q.Vars == 0 && c.customQuestProgress(2006, 1, "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case 33:
		if q.Vars != 1 {
			return false
		}
		if !c.s.hasQuestItems(c.player, c.s.data.Quests[2006]) {
			c.send(dialogWindow(o.id, 1353, 2006))
			return true
		}
		if c.customQuestProgress(2006, q.Vars, "REWARD") {
			c.s.removeItemsByID(c.player, 182203008, 7)
			c.send(dialogWindow(o.id, 1693, 2006))
			return true
		}
	}
	return false
}
