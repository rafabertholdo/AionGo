package game

import (
	"time"

	"aionlightning/game/data"
)

const (
	disappearingAetherQuestID int32 = 1034
	disappearingAetherNPCID   int32 = 203903
	lakaiasNPCID              int32 = 204032
	disappearingAetherItemID  int32 = 182201002
	destroyedArtifactNPCID    int32 = 700149
)

func (c *conn) disappearingAetherLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(disappearingAetherQuestID)
	prerequisite := c.player.quest(1300)
	template := c.s.data.Quests[disappearingAetherQuestID]
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Disappearing Aether", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) disappearingAetherDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != disappearingAetherQuestID {
		return false
	}
	quest := c.player.quest(disappearingAetherQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != disappearingAetherNPCID {
			return false
		}
		switch dialogID {
		case -1:
			c.send(dialogWindow(o.id, 2375, disappearingAetherQuestID))
			return true
		case 1009:
			c.send(dialogWindow(o.id, 5, disappearingAetherQuestID))
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
	case disappearingAetherNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, disappearingAetherQuestID))
				return true
			}
		case 10000:
			if variable == 0 && c.customQuestProgress(disappearingAetherQuestID, setQuestVar(quest.Vars, 0, 1), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case lakaiasNPCID:
		switch dialogID {
		case 25:
			switch variable {
			case 1:
				c.send(dialogWindow(o.id, 1352, disappearingAetherQuestID))
				return true
			case 3:
				c.send(dialogWindow(o.id, 1693, disappearingAetherQuestID))
				return true
			case 4:
				c.send(dialogWindow(o.id, 2034, disappearingAetherQuestID))
				return true
			}
			return c.disappearingAetherCollectItems(o, script)
		case 33:
			return c.disappearingAetherCollectItems(o, script)
		case 1353:
			c.send(ascensionMovie(179))
			return false
		case 10001:
			if variable == 1 && c.customQuestProgress(disappearingAetherQuestID, setQuestVar(quest.Vars, 0, 2), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		case 10002:
			if variable == 3 && c.customQuestProgress(disappearingAetherQuestID, setQuestVar(quest.Vars, 0, 4), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case destroyedArtifactNPCID:
		if variable == 2 && dialogID == -1 {
			return c.disappearingAetherUseArtifact(o)
		}
	}
	return false
}

func (c *conn) disappearingAetherCollectItems(o *object, script *data.QuestScript) bool {
	quest := c.player.quest(disappearingAetherQuestID)
	template := c.s.data.Quests[disappearingAetherQuestID]
	if quest == nil || template == nil {
		return false
	}
	if !c.s.hasQuestItems(c.player, template) {
		c.send(dialogWindow(o.id, 2120, disappearingAetherQuestID))
		return true
	}
	for _, required := range template.CollectItems {
		c.s.removeItemsByID(c.player, required.ID, required.Count)
	}
	if !c.customQuestProgress(disappearingAetherQuestID, quest.Vars, "REWARD") {
		return false
	}
	c.send(dialogWindow(o.id, 2035, disappearingAetherQuestID))
	return true
}

func (c *conn) disappearingAetherUseArtifact(o *object) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.useTask != nil {
		return false
	}
	p := c.player
	quest := p.quest(disappearingAetherQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 2 {
		return false
	}
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	if !c.customQuestProgress(disappearingAetherQuestID, setQuestVar(quest.Vars, 0, 3), "") {
		return false
	}
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
	})
	return false
}
