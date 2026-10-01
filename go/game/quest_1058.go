package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	aetherInsanityQuestID      int32 = 1058
	aetherInsanityStartNPCID   int32 = 204020
	aetherInsanityEndNPCID     int32 = 204501
	aetherInsanityFirstItemID  int32 = 182201617
	aetherInsanitySecondItemID int32 = 182201618
)

func (c *conn) aetherInsanityLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(aetherInsanityQuestID)
	prerequisite := c.player.quest(1500)
	template := c.s.data.Quests[aetherInsanityQuestID]
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Aether Insanity", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) aetherInsanityDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != aetherInsanityQuestID {
		return false
	}
	quest := c.player.quest(aetherInsanityQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != aetherInsanityEndNPCID {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, aetherInsanityQuestID))
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
	variable := questVar(quest.Vars, 0)
	switch o.npc.ID {
	case aetherInsanityStartNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, aetherInsanityQuestID))
				return true
			}
			fallthrough
		case 10000:
			if variable == 0 {
				return c.aetherInsanityAdvance(o, quest, 1)
			}
		}
	case aetherInsanityEndNPCID:
		switch dialogID {
		case 25:
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, aetherInsanityQuestID))
				return true
			}
			if variable == 2 {
				c.send(dialogWindow(o.id, 1693, aetherInsanityQuestID))
				return true
			}
			// Java falls through from case 25 to the collection check at case 33.
			fallthrough
		case 33:
			return c.aetherInsanityCollect(o, quest)
		case 1353:
			c.send(playMovie(191))
			return false
		case 10001:
			if variable == 1 {
				return c.aetherInsanityAdvance(o, quest, 2)
			}
		}
	}
	return false
}

func (c *conn) aetherInsanityAdvance(o *object, quest *store.Quest, variable int32) bool {
	if !c.customQuestProgress(aetherInsanityQuestID, setQuestVar(quest.Vars, 0, variable), "") {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}

func (c *conn) aetherInsanityCollect(o *object, quest *store.Quest) bool {
	template := c.s.data.Quests[aetherInsanityQuestID]
	if template == nil || !c.s.hasQuestItems(c.player, template) {
		c.send(dialogWindow(o.id, 10001, aetherInsanityQuestID))
		return true
	}
	for _, item := range template.CollectItems {
		c.s.removeItemsByID(c.player, item.ID, item.Count)
	}
	next := *quest
	next.Status = "REWARD"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("completing Aether Insanity collection", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	c.send(dialogWindow(o.id, 5, aetherInsanityQuestID))
	return true
}
