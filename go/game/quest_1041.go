package game

import (
	"time"

	"aionlightning/game/data"
)

const (
	dangerousArtifactQuestID    int32 = 1041
	dangerousArtifactStartNPC   int32 = 203901
	dangerousArtifactEngineer   int32 = 204015
	dangerousArtifactXenophon   int32 = 203833
	dangerousArtifactYuditio    int32 = 278500
	dangerousArtifactLaigas     int32 = 204042
	dangerousArtifactBeacon     int32 = 700267
	dangerousArtifactRelic      int32 = 700181
	dangerousArtifactWorld      int32 = 210020000
	dangerousArtifactProof      int32 = 182201014
	dangerousArtifactReportItem int32 = 182201011
)

func (c *conn) dangerousArtifactLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(dangerousArtifactQuestID)
	template := c.s.data.Quests[dangerousArtifactQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	for _, prerequisiteID := range template.FinishedQuestConditions {
		prerequisite := c.player.quest(prerequisiteID)
		if prerequisite == nil || prerequisite.Status != "COMPLETE" {
			return false
		}
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking A Dangerous Artifact", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) dangerousArtifactDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != dangerousArtifactQuestID {
		return false
	}
	quest := c.player.quest(dangerousArtifactQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != dangerousArtifactLaigas {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, dangerousArtifactQuestID))
			return true
		case dialogID >= 8 && dialogID <= 11:
			c.finishQuestReward(script, o.id, uint16(dialogID), 0)
			return true
		case dialogID == 17:
			c.finishQuestReward(script, o.id, uint16(dialogID), 0)
			return true
		}
		return false
	}
	if quest.Status != "START" {
		return false
	}
	advance := func(value int32) bool {
		if !c.customQuestProgress(dangerousArtifactQuestID, setQuestVar(quest.Vars, 0, value), "") {
			return false
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	}
	switch o.npc.ID {
	case dangerousArtifactStartNPC:
		switch dialogID {
		case 25:
			switch variable {
			case 0:
				c.send(dialogWindow(o.id, 1011, dangerousArtifactQuestID))
				return true
			case 3:
				c.send(dialogWindow(o.id, 1693, dangerousArtifactQuestID))
				return true
			case 6:
				c.send(dialogWindow(o.id, 2716, dangerousArtifactQuestID))
				return true
			}
		case 10000:
			if variable == 0 {
				return advance(1)
			}
		case 10002:
			if variable == 3 {
				return advance(4)
			}
		case 10005:
			if variable == 6 {
				return advance(7)
			}
		}
	case dangerousArtifactEngineer:
		switch dialogID {
		case 25:
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, dangerousArtifactQuestID))
				return true
			}
		case 10001:
			if variable == 1 && c.customQuestProgress(dangerousArtifactQuestID, setQuestVar(quest.Vars, 0, 2), "") {
				c.spawnDangerousArtifactBeacons(c.player.instance)
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case dangerousArtifactBeacon:
		if variable == 2 && dialogID == -1 {
			return c.dangerousArtifactUseObject(o, 2, 3, 0)
		}
	case dangerousArtifactXenophon:
		switch dialogID {
		case 25:
			if variable == 4 {
				c.send(dialogWindow(o.id, 2034, dangerousArtifactQuestID))
				return true
			}
		case 10003:
			if variable == 4 {
				return advance(5)
			}
		}
	case dangerousArtifactYuditio:
		switch dialogID {
		case 25:
			if variable == 5 {
				c.send(dialogWindow(o.id, 2375, dangerousArtifactQuestID))
				return true
			}
		case 10004:
			if variable == 5 {
				return advance(6)
			}
		}
	case dangerousArtifactLaigas:
		switch dialogID {
		case 25:
			switch variable {
			case 7:
				c.send(dialogWindow(o.id, 3057, dangerousArtifactQuestID))
				return true
			case 9:
				c.send(dialogWindow(o.id, 3398, dangerousArtifactQuestID))
				return true
			}
		case 10006:
			if variable == 7 && c.s.questRewardsFit(c.player, []data.QuestItem{{ID: dangerousArtifactReportItem, Count: 1}}) {
				if !c.s.addItem(c.player, dangerousArtifactReportItem, 1) || !c.customQuestProgress(dangerousArtifactQuestID, setQuestVar(quest.Vars, 0, 8), "") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				c.send(ascensionMovie(37))
				return true
			}
		case 10007:
			if variable == 9 && c.customQuestProgress(dangerousArtifactQuestID, quest.Vars, "REWARD") {
				c.send(dialogWindow(o.id, 10, 0))
				c.send(ascensionMovie(38))
				return true
			}
		}
	case dangerousArtifactRelic:
		if variable == 8 && dialogID == -1 {
			return c.dangerousArtifactUseObject(o, 8, 9, dangerousArtifactProof)
		}
	}
	return false
}

func (c *conn) dangerousArtifactUseObject(o *object, variable, nextVariable, removeItem int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.useTask != nil {
		return false
	}
	p := c.player
	quest := p.quest(dangerousArtifactQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != variable {
		return false
	}
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		current := p.quest(dangerousArtifactQuestID)
		if p.conn != c || current == nil || current.Status != "START" || questVar(current.Vars, 0) != variable {
			return
		}
		c.customQuestProgress(dangerousArtifactQuestID, setQuestVar(current.Vars, 0, nextVariable), "")
		if removeItem != 0 {
			c.s.removeItemsByID(p, removeItem, 1)
		}
		if p.targetID != o.id {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
	})
	return false
}

func (c *conn) spawnDangerousArtifactBeacons(instance int32) []*object {
	if c == nil || c.s == nil {
		return nil
	}
	template := c.s.data.Npcs[dangerousArtifactBeacon]
	if template == nil {
		return nil
	}
	positions := [][3]float32{{2265.621, 2357.8164, 277.8047}, {1827.1799, 2537.9143, 267.5}}
	spawned := make([]*object, 0, len(positions))
	for _, position := range positions {
		o := &object{id: c.s.ids.nextID(), worldID: dangerousArtifactWorld, instance: instance,
			x: position[0], y: position[1], z: position[2], npc: template,
			homeX: position[0], homeY: position[1], homeZ: position[2]}
		c.s.initNpc(o)
		c.s.byID[o.id] = o
		c.s.addObject(o)
		spawned = append(spawned, o)
	}
	return spawned
}
