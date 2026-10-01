package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

const (
	sourcePollutionQuestID  int32 = 1016
	sourcePollutionWater    int32 = 182200017
	sourcePollutionGuide    int32 = 182200013
	sourcePollutionTear     int32 = 182200014
	sourcePollutionMedicine int32 = 182200015
	sourcePollutionLetter   int32 = 182200018
	sourcePollutionDiary    int32 = 182200016
)

// sourcePollutionLevelUp is Java's level-up event for the level 12 quest.
func (c *conn) sourcePollutionLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(sourcePollutionQuestID)
	template := c.s.data.Quests[sourcePollutionQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Source of the Pollution", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func sourcePollutionMovie() *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(0)
	w.D(0)
	w.D(0)
	w.H(28)
	w.D(0)
	return w
}

// sourcePollutionKill handles the pollution monster that triggers the final
// investigation stage and spawns the temporary researcher NPC.
func (c *conn) sourcePollutionKill(o *object) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.npc.ID != 210318 {
		return false
	}
	quest := c.player.quest(sourcePollutionQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 8 {
		return false
	}
	if !c.customQuestProgress(sourcePollutionQuestID, setQuestVar(quest.Vars, 0, 9), "") {
		return false
	}
	worldID, instanceID := c.player.WorldID, c.player.instance
	x, y, z, heading := o.x, o.y, o.z, o.heading
	c.s.later(5*time.Second, func() {
		if template := c.s.data.Npcs[203195]; template != nil {
			spawn := &object{id: c.s.ids.nextID(), worldID: worldID, instance: instanceID,
				x: x, y: y, z: z, heading: heading, homeX: x, homeY: y, homeZ: z, npc: template}
			c.s.initNpc(spawn)
			c.s.byID[spawn.id] = spawn
			c.s.addObject(spawn)
		}
	})
	return true
}

