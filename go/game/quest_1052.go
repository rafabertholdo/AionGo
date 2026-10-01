package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	rootOfRotQuestID    int32 = 1052
	rootOfRotStartNPC   int32 = 204549
	rootOfRotMiddleNPC  int32 = 730026
	rootOfRotEndNPC     int32 = 730024
	rootOfRotFirstItem  int32 = 182201603
	rootOfRotSecondItem int32 = 182201604
)

func (c *conn) rootOfRotLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(rootOfRotQuestID)
	prerequisite := c.player.quest(1500)
	template := c.s.data.Quests[rootOfRotQuestID]
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Root of the Rot", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) rootOfRotDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != rootOfRotQuestID {
		return false
	}
	quest := c.player.quest(rootOfRotQuestID)
	if quest == nil {
		return false
	}
	targetID := o.npc.ID
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if targetID != rootOfRotEndNPC {
			return false
		}
		// Java removes one of each collected quest item before delegating every
		// end dialog (including the click and reward selections).
		c.s.removeItemsByID(c.player, rootOfRotFirstItem, 1)
		c.s.removeItemsByID(c.player, rootOfRotSecondItem, 1)
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, rootOfRotQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			// This reward is fixed, but Java accepts every default end selection.
			c.finishQuestReward(script, o.id, 17, 0)
			return true
		default:
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	switch targetID {
	case rootOfRotStartNPC:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, rootOfRotQuestID))
				return true
			}
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, rootOfRotQuestID))
				return true
			}
			// Java falls through from the unmatched 25 branch into case 33.
			return c.rootOfRotCollect(o, variable)
		case 33:
			return c.rootOfRotCollect(o, variable)
		case 10000:
			if variable == 0 {
				return c.rootOfRotAdvance(o, quest, variable+1)
			}
			// The Java case has no break; an unmatched 10000 falls into 10001.
			fallthrough
		case 10001:
			if variable == 1 {
				return c.rootOfRotAdvance(o, quest, variable+1)
			}
		}
	case rootOfRotMiddleNPC:
		switch dialogID {
		case 25:
			if variable == 2 {
				c.send(dialogWindow(o.id, 1693, rootOfRotQuestID))
				return true
			}
			// Java falls through from 25 into case 10255.
			fallthrough
		case 10255:
			if variable == 2 {
				if !c.customQuestProgress(rootOfRotQuestID, quest.Vars, "REWARD") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	}
	return false
}

func (c *conn) rootOfRotCollect(o *object, variable int32) bool {
	if !c.s.hasQuestItems(c.player, c.s.data.Quests[rootOfRotQuestID]) {
		c.send(dialogWindow(o.id, 10001, rootOfRotQuestID))
		return true
	}
	if !c.customQuestProgress(rootOfRotQuestID, setQuestVar(c.player.quest(rootOfRotQuestID).Vars, 0, variable+1), "") {
		return false
	}
	c.send(dialogWindow(o.id, 10000, rootOfRotQuestID))
	return true
}

func (c *conn) rootOfRotAdvance(o *object, quest *store.Quest, variable int32) bool {
	if !c.customQuestProgress(rootOfRotQuestID, setQuestVar(quest.Vars, 0, variable), "") {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}
