package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	spiritOfNatureQuestID      int32 = 1183
	spiritOfNatureStartNPCID   int32 = 730012
	spiritOfNatureFirstNPCID   int32 = 730013
	spiritOfNatureSecondNPCID  int32 = 730014
	spiritOfNatureFirstItemID  int32 = 182200550
	spiritOfNatureSecondItemID int32 = 182200565
)

func (c *conn) spiritOfNatureDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != spiritOfNatureQuestID {
		return false
	}
	quest := c.player.quest(spiritOfNatureQuestID)
	if quest == nil || quest.Status == "NONE" {
		if o.npc.ID != spiritOfNatureStartNPCID {
			return false
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 1011, spiritOfNatureQuestID))
			return true
		case 1007:
			c.send(dialogWindow(o.id, 4, spiritOfNatureQuestID))
			return true
		case 1003:
			c.send(dialogWindow(o.id, 1004, spiritOfNatureQuestID))
			return true
		case 1002:
			if !c.s.canStartQuest(c.player, script) || c.player.level < c.s.data.Quests[spiritOfNatureQuestID].MinLevel {
				return false
			}
			c.startQuest(script, o.id)
			quest = c.player.quest(spiritOfNatureQuestID)
			return quest != nil && quest.Status == "START"
		default:
			return false
		}
	}
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != spiritOfNatureStartNPCID {
			return false
		}
		if dialogID == 1009 {
			c.send(dialogWindow(o.id, 5, spiritOfNatureQuestID))
			return true
		}
		if dialogID >= 8 && dialogID <= 17 {
			c.finishQuestReward(script, o.id, uint16(dialogID), 0)
			return true
		}
		if dialogID == -1 {
			c.send(dialogWindow(o.id, 5, spiritOfNatureQuestID))
			return true
		}
		return false
	}
	if quest.Status != "START" {
		return false
	}
	switch o.npc.ID {
	case spiritOfNatureFirstNPCID:
		if dialogID == -1 {
			c.send(dialogWindow(o.id, 1352, spiritOfNatureQuestID))
			return true
		}
		if dialogID == 10000 {
			return c.spiritOfNatureGiveItem(o, quest, spiritOfNatureFirstItemID)
		}
		// Java's first-NPC case falls through into the second-NPC case.
		fallthrough
	case spiritOfNatureSecondNPCID:
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1693, spiritOfNatureQuestID))
			return true
		}
		if dialogID == 10001 {
			return c.spiritOfNatureGiveItem(o, quest, spiritOfNatureSecondItemID)
		}
		// The second-NPC switch has a defaultQuestEndDialog, which only
		// handles reward state. START therefore falls through as unhandled.
	case spiritOfNatureStartNPCID:
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 2375, spiritOfNatureQuestID))
			return true
		}
		if dialogID == 1009 {
			next := *quest
			next.Vars = setQuestVar(next.Vars, 0, 3)
			next.Status = "REWARD"
			c.s.removeItemsByID(c.player, spiritOfNatureFirstItemID, c.s.countItems(c.player, spiritOfNatureFirstItemID))
			c.s.removeItemsByID(c.player, spiritOfNatureSecondItemID, c.s.countItems(c.player, spiritOfNatureSecondItemID))
			if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
				c.s.log.Error("completing Spirit of Nature", "err", err)
				return false
			}
			*quest = next
			c.send(questAccepted(2, next))
			c.send(dialogWindow(o.id, 5, spiritOfNatureQuestID))
			return true
		}
	}
	return false
}

func (c *conn) spiritOfNatureGiveItem(o *object, quest *store.Quest, itemID int32) bool {
	if c.s.countItems(c.player, itemID) == 0 {
		if !c.s.questRewardsFit(c.player, []data.QuestItem{{ID: itemID, Count: 1}}) || !c.s.addItem(c.player, itemID, 1) {
			return true
		}
	}
	if !c.customQuestProgress(spiritOfNatureQuestID, setQuestVar(quest.Vars, 0, questVar(quest.Vars, 0)+1), "") {
		// Java has already granted (or found) the item before updating the
		// quest state, and returns true without adding another dialog.
		return true
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}
