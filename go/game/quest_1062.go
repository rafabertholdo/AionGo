package game

import (
	"time"

	"aionlightning/game/data"
)

const (
	indratuLegionQuestID      int32 = 1062
	indratuLegionStartNPCID   int32 = 204500
	indratuLegionSecondNPCID  int32 = 204600
	indratuLegionThirdNPCID   int32 = 204610
	indratuLegionTargetNPCID  int32 = 700220
	indratuLegionBossNPCID    int32 = 212588
	indratuLegionDropItemID   int32 = 182201622
	indratuLegionFlightPathID int32 = 54001
)

func (c *conn) indratuLegionLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(indratuLegionQuestID)
	prerequisite := c.player.quest(1500)
	template := c.s.data.Quests[indratuLegionQuestID]
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Indratu Legion", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) indratuLegionDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != indratuLegionQuestID {
		return false
	}
	quest := c.player.quest(indratuLegionQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != indratuLegionStartNPCID {
			return false
		}
		switch dialogID {
		case -1:
			c.send(dialogWindow(o.id, 10002, indratuLegionQuestID))
			return true
		case 1009:
			c.send(dialogWindow(o.id, 5, indratuLegionQuestID))
			return true
		default:
			if dialogID >= 8 && dialogID <= 17 {
				c.finishQuestReward(script, o.id, uint16(dialogID), 0)
				return true
			}
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	advance := func(nextVariable int32, windowID uint16) bool {
		if !c.customQuestProgress(indratuLegionQuestID, setQuestVar(quest.Vars, 0, nextVariable), "") {
			return false
		}
		c.send(dialogWindow(o.id, windowID, 0))
		return true
	}
	switch o.npc.ID {
	case indratuLegionStartNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, indratuLegionQuestID))
				return true
			}
			fallthrough
		case 10000:
			if variable == 0 {
				return advance(1, 10)
			}
		}
	case indratuLegionSecondNPCID:
		switch dialogID {
		case 25:
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, indratuLegionQuestID))
				return true
			}
			fallthrough
		case 10001:
			if variable == 1 {
				if !c.customQuestProgress(indratuLegionQuestID, setQuestVar(quest.Vars, 0, 2), "") {
					return false
				}
				c.send(dialogWindow(o.id, 0, 0))
				c.send(c.s.playerEmotionTo(c.player, emoteStartFlyTele, indratuLegionFlightPathID, 0, 0, 0, 0, 0))
				return true
			}
		}
	case indratuLegionThirdNPCID:
		switch dialogID {
		case 25:
			if variable == 2 {
				c.send(dialogWindow(o.id, 1693, indratuLegionQuestID))
				return true
			}
		case 1694:
			c.send(playMovie(195))
			return false
		case 10002:
			if variable == 2 {
				return advance(3, 10)
			}
		}
	}
	return false
}

func (c *conn) indratuLegionKill(dead *object) bool {
	if c == nil || c.player == nil || dead == nil || dead.npc == nil {
		return false
	}
	quest := c.player.quest(indratuLegionQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if dead.npc.ID == indratuLegionTargetNPCID && variable > 2 && variable < 13 {
		nextVariable := variable + 1
		if !c.customQuestProgress(indratuLegionQuestID, setQuestVar(quest.Vars, 0, nextVariable), "") {
			return false
		}
		if variable == 12 {
			c.s.later(3*time.Second, func() {
				c.spawnIndratuLegionBoss(dead)
			})
		}
		return true
	}
	if dead.npc.ID == indratuLegionBossNPCID && variable == 13 {
		next := *quest
		next.Status = "REWARD"
		if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
			c.s.log.Error("completing Indratu Legion objective", "err", err)
			return false
		}
		*quest = next
		c.send(questAccepted(2, next))
		return true
	}
	return false
}

func (c *conn) spawnIndratuLegionBoss(dead *object) *object {
	if c == nil || c.player == nil || dead == nil {
		return nil
	}
	template := c.s.data.Npcs[indratuLegionBossNPCID]
	if template == nil {
		return nil
	}
	p := c.player
	spawned := &object{
		id: c.s.ids.nextID(), worldID: p.WorldID, instance: p.instance,
		x: dead.x, y: dead.y, z: dead.z, heading: dead.heading,
		homeX: dead.x, homeY: dead.y, homeZ: dead.z,
		npc: template, noRespawn: true,
	}
	c.s.initNpc(spawned)
	c.s.byID[spawned.id] = spawned
	c.s.addObject(spawned)
	return spawned
}
