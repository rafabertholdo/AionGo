package game

import (
	"aionlightning/game/data"
	"aionlightning/wire"
)

// kerubThreatLevelUp unlocks the quest created by Kalio's Call.
func (c *conn) kerubThreatLevelUp() bool {
	if c.player == nil {
		return false
	}
	q := c.player.quest(1001)
	if q == nil || q.Status != "LOCKED" {
		return false
	}
	next := *q
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Kerub Threat", "err", err)
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) kerubThreatKill(npcID int32) bool {
	if c.player == nil || npcID != 210670 {
		return false
	}
	q := c.player.quest(1001)
	if q == nil || q.Status != "START" || q.Vars < 1 || q.Vars >= 6 {
		return false
	}
	return c.customQuestProgress(1001, q.Vars+1, "")
}

// kerubThreatDialog is _1001TheKerubThreat.onDialogEvent. The client reaches Muranes through the main menu (window
// 10, sent on a plain click) and its select 25; the opening pages 1012 and 1013 are not handled here, so the
// framework answers them with the window of the same number, and only 10000 accepts the quest.
func (c *conn) kerubThreatDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != 1001 {
		return
	}
	q := c.player.quest(1001)
	if q == nil {
		return
	}
	if q.Status == "REWARD" && o.npc.ID == 203067 {
		if dialogID == 1009 || dialogID == ^uint16(0) {
			c.send(dialogWindow(o.id, 5, 1001))
		} else if dialogID >= 8 && dialogID <= 17 {
			c.finishQuest(script, o.id, dialogID)
		}
		return
	}
	if q.Status != "START" || o.npc.ID != 203071 {
		return
	}
	switch dialogID {
	case 1012:
		c.send(kerubThreatMovie())
		c.dialogNotHandled() // Java returns false: the window 1012 follows the movie
	case 25:
		switch q.Vars {
		case 0:
			c.send(dialogWindow(o.id, 1011, 1001))
		case 6:
			c.send(dialogWindow(o.id, 1352, 1001))
		case 7:
			c.send(dialogWindow(o.id, 1693, 1001))
		}
	case 10000, 10001:
		c.dialogSilent()
		if q.Vars == 0 || q.Vars == 6 {
			if c.customQuestProgress(1001, q.Vars+1, "") {
				c.send(dialogWindow(o.id, 10, 0))
			}
		}
	case 33, 10002:
		c.dialogSilent()
		if q.Vars != 7 {
			return
		}
		items := c.s.countItems(c.player, 182200001)
		if items < 5 {
			c.send(dialogWindow(o.id, 1779, 1001))
		} else if dialogID == 33 {
			c.send(dialogWindow(o.id, 1694, 1001))
		} else {
			// Java removes every stack of this quest item before it moves the quest on.
			c.s.removeItemsByID(c.player, 182200001, items)
			if c.customQuestProgress(1001, 8, "REWARD") {
				c.send(dialogWindow(o.id, 10, 0))
			}
		}
	}
}

// kerubThreatShowDialog is the plain click (dialog -1): only the reward NPC answers, with the reward window.
func (c *conn) kerubThreatShowDialog(o *object, script *data.QuestScript) bool {
	if c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != 1001 {
		return false
	}
	if q := c.player.quest(1001); q != nil && q.Status == "REWARD" && o.npc.ID == 203067 {
		c.send(dialogWindow(o.id, 5, 1001))
		return true
	}
	return false
}

// SM_PLAY_MOVIE(0, 15) uses the same packet type as the Q1123 cutscene.
func kerubThreatMovie() *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(0)
	w.D(0)
	w.D(0)
	w.H(15)
	w.D(0)
	return w
}
