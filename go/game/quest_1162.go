package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	altenosWeddingRingQuestID     int32 = 1162
	altenosWeddingRingStartNPCID  int32 = 203095
	altenosWeddingRingReportNPCID int32 = 203093
	altenosWeddingRingObjectID    int32 = 700005
	altenosWeddingRingItemID      int32 = 182200563
)

func (c *conn) altenosWeddingRingDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != altenosWeddingRingQuestID {
		return false
	}
	quest := c.player.quest(altenosWeddingRingQuestID)
	if quest == nil || quest.Status == "NONE" {
		if o.npc.ID != altenosWeddingRingStartNPCID {
			return false
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 1011, altenosWeddingRingQuestID))
			return true
		case 1007:
			c.send(dialogWindow(o.id, 4, altenosWeddingRingQuestID))
			return true
		case 1003:
			c.send(dialogWindow(o.id, 1004, altenosWeddingRingQuestID))
			return true
		case 1002:
			template := c.s.data.Quests[altenosWeddingRingQuestID]
			if template == nil || !c.s.canStartQuest(c.player, script) || c.player.level < template.MinLevel {
				return false
			}
			c.startQuest(script, o.id)
			return c.player.quest(altenosWeddingRingQuestID) != nil && c.player.quest(altenosWeddingRingQuestID).Status == "START"
		default:
			return false
		}
	}
	if quest == nil {
		return false
	}

	if quest.Status == "REWARD" {
		if o.npc.ID != altenosWeddingRingStartNPCID {
			return false
		}
		if dialogID == -1 || dialogID == 1009 {
			c.send(dialogWindow(o.id, 5, altenosWeddingRingQuestID))
			return true
		}
		if dialogID >= 8 && dialogID <= 17 {
			c.finishQuestReward(script, o.id, 17, 0)
			return true
		}
		return false
	}
	if quest.Status != "START" {
		return false
	}

	switch o.npc.ID {
	case altenosWeddingRingObjectID:
		if dialogID == -1 {
			return c.altenosWeddingRingUse(o, quest)
		}
		// Java falls through from the object's switch to the two reporter cases
		// when a non-click dialog arrives.
		fallthrough
	case altenosWeddingRingReportNPCID, altenosWeddingRingStartNPCID:
		if questVar(quest.Vars, 0) != 1 {
			return false
		}
		if !c.customQuestProgress(altenosWeddingRingQuestID, quest.Vars, "REWARD") {
			return false
		}
		c.s.removeItemsByID(c.player, altenosWeddingRingItemID, c.s.countItems(c.player, altenosWeddingRingItemID))
		c.send(dialogWindow(o.id, 10, 0))
		return true
	default:
		return false
	}
}

func (c *conn) altenosWeddingRingUse(o *object, quest *store.Quest) bool {
	if c == nil || c.player == nil || o == nil || quest == nil || quest.Status != "START" {
		return false
	}
	p := c.player
	if c.s.countItems(p, altenosWeddingRingItemID) == 0 {
		if !c.s.questRewardsFit(p, []data.QuestItem{{ID: altenosWeddingRingItemID, Count: 1}}) || !c.s.addItem(p, altenosWeddingRingItemID, 1) {
			return true
		}
	}
	if o.useTask != nil {
		return true
	}
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		if p.targetID != o.id {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		if !c.customQuestProgress(altenosWeddingRingQuestID, setQuestVar(quest.Vars, 0, questVar(quest.Vars, 0)+1), "") {
			return
		}
		o.broadcast(c.s.emote(o, emoteDie, 0), true)
	})
	return true
}
