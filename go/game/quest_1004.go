package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

func (c *conn) neutralizingOdiumLevelUp() bool {
	if c.player == nil || c.player.level < 4 {
		return false
	}
	q := c.player.quest(1004)
	if q == nil || q.Status != "LOCKED" {
		return false
	}
	next := *q
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Neutralizing Odium", "err", err)
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

func neutralizingOdiumMovie() *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(0)
	w.D(0)
	w.D(0)
	w.H(19)
	w.D(0)
	return w
}

func (c *conn) neutralizingOdiumDialog(o *object, script *data.QuestScript, dialogID int32) {
	if c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != 1004 {
		return
	}
	p := c.player
	q := p.quest(1004)
	if q == nil {
		return
	}
	if q.Status == "REWARD" {
		if o.npc.ID == 203067 {
			switch {
			case dialogID == -1 || dialogID == 1009:
				c.send(dialogWindow(o.id, 5, 1004))
			case dialogID >= 8 && dialogID <= 17:
				c.finishQuest(script, o.id, uint16(dialogID))
			}
		}
		return
	}
	if q.Status != "START" {
		return
	}
	variable := questVar(q.Vars, 0)
	send := func(page uint16) { c.send(dialogWindow(o.id, page, 1004)) }
	advance := func(next int32, status string) {
		if c.customQuestProgress(1004, setQuestVar(q.Vars, 0, next), status) {
			c.send(dialogWindow(o.id, 10, 0))
		}
	}
	switch o.npc.ID {
	case 203082:
		switch dialogID {
		case 25:
			if variable == 0 {
				send(1011)
			} else if variable == 5 {
				send(2034)
			}
		case 1013:
			if variable == 0 {
				c.send(neutralizingOdiumMovie())
				c.dialogNotHandled() // Java returns false: window 1013 follows the movie
			}
		case 10000:
			if variable == 0 {
				advance(1, "")
			}
		case 10002:
			if variable == 5 {
				advance(5, "REWARD")
			}
		}
	case 700030:
		if (variable != 1 && variable != 4) || dialogID != -1 || o.useTask != nil {
			return
		}
		if variable == 1 && c.s.countItems(p, 182200005) == 0 && !c.s.questRewardsFit(p, []data.QuestItem{{ID: 182200005, Count: 1}}) {
			c.send(systemMessage(msgInventoryFull))
			return
		}
		if variable == 4 && c.s.countItems(p, 182200006) == 0 {
			return
		}
		c.send(useObject(p.ID, o.id, 1))
		c.dialogNotHandled() // Java's -1 case returns false, so the main menu is sent as well
		p.broadcast(c.s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
		o.useTask = c.s.later(3*time.Second, func() {
			o.useTask = nil
			current := p.quest(1004)
			if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead || p.targetID != o.id || current == nil || current.Status != "START" {
				return
			}
			stage := questVar(current.Vars, 0)
			if stage != 1 && stage != 4 {
				return
			}
			if stage == 4 && c.s.countItems(p, 182200006) == 0 {
				return
			}
			if stage == 1 && c.s.countItems(p, 182200005) == 0 && !c.s.questRewardsFit(p, []data.QuestItem{{ID: 182200005, Count: 1}}) {
				c.send(systemMessage(msgInventoryFull))
				return
			}
			if !c.customQuestProgress(1004, setQuestVar(current.Vars, 0, stage+1), "") {
				return
			}
			c.send(useObject(p.ID, o.id, 0))
			p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
			if stage == 1 && c.s.countItems(p, 182200005) == 0 {
				c.s.addItem(p, 182200005, 1)
			} else if stage == 4 {
				c.s.removeItemsByID(p, 182200006, 1)
			}
		})
	case 790001:
		switch dialogID {
		case 25:
			switch variable {
			case 2:
				send(1352)
			case 3:
				send(1693)
			case 11:
				send(1694)
			}
		case 10001:
			if variable == 2 {
				advance(3, "")
			}
		case 33:
			if variable != 3 && variable != 11 {
				return
			}
			if !c.s.hasQuestItems(p, c.s.data.Quests[1004]) {
				send(1779)
			} else if variable == 11 || c.customQuestProgress(1004, setQuestVar(q.Vars, 0, 11), "") {
				c.s.removeItemsByID(p, 182200004, 3)
				send(1694)
			}
		case 10002:
			if variable != 11 || c.s.countItems(p, 182200005) == 0 || !c.s.questRewardsFit(p, []data.QuestItem{{ID: 182200006, Count: 1}}) {
				return
			}
			if c.customQuestProgress(1004, setQuestVar(q.Vars, 0, 4), "") {
				c.s.removeItemsByID(p, 182200005, 1)
				c.s.addItem(p, 182200006, 1)
				c.send(dialogWindow(o.id, 10, 0))
			}
		}
	}
}
