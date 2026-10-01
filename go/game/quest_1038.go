package game

import (
	"time"

	"aionlightning/game/data"
)

const (
	shadowsCommandQuestID        int32 = 1038
	shadowsCommandNPCID          int32 = 203933
	shadowsCommandEndNPCID       int32 = 203991
	shadowsCommandFirstObjectID  int32 = 700162
	shadowsCommandSecondObjectID int32 = 700172
	shadowsCommandBossID         int32 = 204005
	shadowsCommandItemOne        int32 = 182201015
	shadowsCommandItemTwo        int32 = 182201016
	shadowsCommandItemThree      int32 = 182201017
	shadowsCommandOfferingID     int32 = 182201007
	shadowsCommandWorldID        int32 = 210020000
)

func (c *conn) shadowsCommandLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(shadowsCommandQuestID)
	template := c.s.data.Quests[shadowsCommandQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking The Shadow's Command", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) shadowsCommandDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != shadowsCommandQuestID {
		return false
	}
	quest := c.player.quest(shadowsCommandQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != shadowsCommandEndNPCID {
			return false
		}
		switch dialogID {
		case -1, 1009:
			c.send(dialogWindow(o.id, 5, shadowsCommandQuestID))
			return true
		default:
			if dialogID >= 8 && dialogID <= 11 || dialogID == 17 {
				c.finishQuestReward(script, o.id, uint16(dialogID), 0)
				return true
			}
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	switch o.npc.ID {
	case shadowsCommandFirstObjectID:
		if variable == 0 && dialogID == -1 {
			return c.shadowsCommandUseObject(o, 0, 1, 34, 0)
		}
	case shadowsCommandSecondObjectID:
		if variable == 2 && dialogID == -1 {
			return c.shadowsCommandUseObject(o, 2, 3, 0, shadowsCommandOfferingID)
		}
	case shadowsCommandNPCID:
		switch dialogID {
		case 25:
			switch variable {
			case 1:
				c.send(dialogWindow(o.id, 1352, shadowsCommandQuestID))
				return true
			case 3:
				c.send(dialogWindow(o.id, 1694, shadowsCommandQuestID))
				return true
			case 4:
				c.send(dialogWindow(o.id, 2034, shadowsCommandQuestID))
				return true
			default:
				return c.shadowsCommandCollectItems(o)
			}
		case 33:
			return c.shadowsCommandCollectItems(o)
		case 10001:
			if variable == 1 && c.customQuestProgress(shadowsCommandQuestID, setQuestVar(quest.Vars, 0, 2), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		case 10002:
			if variable == 3 && c.customQuestProgress(shadowsCommandQuestID, setQuestVar(quest.Vars, 0, 4), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		case 10003:
			if variable == 4 && c.customQuestProgress(shadowsCommandQuestID, setQuestVar(quest.Vars, 0, 6), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case shadowsCommandEndNPCID:
		if dialogID == 25 && variable == 6 {
			c.send(dialogWindow(o.id, 2375, shadowsCommandQuestID))
			return true
		}
		if dialogID == 10004 && variable == 6 {
			c.send(ascensionMovie(35))
			if c.customQuestProgress(shadowsCommandQuestID, setQuestVar(quest.Vars, 0, 7), "") {
				c.send(dialogWindow(o.id, 10, 0))
				c.spawnShadowsCommandBoss()
				return true
			}
		}
	}
	return false
}

func (c *conn) shadowsCommandCollectItems(o *object) bool {
	template := c.s.data.Quests[shadowsCommandQuestID]
	if !c.s.hasQuestItems(c.player, template) {
		c.send(dialogWindow(o.id, 2120, shadowsCommandQuestID))
		return true
	}
	for _, required := range template.CollectItems {
		c.s.removeItemsByID(c.player, required.ID, required.Count)
	}
	c.send(dialogWindow(o.id, 2035, shadowsCommandQuestID))
	return true
}

func (c *conn) shadowsCommandUseObject(o *object, variable, nextVariable int32, movieID uint16, giveItem int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.useTask != nil {
		return false
	}
	p := c.player
	quest := p.quest(shadowsCommandQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != variable {
		return false
	}
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		if movieID != 0 {
			c.send(ascensionMovie(movieID))
		}
		current := p.quest(shadowsCommandQuestID)
		if current == nil || current.Status != "START" || questVar(current.Vars, 0) != variable {
			return
		}
		if giveItem != 0 {
			if !c.s.questRewardsFit(p, []data.QuestItem{{ID: giveItem, Count: 1}}) || !c.s.addItem(p, giveItem, 1) {
				c.send(systemMessage(msgInventoryFull))
				return
			}
		}
		c.customQuestProgress(shadowsCommandQuestID, setQuestVar(current.Vars, 0, nextVariable), "")
	})
	return false
}

func (c *conn) shadowsCommandKill(npcID int32) bool {
	if c == nil || c.player == nil || npcID != shadowsCommandBossID {
		return false
	}
	quest := c.player.quest(shadowsCommandQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 7 {
		return false
	}
	return c.customQuestProgress(shadowsCommandQuestID, quest.Vars, "REWARD")
}

func (c *conn) spawnShadowsCommandBoss() *object {
	template := c.s.data.Npcs[shadowsCommandBossID]
	if template == nil {
		return nil
	}
	x, y, z := float32(1768.16), float32(924.47), float32(422.02)
	o := &object{id: c.s.ids.nextID(), worldID: shadowsCommandWorldID, x: x, y: y, z: z, homeX: x, homeY: y, homeZ: z, npc: template}
	c.s.initNpc(o)
	c.s.byID[o.id] = o
	c.s.addObject(o)
	if c.player != nil && c.player.WorldID == shadowsCommandWorldID {
		c.s.addDamage(o, c.player, 1000)
	}
	return o
}
