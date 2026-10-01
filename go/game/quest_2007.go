package game

import (
	"time"

	"aionlightning/game/data"
)

// wheresRaeThisTimeLevelUp unlocks the finale after the six earlier quests.
func (c *conn) wheresRaeThisTimeLevelUp() bool {
	if c.player == nil {
		return false
	}
	q := c.player.quest(2007)
	if q == nil || q.Status != "LOCKED" {
		return false
	}
	for id := int32(2001); id <= 2006; id++ {
		previous := c.player.quest(id)
		if previous == nil || previous.Status != "COMPLETE" {
			return false
		}
	}
	next := *q
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Where's Rae This Time", "err", err)
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) wheresRaeThisTimeDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != 2007 {
		return false
	}
	q := c.player.quest(2007)
	if q == nil {
		return false
	}
	if q.Status == "REWARD" && o.npc.ID == 203516 {
		switch {
		case dialogID == -1:
			c.send(wheresRaeMovie(58))
			c.send(dialogWindow(o.id, 3057, 2007))
		case dialogID == 1009:
			c.send(dialogWindow(o.id, 5, 2007))
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
	stage := questVar(q.Vars, 0)
	if o.npc.ID == 203539 && dialogID == 1694 {
		c.send(wheresRaeMovie(55))
		return true
	}
	for index, npcID := range []int32{203516, 203519, 203539, 203552, 203554} {
		if o.npc.ID != npcID || stage != int32(index) {
			continue
		}
		if dialogID == 25 {
			c.send(dialogWindow(o.id, uint16(1011+341*index), 2007))
			return true
		}
		if dialogID == int32(10000+index) && c.customQuestProgress(2007, setQuestVar(q.Vars, 0, stage+1), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
		return false
	}
	if o.npc.ID == 203554 && stage == 8 {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 2716, 2007))
			return true
		}
		if dialogID == 10005 && c.customQuestProgress(2007, q.Vars, "REWARD") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	}
	for index, npcID := range []int32{700081, 700082, 700083} {
		if o.npc.ID == npcID && stage == int32(index+5) && dialogID == -1 {
			return c.wheresRaeDestroy(o, index)
		}
	}
	return false
}

func (c *conn) wheresRaeDestroy(o *object, index int) bool {
	if o.useTask != nil || o.dead {
		return false
	}
	p := c.player
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		q := p.quest(2007)
		if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead || p.targetID != o.id || q == nil || q.Status != "START" || questVar(q.Vars, 0) != int32(index+5) {
			return
		}
		if !c.customQuestProgress(2007, setQuestVar(q.Vars, 0, int32(index+6)), "") {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		if index == 2 {
			c.send(wheresRaeMovie(56))
		}
	})
	return true
}
