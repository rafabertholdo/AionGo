package game

import (
	"math/rand"

	"aionlightning/game/data"
)

const (
	klawThreatQuestID       int32 = 1053
	klawThreatStartNPCID    int32 = 204583
	klawThreatEndNPCID      int32 = 204502
	klawThreatLarvaNPCID    int32 = 700169
	klawThreatQueenNPCID    int32 = 212120
	klawThreatWorldID       int32 = 210040000
	klawThreatCollectItemID int32 = 182201605
)

func (c *conn) klawThreatLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(klawThreatQuestID)
	prerequisite := c.player.quest(1500)
	template := c.s.data.Quests[klawThreatQuestID]
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking The Klaw Threat", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) klawThreatDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != klawThreatQuestID {
		return false
	}
	quest := c.player.quest(klawThreatQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != klawThreatEndNPCID {
			return false
		}
		switch dialogID {
		case -1, 1009:
			c.send(dialogWindow(o.id, 5, klawThreatQuestID))
			return true
		default:
			if dialogID >= 8 && dialogID <= 17 {
				c.finishQuestReward(script, o.id, uint16(dialogID), 0)
				return true
			}
		}
		return false
	}
	if quest.Status != "START" || o.npc.ID != klawThreatStartNPCID {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch dialogID {
	case 25:
		switch variable {
		case 0:
			c.send(dialogWindow(o.id, 1011, klawThreatQuestID))
			return true
		case 1:
			c.send(dialogWindow(o.id, 1352, klawThreatQuestID))
			return true
		case 2:
			c.send(dialogWindow(o.id, 1693, klawThreatQuestID))
			return true
		}
		// Java falls through from case 25 into case 33 after the three pages above.
		return c.klawThreatCollect(o)
	case 33:
		return c.klawThreatCollect(o)
	case 1693:
		c.send(ascensionMovie(186))
		return false
	case 10000:
		if variable == 0 && c.customQuestProgress(klawThreatQuestID, setQuestVar(quest.Vars, 0, 1), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
		// Java falls through into case 10002 when var is not zero.
		fallthrough
	case 10002:
		if variable == 1 && c.customQuestProgress(klawThreatQuestID, setQuestVar(quest.Vars, 0, 3), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	}
	return false
}

func (c *conn) klawThreatCollect(o *object) bool {
	quest := c.player.quest(klawThreatQuestID)
	if quest == nil || questVar(quest.Vars, 0) != 1 || c.s.countItems(c.player, klawThreatCollectItemID) < 3 {
		c.send(dialogWindow(o.id, 10001, klawThreatQuestID))
		return true
	}
	c.s.removeItemsByID(c.player, klawThreatCollectItemID, 3)
	c.send(dialogWindow(o.id, 10000, klawThreatQuestID))
	return true
}

func (c *conn) klawThreatKill(dead *object) bool {
	if c == nil || c.player == nil || dead == nil || dead.npc == nil {
		return false
	}
	quest := c.player.quest(klawThreatQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	switch dead.npc.ID {
	case klawThreatLarvaNPCID:
		if rand.Intn(5) != 1 {
			return false
		}
		c.spawnKlawQueen(dead)
		return true
	case klawThreatQueenNPCID:
		if questVar(quest.Vars, 0) != 3 {
			return false
		}
		next := *quest
		next.Status = "REWARD"
		if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
			c.s.log.Error("completing The Klaw Threat objective", "err", err)
			return false
		}
		*quest = next
		c.send(questAccepted(2, next))
		return false
	default:
		return false
	}
}

func (c *conn) spawnKlawQueen(dead *object) *object {
	template := c.s.data.Npcs[klawThreatQueenNPCID]
	if template == nil {
		return nil
	}
	spawned := &object{
		id: c.s.ids.nextID(), worldID: klawThreatWorldID, instance: 1,
		x: dead.x, y: dead.y, z: dead.z, heading: 0,
		homeX: dead.x, homeY: dead.y, homeZ: dead.z,
		npc: template, noRespawn: true,
	}
	c.s.initNpc(spawned)
	c.s.byID[spawned.id] = spawned
	c.s.addObject(spawned)
	c.s.addDamage(spawned, c.player, 1000)
	return spawned
}