// sourcePollutionDialog ports the NPC conversations, borrowed-item cleanup,
// kill handoff, temporary NPC removal, and selectable reward turn-in.
func (c *conn) sourcePollutionDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != sourcePollutionQuestID {
		return false
	}
	p := c.player
	quest := p.quest(sourcePollutionQuestID)
	if quest == nil {
		return false
	}
	npcID := o.npc.ID
	if quest.Status == "REWARD" {
		if npcID != 203098 {
			return false
		}
		switch {
		case dialogID == -1:
			c.send(dialogWindow(o.id, 4080, sourcePollutionQuestID))
		case dialogID == 1009:
			c.send(dialogWindow(o.id, 5, sourcePollutionQuestID))
		case dialogID >= 8 && dialogID <= 11:
			c.finishQuestReward(script, o.id, uint16(dialogID), 0)
		default:
			return false
		}
		return true
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if npcID == 203149 && dialogID == 3400 {
		c.send(sourcePollutionMovie())
		return true
	}
	advance := func(next int32, consumeID, consumeCount, giveID, giveCount int32) bool {
		if giveID != 0 && !c.s.questRewardsFit(p, []data.QuestItem{{ID: giveID, Count: int64(giveCount)}}) {
			return false
		}
		if !c.customQuestProgress(sourcePollutionQuestID, setQuestVar(quest.Vars, 0, next), "") {
			return false
		}
		if consumeID != 0 {
			c.s.removeItemsByID(p, consumeID, int64(consumeCount))
		}
		if giveID != 0 {
			c.s.addItem(p, giveID, int64(giveCount))
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	}
	switch npcID {
	case 203149:
		switch dialogID {
		case 25:
			switch variable {
			case 0:
				c.send(dialogWindow(o.id, 1011, sourcePollutionQuestID))
			case 2:
				c.send(dialogWindow(o.id, 1693, sourcePollutionQuestID))
			case 7:
				c.send(dialogWindow(o.id, 3398, sourcePollutionQuestID))
			case 8:
				page := uint16(3569)
				if c.s.countItems(p, sourcePollutionMedicine) == 0 {
					page = 3484
				}
				c.send(dialogWindow(o.id, page, sourcePollutionQuestID))
			default:
				return false
			}
			return true
		case 10000, 10002:
			if variable == 0 || variable == 2 {
				return advance(variable+1, 0, 0, 0, 0)
			}
		case 10007:
			if variable == 7 && c.s.countItems(p, sourcePollutionGuide) > 0 && c.s.countItems(p, sourcePollutionTear) > 0 {
				if !advance(8, sourcePollutionGuide, 1, sourcePollutionMedicine, 2) {
					return false
				}
				c.s.removeItemsByID(p, sourcePollutionTear, 1)
				return true
			}
		case 10008:
			if variable == 8 && c.s.questRewardsFit(p, []data.QuestItem{{ID: sourcePollutionMedicine, Count: 2}}) {
				return c.s.addItem(p, sourcePollutionMedicine, 2)
			}
		}
	case 203148:
		if variable == 1 && dialogID == 25 {
			c.send(dialogWindow(o.id, 1352, sourcePollutionQuestID))
			return true
		}
		if variable == 1 && dialogID == 10001 {
			return advance(2, 0, 0, sourcePollutionWater, 1)
		}
	case 203832:
		if variable == 3 && dialogID == 25 {
			c.send(dialogWindow(o.id, 2034, sourcePollutionQuestID))
			return true
		}
		if variable == 3 && dialogID == 10003 {
			return advance(4, 0, 0, sourcePollutionGuide, 1)
		}
	case 203705:
		if variable == 4 && dialogID == 25 {
			c.send(dialogWindow(o.id, 2375, sourcePollutionQuestID))
			return true
		}
		if variable == 4 && dialogID == 10004 {
			return advance(5, 0, 0, 0, 0)
		}
	case 203822:
		if variable == 5 && dialogID == 25 {
			c.send(dialogWindow(o.id, 2716, sourcePollutionQuestID))
			return true
		}
		if variable == 5 && dialogID == 10005 && c.s.countItems(p, sourcePollutionWater) > 0 {
			return advance(6, sourcePollutionWater, 1, sourcePollutionLetter, 1)
		}
	case 203761:
		if variable == 6 && dialogID == 25 {
			c.send(dialogWindow(o.id, 3057, sourcePollutionQuestID))
			return true
		}
		if variable == 6 && dialogID == 10006 && c.s.countItems(p, sourcePollutionLetter) > 0 {
			return advance(7, sourcePollutionLetter, 1, sourcePollutionTear, 1)
		}
	case 203195:
		if variable == 9 && dialogID == 25 {
			c.send(dialogWindow(o.id, 3739, sourcePollutionQuestID))
			return true
		}
		if variable == 9 && dialogID == 10008 && c.s.countItems(p, sourcePollutionMedicine) > 0 &&
			c.s.questRewardsFit(p, []data.QuestItem{{ID: sourcePollutionDiary, Count: 1}}) {
			if !c.customQuestProgress(sourcePollutionQuestID, quest.Vars, "REWARD") {
				return false
			}
			c.s.removeItemsByID(p, sourcePollutionMedicine, 1)
			c.s.addItem(p, sourcePollutionDiary, 1)
			c.send(dialogWindow(o.id, 10, 0))
			c.s.later(40*time.Second, func() {
				if c.s.byID[o.id] == o {
					c.s.despawnNpc(o, true)
				}
			})
			return true
		}
	}
	return false
}

// sourcePollutionShowDialog handles the click-to-open reward preview.
func (c *conn) sourcePollutionShowDialog(o *object, script *data.QuestScript) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != sourcePollutionQuestID || o.npc.ID != 203098 {
		return false
	}
	quest := c.player.quest(sourcePollutionQuestID)
	if quest == nil || quest.Status != "REWARD" {
		return false
	}
	c.send(dialogWindow(o.id, 4080, sourcePollutionQuestID))
	return true
}
