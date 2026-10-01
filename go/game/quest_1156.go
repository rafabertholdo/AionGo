package game

import (
	"time"

	"aionlightning/game/data"
)

const stolenVillageSealQuestID int32 = 1156

// stolenVillageSealDialog handles the quest offer, seal interaction, and
// return to the Verteron recruiter.
func (c *conn) stolenVillageSealDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != stolenVillageSealQuestID {
		return false
	}
	p := c.player
	quest := p.quest(stolenVillageSealQuestID)
	if quest == nil || quest.Status == "NONE" {
		if o.npc.ID != 203128 {
			return false
		}
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1011, stolenVillageSealQuestID))
			return true
		}
		return c.customQuestStart(o, script, uint16(dialogID), 0)
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != 798003 {
			return false
		}
		switch {
		case dialogID == -1:
			c.send(dialogWindow(o.id, 2375, stolenVillageSealQuestID))
			return true
		case dialogID == 1009:
			c.send(dialogWindow(o.id, 5, stolenVillageSealQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			// Java's default reward handler accepts the whole range for fixed rewards.
			c.finishQuest(script, o.id, 17)
			return true
		default:
			return false
		}
	}
	if quest.Status != "START" || questVar(quest.Vars, 0) != 0 || o.npc.ID != 700003 {
		return false
	}
	switch dialogID {
	case 25:
		c.send(dialogWindow(o.id, 1352, stolenVillageSealQuestID))
		return true
	case 10000:
		if !c.customQuestProgress(stolenVillageSealQuestID, setQuestVar(quest.Vars, 0, 1), "REWARD") {
			return false
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	case 1353:
		return c.stolenVillageSealUseObject(o)
	default:
		return false
	}
}

// stolenVillageSealUseObject plays the three-second search animation. The
// Java quest leaves progress unchanged and reopens the object's quest page.
func (c *conn) stolenVillageSealUseObject(o *object) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != 700003 || o.useTask != nil || c.player.targetID != o.id {
		return false
	}
	p := c.player
	quest := p.quest(stolenVillageSealQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 0 {
		return false
	}
	c.send(dialogWindow(0, 0, 0))
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		current := p.quest(stolenVillageSealQuestID)
		if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead || p.targetID != o.id || current == nil || current.Status != "START" || questVar(current.Vars, 0) != 0 {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		c.send(dialogWindow(o.id, 1353, stolenVillageSealQuestID))
	})
	return true
}

// stolenVillageSealShowDialog is the field seal's Java dialog-id -1 path.
func (c *conn) stolenVillageSealShowDialog(o *object, script *data.QuestScript) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != stolenVillageSealQuestID {
		return false
	}
	return c.stolenVillageSealDialog(o, script, -1)
}

// stolenVillageSealRewardShowDialog displays the NPC's reward preview.
func (c *conn) stolenVillageSealRewardShowDialog(o *object, script *data.QuestScript) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != 798003 || script == nil || script.ID != stolenVillageSealQuestID {
		return false
	}
	quest := c.player.quest(stolenVillageSealQuestID)
	if quest == nil || quest.Status != "REWARD" {
		return false
	}
	c.send(dialogWindow(o.id, 2375, stolenVillageSealQuestID))
	return true
}
