package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	newWingsQuestID       int32 = 1075
	newWingsFirstNPCID    int32 = 278506
	newWingsEndNPCID      int32 = 279023
	newWingsInstanceNPCID int32 = 278643
	newWingsBalaurNPCID   int32 = 214102
	newWingsInstanceID    int32 = 400010000
	newWingsInstanceIndex int32 = 1
	newWingsFlightPathID  int32 = 57001
)

func (c *conn) newWingsLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(newWingsQuestID)
	prerequisite := c.player.quest(1072)
	template := c.s.data.Quests[newWingsQuestID]
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking New Wings", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) newWingsDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != newWingsQuestID {
		return false
	}
	quest := c.player.quest(newWingsQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != newWingsEndNPCID {
			return false
		}
		switch {
		case dialogID == -1:
			c.send(dialogWindow(o.id, 10002, newWingsQuestID))
			return true
		case dialogID == 1009:
			c.send(dialogWindow(o.id, 5, newWingsQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			// The Java default end handler accepts every selection for this fixed-reward quest.
			c.finishQuestReward(script, o.id, 17, 0)
			return true
		default:
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	switch o.npc.ID {
	case newWingsFirstNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, newWingsQuestID))
				return true
			}
			// An unmatched Java case 25 falls into the movie case.
			fallthrough
		case 1013:
			c.send(movie(0, 272))
			return false
		case 10000:
			if variable == 0 {
				return c.newWingsAdvance(o, quest, 1, 10)
			}
		}
	case newWingsEndNPCID:
		switch dialogID {
		case 25:
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, newWingsQuestID))
				return true
			}
			fallthrough
		case 10001:
			if variable == 1 {
				if !c.newWingsSetVariable(quest, 2, "") {
					return false
				}
				c.send(dialogWindow(o.id, 0, 0))
				c.send(c.s.playerEmotionTo(c.player, emoteStartFlyTele, newWingsFlightPathID, 0, 0, 0, 0, 0))
				return true
			}
		}
	case newWingsInstanceNPCID:
		switch dialogID {
		case 25:
			if variable == 2 {
				c.send(dialogWindow(o.id, 1693, newWingsQuestID))
				return true
			}
			if variable == 3 {
				c.send(dialogWindow(o.id, 2034, newWingsQuestID))
				return true
			}
			fallthrough
		case 10002:
			if variable == 2 {
				if !c.newWingsSetVariable(quest, 3, "") {
					return false
				}
				c.newWingsSpawnBalaur(2344.32, 1789.96, 2258.88, 86)
				c.newWingsSpawnBalaur(2344.51, 1786.01, 2258.88, 52)
				c.send(dialogWindow(o.id, 0, 0))
				return true
			}
			// Java case 10002 falls through to 10003 at variable three.
			fallthrough
		case 10003:
			if variable == 3 {
				if !c.newWingsSetVariable(quest, 12, "REWARD") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	}
	return false
}

func (c *conn) newWingsAdvance(o *object, questVars *store.Quest, variable int32, windowID uint16) bool {
	if !c.newWingsSetVariable(questVars, variable, "") {
		return false
	}
	c.send(dialogWindow(o.id, windowID, 0))
	return true
}

func (c *conn) newWingsSetVariable(quest *store.Quest, variable int32, status string) bool {
	return c.customQuestProgress(newWingsQuestID, setQuestVar(quest.Vars, 0, variable), status)
}

func (c *conn) newWingsSpawnBalaur(x, y, z float32, heading byte) *object {
	template := c.s.data.Npcs[newWingsBalaurNPCID]
	if template == nil {
		return nil
	}
	o := &object{
		id: c.s.ids.nextID(), worldID: newWingsInstanceID, instance: newWingsInstanceIndex,
		x: x, y: y, z: z, heading: heading, homeX: x, homeY: y, homeZ: z,
		npc: template, noRespawn: true,
	}
	c.s.initNpc(o)
	c.s.byID[o.id] = o
	c.s.addObject(o)
	return o
}
