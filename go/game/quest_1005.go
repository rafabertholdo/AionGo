package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// barringTheGateLevelUp unlocks the final Poeta campaign quest once all four
// preceding quests are complete.
func (c *conn) barringTheGateLevelUp() bool {
	if c.player == nil {
		return false
	}
	q := c.player.quest(1005)
	if q == nil || q.Status != "LOCKED" {
		return false
	}
	for _, id := range []int32{1001, 1002, 1003, 1004} {
		previous := c.player.quest(id)
		if previous == nil || previous.Status != "COMPLETE" {
			return false
		}
	}
	next := *q
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Barring the Gate", "err", err)
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

func barringTheGateMovie() *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(0)
	w.D(0)
	w.D(0)
	w.H(21)
	w.D(0)
	return w
}

// barringTheGateDialog ports Java's NPC conversations and four gate uses.
func (c *conn) barringTheGateDialog(o *object, script *data.QuestScript, dialogID int32) {
	if c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != 1005 {
		return
	}
	p := c.player
	q := p.quest(1005)
	if q == nil {
		return
	}
	if q.Status == "REWARD" {
		if o.npc.ID == 203067 {
			switch {
			case dialogID == -1:
				c.send(dialogWindow(o.id, 2716, 1005))
			case dialogID == 1009:
				c.send(dialogWindow(o.id, 5, 1005))
			case dialogID >= 8 && dialogID <= 17:
				c.finishQuest(script, o.id, uint16(dialogID))
			}
		}
		return
	}
	if q.Status != "START" {
		return
	}
	stage := questVar(q.Vars, 0)
	for index, npcID := range []int32{203067, 203081, 790001, 203085, 203086} {
		if o.npc.ID != npcID || stage != int32(index) {
			continue
		}
		if dialogID == 25 {
			c.send(dialogWindow(o.id, uint16(1011+341*index), 1005))
		} else if dialogID == int32(10000+index) && c.customQuestProgress(1005, setQuestVar(q.Vars, 0, stage+1), "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
		return
	}
	for index, npcID := range []int32{700081, 700082, 700083, 700080} {
		if o.npc.ID != npcID || stage != int32(index+5) || dialogID != -1 || o.useTask != nil {
			continue
		}
		c.send(useObject(p.ID, o.id, 1))
		c.dialogNotHandled() // Java's destroy() case returns false, so the main menu is sent as well
		p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
		o.useTask = c.s.later(3*time.Second, func() {
			o.useTask = nil
			current := p.quest(1005)
			if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead || p.targetID != o.id || current == nil || current.Status != "START" || questVar(current.Vars, 0) != int32(index+5) {
				return
			}
			status := ""
			next := int32(index + 6)
			if index == 3 {
				status = "REWARD"
				next = int32(index + 5)
			}
			c.send(useObject(p.ID, o.id, 0))
			p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
			o.broadcast(emotionPacket(o.id, emoteEmote, o.state, float32(o.stats.current(data.Speed))/speedScale, 128, 0, 0, 0, 0, 0,
				uint16(o.stats.base(data.AttackSpeed)), uint16(o.stats.current(data.AttackSpeed))), false)
			if index == 3 {
				c.send(barringTheGateMovie())
			}
			c.customQuestProgress(1005, setQuestVar(current.Vars, 0, next), status)
		})
		return
	}
}
