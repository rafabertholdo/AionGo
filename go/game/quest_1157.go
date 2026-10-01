package game

import (
	"math"

	"aionlightning/game/data"
	"aionlightning/wire"
)

const gaphyrksLoveQuestID int32 = 1157

func gaphyrksLoveMovie(id uint16) *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(0)
	w.D(0)
	w.D(0)
	w.H(id)
	w.D(0)
	return w
}

// gaphyrksLoveAttack handles the player's first attack on the marked scout.
// The quest remains START until the client reports that movie 17 ended.
func (c *conn) gaphyrksLoveAttack(o *object) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.npc.ID != 210319 || o.dead ||
		(o.respawn != nil && !o.respawn.cancelled) {
		return false
	}
	quest := c.player.quest(gaphyrksLoveQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	dx, dy, dz := float64(o.x-892), float64(o.y-2024), float64(o.z-166)
	if math.Sqrt(dx*dx+dy*dy+dz*dz) > 13 {
		return false
	}
	c.s.despawnNpc(o, true)
	c.s.scheduleRespawn(o)
	c.send(gaphyrksLoveMovie(17))
	return true
}

// gaphyrksLoveMovieEnd is Java's movie 17 completion callback.
func (c *conn) gaphyrksLoveMovieEnd(movieID uint16) bool {
	if c == nil || c.player == nil || movieID != 17 {
		return false
	}
	quest := c.player.quest(gaphyrksLoveQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	return c.customQuestProgress(gaphyrksLoveQuestID, quest.Vars, "REWARD")
}

// gaphyrksLoveDialog handles Gaphyrk's offer and reward dialog.
func (c *conn) gaphyrksLoveDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != 798003 ||
		script == nil || script.ID != gaphyrksLoveQuestID {
		return false
	}
	quest := c.player.quest(gaphyrksLoveQuestID)
	if quest == nil || quest.Status == "NONE" {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1011, gaphyrksLoveQuestID))
			return true
		}
		return c.customQuestStart(o, script, uint16(dialogID), 0)
	}
	if quest.Status != "REWARD" {
		return false
	}
	switch {
	case dialogID == -1:
		c.send(dialogWindow(o.id, 2375, gaphyrksLoveQuestID))
		return true
	case dialogID == 1009:
		c.send(dialogWindow(o.id, 5, gaphyrksLoveQuestID))
		return true
	case dialogID >= 8 && dialogID <= 17:
		// Java's default end handler accepts the full dialog range for this fixed reward.
		c.finishQuest(script, o.id, 17)
		return true
	default:
		return false
	}
}

// gaphyrksLoveShowDialog handles Gaphyrk's default reward preview.
func (c *conn) gaphyrksLoveShowDialog(o *object, script *data.QuestScript) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != 798003 ||
		script == nil || script.ID != gaphyrksLoveQuestID {
		return false
	}
	quest := c.player.quest(gaphyrksLoveQuestID)
	if quest == nil || quest.Status != "REWARD" {
		return false
	}
	c.send(dialogWindow(o.id, 2375, gaphyrksLoveQuestID))
	return true
}
