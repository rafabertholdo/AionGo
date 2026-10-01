package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	josnackDilemmaQuestID       int32 = 1092
	josnackDilemmaAtroposNPCID  int32 = 798155
	josnackDilemmaJosnackNPCID  int32 = 798206
	josnackDilemmaGuideNPCID    int32 = 700388
	josnackDilemmaScoutNPCID    int32 = 700389
	josnackDilemmaHeraldNPCID   int32 = 700390
	josnackDilemmaSpawnNPCID    int32 = 214552
	josnackDilemmaMapID         int32 = 210070000
	josnackDilemmaInstanceID    int32 = 6
	josnackDilemmaCollectItemID int32 = 182208012
)

// josnackDilemmaLevelUp mirrors the Java LOCKED-quest level-up event. The
// follow-up is offered only after A Request from Atropos has been completed.
func (c *conn) josnackDilemmaLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(josnackDilemmaQuestID)
	previousQuest := c.player.quest(1091)
	template := c.s.data.Quests[josnackDilemmaQuestID]
	if quest == nil || quest.Status != "LOCKED" || previousQuest == nil || previousQuest.Status != "COMPLETE" || template == nil || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Josnack's Dilemma", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) josnackDilemmaDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != josnackDilemmaQuestID {
		return false
	}
	quest := c.player.quest(josnackDilemmaQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != josnackDilemmaAtroposNPCID {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, josnackDilemmaQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuestReward(script, o.id, uint16(dialogID), 0)
			return true
		default:
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	switch o.npc.ID {
	case josnackDilemmaAtroposNPCID:
		switch dialogID {
		case 25:
			switch variable {
			case 0:
				c.send(dialogWindow(o.id, 1011, josnackDilemmaQuestID))
			case 3:
				c.send(dialogWindow(o.id, 2034, josnackDilemmaQuestID))
			case 4:
				c.send(dialogWindow(o.id, 2375, josnackDilemmaQuestID))
			default:
				c.dialogSilent()
			}
			return true
		case 10000:
			if variable == 0 {
				return c.josnackDilemmaAdvance(o, quest, 1)
			}
			// Java falls through to 10003 and then to the item hand-in case.
			fallthrough
		case 10003:
			if variable == 3 {
				return c.josnackDilemmaAdvance(o, quest, 4)
			}
			// Java falls through into dialog 33 when the variable did not match.
			fallthrough
		case 33:
			if variable != 4 {
				return false
			}
			if c.s.countItems(c.player, josnackDilemmaCollectItemID) < 6 {
				c.send(dialogWindow(o.id, 10008, josnackDilemmaQuestID))
				return true
			}
			c.s.removeItemsByID(c.player, josnackDilemmaCollectItemID, 6)
			next := *quest
			next.Status = "REWARD"
			if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
				c.s.log.Error("completing Josnack's Dilemma collection", "err", err)
				return false
			}
			*quest = next
			c.send(questAccepted(2, next))
			c.send(dialogWindow(o.id, 10001, josnackDilemmaQuestID))
			return true
		}
	case josnackDilemmaJosnackNPCID:
		switch dialogID {
		case 25:
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, josnackDilemmaQuestID))
				return true
			}
			// Java falls through to 10001, which does not match dialog 25.
		case 10001:
			if variable == 1 {
				return c.josnackDilemmaAdvance(o, quest, 2)
			}
		}
	case josnackDilemmaGuideNPCID:
		if dialogID == -1 && variable == 2 {
			if c.customQuestProgress(josnackDilemmaQuestID, quest.Vars, "") {
				return true
			}
		}
	case josnackDilemmaScoutNPCID:
		if dialogID == -1 && variable == 2 {
			return c.josnackDilemmaAdvance(o, quest, 3)
		}
	case josnackDilemmaHeraldNPCID:
		if dialogID == -1 && variable == 4 {
			if c.spawnJosnackDilemmaNPC(o) {
				c.s.despawnNpc(o, true)
				c.s.scheduleRespawn(o)
				return true
			}
		}
	}
	return false
}

func (c *conn) josnackDilemmaAdvance(o *object, quest *store.Quest, variable int32) bool {
	if !c.customQuestProgress(josnackDilemmaQuestID, setQuestVar(quest.Vars, 0, variable), "") {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}

func (c *conn) spawnJosnackDilemmaNPC(source *object) bool {
	template := c.s.data.Npcs[josnackDilemmaSpawnNPCID]
	if template == nil {
		return false
	}
	spawn := &object{
		id:        c.s.ids.nextID(),
		worldID:   josnackDilemmaMapID,
		instance:  josnackDilemmaInstanceID,
		x:         source.x,
		y:         source.y,
		z:         source.z,
		heading:   source.heading,
		homeX:     source.x,
		homeY:     source.y,
		homeZ:     source.z,
		npc:       template,
		noRespawn: true,
	}
	c.s.initNpc(spawn)
	c.s.byID[spawn.id] = spawn
	c.s.addObject(spawn)
	return true
}
