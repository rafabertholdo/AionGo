package game

import (
	"aionlightning/game/data"
)

const maskedLoiterersItemID int32 = 182200010

// maskedLoiterersLevelUp ports the Java LOCKED-to-START level-up event.
func (c *conn) maskedLoiterersLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	template := c.s.data.Quests[1012]
	q := c.player.quest(1012)
	if template == nil || q == nil || q.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *q
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Masked Loiterers", "err", err)
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

// maskedLoiterersEnterZone is Java's onEnterZoneEvent for Q1012. The caller
// scopes the event to Verteron (map 210030000) and only invokes on zone entry.
func (c *conn) maskedLoiterersEnterZone(zoneName string) bool {
	if c == nil || c.player == nil || zoneName != "Q1012" {
		return false
	}
	q := c.player.quest(1012)
	if q == nil || q.Status != "START" || questVar(q.Vars, 0) != 1 {
		return false
	}
	return c.customQuestProgress(1012, setQuestVar(q.Vars, 0, 2), "")
}

// maskedLoiterersDialog ports the conversation, five-mask turn-in and reward
// branch at Lieutenant Gallia. Java's switch deliberately falls through from
// dialog 25/10000 into the subsequent state checks; this keeps those paths.
func (c *conn) maskedLoiterersDialog(o *object, script *data.QuestScript, dialogID uint16) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.npc.ID != 203111 || script == nil || script.ID != 1012 {
		return false
	}
	q := c.player.quest(1012)
	if q == nil {
		return false
	}
	if q.Status == "REWARD" {
		switch dialogID {
		case 1009:
			c.send(dialogWindow(o.id, 5, 1012))
		case 8, 9, 10, 11, 12, 13, 14, 15, 16, 17:
			c.finishQuest(script, o.id, dialogID)
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
			c.send(dialogWindow(o.id, 1011, 1012))
			return true
		}
		if variable == 2 {
			c.send(dialogWindow(o.id, 1352, 1012))
			return true
		}
		if variable != 3 {
			return false
		}
		fallthrough
	case 10000:
		if variable == 0 {
			return c.customQuestProgress(1012, setQuestVar(q.Vars, 0, 1), "") && c.sendMaskedLoiterersClose(o)
		}
		if variable == 2 {
			c.send(dialogWindow(o.id, 1352, 1012))
			return true
		}
		if variable != 3 {
			return false
		}
		fallthrough
	case 33, 10001:
		if variable == 2 {
			return c.customQuestProgress(1012, setQuestVar(q.Vars, 0, 3), "") && c.sendMaskedLoiterersClose(o)
		}
		if variable != 3 {
			return false
		}
		if c.s.countItems(c.player, maskedLoiterersItemID) < 5 {
			c.send(dialogWindow(o.id, 1779, 1012))
			return true
		}
		if dialogID == 33 {
			c.send(dialogWindow(o.id, 1694, 1012))
			return true
		}
		if !c.customQuestProgress(1012, setQuestVar(q.Vars, 0, 4), "REWARD") {
			return false
		}
		c.s.removeItemsByID(c.player, maskedLoiterersItemID, c.s.countItems(c.player, maskedLoiterersItemID))
		c.sendMaskedLoiterersClose(o)
		return true
	default:
		return false
	}
}

func (c *conn) sendMaskedLoiterersClose(o *object) bool {
	c.send(dialogWindow(o.id, 10, 0))
	return true
}
