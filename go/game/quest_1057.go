package game

import (
	"time"

	"aionlightning/game/data"
)

const (
	creatingMonsterQuestID       int32 = 1057
	creatingMonsterStartNPCID    int32 = 204502
	creatingMonsterSecondNPCID   int32 = 204619
	creatingMonsterEndNPCID      int32 = 204500
	creatingMonsterTabletID      int32 = 700218
	creatingMonsterFinalObjectID int32 = 700279
	creatingMonsterKillNPCID     int32 = 700219
	creatingMonsterFinalKillID   int32 = 212211
	creatingMonsterArtifactID    int32 = 182201616
	creatingMonsterHeironWorldID int32 = 310050000
)

func (c *conn) creatingMonsterLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(creatingMonsterQuestID)
	prerequisite := c.player.quest(1056)
	template := c.s.data.Quests[creatingMonsterQuestID]
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Creating a Monster", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) creatingMonsterDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != creatingMonsterQuestID {
		return false
	}
	quest := c.player.quest(creatingMonsterQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != creatingMonsterEndNPCID {
			return false
		}
		switch dialogID {
		case -1:
			c.send(dialogWindow(o.id, 10002, creatingMonsterQuestID))
			return true
		case 1009:
			c.send(dialogWindow(o.id, 5, creatingMonsterQuestID))
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
	advance := func(nextVariable int32) bool {
		if !c.customQuestProgress(creatingMonsterQuestID, setQuestVar(quest.Vars, 0, nextVariable), "") {
			return false
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	}
	switch o.npc.ID {
	case creatingMonsterStartNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, creatingMonsterQuestID))
				return true
			}
			if variable == 3 {
				c.send(dialogWindow(o.id, 2034, creatingMonsterQuestID))
				return true
			}
		case 2036:
			if variable == 3 {
				c.send(playMovie(190))
			}
			return false
		case 10000:
			if variable == 0 {
				return advance(1)
			}
		case 10003:
			if variable == 3 {
				return advance(4)
			}
		}
	case creatingMonsterSecondNPCID:
		switch dialogID {
		case 25:
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, creatingMonsterQuestID))
				return true
			}
		case 10001:
			if variable == 1 {
				return advance(2)
			}
		}
	case creatingMonsterTabletID:
		if variable == 2 && dialogID == -1 {
			return c.creatingMonsterUseTablet(o)
		}
		if variable == 2 && dialogID == 10002 {
			c.send(dialogWindow(o.id, 0, 0))
			if !c.s.questRewardsFit(c.player, []data.QuestItem{{ID: creatingMonsterArtifactID, Count: 1}}) || !c.s.addItem(c.player, creatingMonsterArtifactID, 1) {
				c.send(systemMessage(msgInventoryFull))
				return true
			}
			return c.customQuestProgress(creatingMonsterQuestID, setQuestVar(quest.Vars, 0, 3), "")
		}
	case creatingMonsterFinalObjectID:
		if variable == 9 && dialogID == -1 {
			return c.creatingMonsterUseFinalObject(o)
		}
	}
	return false
}

func (c *conn) creatingMonsterEnterWorld() {
	if c == nil || c.player == nil || c.player.WorldID != creatingMonsterHeironWorldID {
		return
	}
	quest := c.player.quest(creatingMonsterQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 4 {
		return
	}
	next := *quest
	next.Vars = setQuestVar(next.Vars, 0, 5)
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("updating Creating a Monster on world entry", "err", err)
		return
	}
	*quest = next
	c.send(questAccepted(2, next))
}

func (c *conn) creatingMonsterKill(npcID int32) bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(creatingMonsterQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch {
	case npcID == creatingMonsterKillNPCID && variable < 8:
		variable++
	case npcID == creatingMonsterFinalKillID && variable == 8:
		variable++
	default:
		return false
	}
	return c.customQuestProgress(creatingMonsterQuestID, setQuestVar(quest.Vars, 0, variable), "")
}

func (c *conn) creatingMonsterUseTablet(o *object) bool {
	if o == nil || o.useTask != nil {
		return false
	}
	p := c.player
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		c.send(dialogWindow(o.id, 1693, creatingMonsterQuestID))
	})
	return false
}

func (c *conn) creatingMonsterUseFinalObject(o *object) bool {
	if o == nil || o.useTask != nil {
		return false
	}
	p := c.player
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		quest := p.quest(creatingMonsterQuestID)
		if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 9 {
			return
		}
		c.customQuestProgress(creatingMonsterQuestID, quest.Vars, "REWARD")
	})
	return false
}
