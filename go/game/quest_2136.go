package game

import (
	"time"

	"aionlightning/game/data"
)

const (
	lostAxeQuestID       int32  = 2136
	lostAxeWorkItemID    int32  = 182203130
	lostAxeActionNPCID   int32  = 700146
	lostAxeReportNPCID   int32  = 790009
	lostAxeMovieID       uint16 = 59
	lostAxeReportWorldID int32  = 220010000
)

// lostAxeAsmodianDialog ports _2136TheLostAxe.onDialogEvent for its report NPC.
func (c *conn) lostAxeAsmodianDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != lostAxeQuestID {
		return false
	}
	quest := c.player.quest(lostAxeQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != lostAxeReportNPCID {
			return false
		}
		if o.useTask == nil {
			o.useTask = c.s.later(10*time.Second, func() {
				o.useTask = nil
				c.s.despawnNpc(o, true)
			})
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, lostAxeQuestID))
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuest(script, o.id, uint16(dialogID))
		default:
			return false
		}
		return true
	}
	if quest.Status != "START" || o.npc.ID != lostAxeReportNPCID || questVar(quest.Vars, 0) != 1 {
		return false
	}
	switch dialogID {
	case 25:
		c.send(dialogWindow(o.id, 1011, lostAxeQuestID))
		return true
	case 10000, 10001:
		if !c.customQuestProgress(lostAxeQuestID, quest.Vars, "REWARD") {
			return false
		}
		if count := c.s.countItems(c.player, lostAxeWorkItemID); count > 0 {
			c.s.removeItemsByID(c.player, lostAxeWorkItemID, count)
		}
		c.send(dialogWindow(o.id, 0, 0))
		page := uint16(6)
		if dialogID == 10001 {
			page = 5
		}
		c.send(dialogWindow(o.id, page, lostAxeQuestID))
		return true
	default:
		return false
	}
}

// lostAxeAction ports the ActionItem on-talk event on the Ishalgen grave.
func (c *conn) lostAxeAction(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != lostAxeQuestID || o.npc.ID != lostAxeActionNPCID || dialogID != -1 {
		return false
	}
	quest := c.player.quest(lostAxeQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	if questVar(quest.Vars, 0) != 0 || o.useTask != nil {
		c.dialogSilent()
		return true
	}
	player := c.player
	instanceID := player.instance
	c.send(useObject(player.ID, o.id, 1))
	player.broadcast(c.s.playerEmotionTo(player, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		if player.conn != c || !player.spawned || player.seen[o.id] != o || o.dead {
			return
		}
		current := player.quest(lostAxeQuestID)
		if current == nil || current.Status != "START" || questVar(current.Vars, 0) != 0 {
			return
		}
		c.send(useObject(player.ID, o.id, 0))
		player.broadcast(c.s.playerEmotionTo(player, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		c.send(playMovie(lostAxeMovieID))
		if !c.customQuestProgress(lostAxeQuestID, setQuestVar(current.Vars, 0, 1), "") {
			return
		}
		if template := c.s.data.Npcs[lostAxeReportNPCID]; template != nil {
			spawn := &object{id: c.s.ids.nextID(), worldID: lostAxeReportWorldID, instance: instanceID,
				x: 1088.5, y: 2371.8, z: 258.375, heading: 87, homeX: 1088.5, homeY: 2371.8, homeZ: 258.375, npc: template}
			c.s.initNpc(spawn)
			c.s.byID[spawn.id] = spawn
			c.s.addObject(spawn)
		}
	})
	c.dialogSilent()
	return true
}
