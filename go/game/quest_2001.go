package game

import (
	"aionlightning/game/data"
	"aionlightning/wire"
)

// thinkingAheadLevelUp unlocks the follow-up quest created by quest 2100.
func (c *conn) thinkingAheadLevelUp() bool {
	if c.player == nil {
		return false
	}
	q := c.player.quest(2001)
	if q == nil || q.Status != "LOCKED" {
		return false
	}
	next := *q
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Thinking Ahead", "err", err)
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

// thinkingAheadEvent handles the two talk targets and the two required kills.
// A kill passes kill=true; Java's show-dialog event uses dialogID=-1.
func (c *conn) thinkingAheadEvent(o *object, script *data.QuestScript, dialogID int32, kill bool) bool {
	if c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != 2001 {
		return false
	}
	q := c.player.quest(2001)
	if q == nil {
		return false
	}
	npcID := o.npc.ID
	variable := questVar(q.Vars, 0)
	if kill {
		if q.Status != "START" || (npcID != 210368 && npcID != 210369) {
			return false
		}
		if variable >= 3 && variable < 8 {
			return c.customQuestProgress(2001, setQuestVar(q.Vars, 0, variable+1), "")
		}
		if variable == 8 {
			return c.customQuestProgress(2001, q.Vars, "REWARD")
		}
		return false
	}
	if q.Status == "REWARD" && npcID == 203518 {
		if dialogID == -1 {
			c.send(dialogWindow(o.id, 2034, 2001))
			return true
		}
		if dialogID == 1009 {
			c.send(dialogWindow(o.id, 5, 2001))
			return true
		}
		if dialogID >= 8 && dialogID <= 17 {
			c.finishQuest(script, o.id, uint16(dialogID))
			return true
		}
		return false
	}
	if q.Status != "START" {
		return false
	}
	if npcID == 700093 {
		return variable == 1 && dialogID == -1
	}
	if npcID != 203518 {
		return false
	}
	switch dialogID {
	case 25:
		var page uint16
		switch variable {
		case 0:
			page = 1011
		case 1:
			page = 1352
		case 2:
			page = 1694
		default:
			return false
		}
		c.send(dialogWindow(o.id, page, 2001))
		return true
	case 1012:
		c.send(thinkingAheadMovie())
		c.dialogNotHandled() // Java returns false: window 1012 follows the movie
		return false
	case 10000, 10002:
		if variable == 1 {
			return c.thinkingAheadCollect(o, q.Vars)
		}
		if variable != 0 && variable != 2 {
			return false
		}
		if !c.customQuestProgress(2001, setQuestVar(q.Vars, 0, variable+1), "") {
			return false
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	case 33:
		if variable != 1 {
			return false
		}
		return c.thinkingAheadCollect(o, q.Vars)
	}
	return false
}

func (c *conn) thinkingAheadCollect(o *object, vars int32) bool {
	if !c.s.hasQuestItems(c.player, c.s.data.Quests[2001]) {
		c.send(dialogWindow(o.id, 1693, 2001))
		return true
	}
	if !c.customQuestProgress(2001, setQuestVar(vars, 0, 2), "") {
		return false
	}
	c.s.removeItemsByID(c.player, 182203002, 4)
	c.send(dialogWindow(o.id, 1694, 2001))
	return true
}

func thinkingAheadMovie() *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(0)
	w.D(0)
	w.D(0)
	w.H(51)
	w.D(0)
	return w
}
