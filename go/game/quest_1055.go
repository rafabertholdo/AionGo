package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	eternalRestQuestID int32 = 1055

	eternalRestStartNPCID  int32 = 204629
	eternalRestMiddleNPCID int32 = 204625
	eternalRestFirstNPCID  int32 = 204628
	eternalRestSecondNPCID int32 = 204627
	eternalRestThirdNPCID  int32 = 204626
	eternalRestFourthNPCID int32 = 204622
	eternalRestObjectID    int32 = 700270

	eternalRestFirstItemID    int32 = 182201609
	eternalRestSecondItemID   int32 = 182201610
	eternalRestThirdItemID    int32 = 182201611
	eternalRestFourthItemID   int32 = 182201612
	eternalRestOfferingItemID int32 = 182201613
)

func (c *conn) eternalRestLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(eternalRestQuestID)
	prerequisite := c.player.quest(1500)
	template := c.s.data.Quests[eternalRestQuestID]
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Eternal Rest", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) eternalRestDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != eternalRestQuestID {
		return false
	}
	quest := c.player.quest(eternalRestQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != eternalRestStartNPCID {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, eternalRestQuestID))
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
	switch o.npc.ID {
	case eternalRestStartNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, eternalRestQuestID))
				return true
			}
			if variable == 2 {
				c.send(dialogWindow(o.id, 1693, eternalRestQuestID))
				return true
			}
			// Java's case 25 falls through to 10000, then 10001.
			fallthrough
		case 10000:
			if variable == 0 {
				return c.eternalRestAdvance(o, quest, 1)
			}
			// Java's case 10000 falls through to 10001 when var is not zero.
			fallthrough
		case 10001:
			if variable == 1 {
				return c.eternalRestAdvance(o, quest, 2)
			}
		}
	case eternalRestMiddleNPCID:
		switch dialogID {
		case 25:
			switch variable {
			case 1:
				c.send(dialogWindow(o.id, 1352, eternalRestQuestID))
				return true
			case 2:
				c.send(dialogWindow(o.id, 1693, eternalRestQuestID))
				return true
			case 4:
				c.send(dialogWindow(o.id, 2375, eternalRestQuestID))
				return true
			}
			// Java's unmatched case 25 falls through to the collection branch.
			return c.eternalRestCollect(o, quest, variable)
		case 33:
			return c.eternalRestCollect(o, quest, variable)
		case 10001:
			if variable == 1 {
				return c.eternalRestAdvance(o, quest, 2)
			}
			// Java's case 10001 falls through to 10255 at var four.
			fallthrough
		case 10255:
			if variable == 4 {
				if !c.customQuestProgress(eternalRestQuestID, quest.Vars, "REWARD") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case eternalRestFirstNPCID:
		return c.eternalRestOfferItem(o, dialogID, variable, eternalRestFirstItemID, 1694)
	case eternalRestSecondNPCID:
		return c.eternalRestOfferItem(o, dialogID, variable, eternalRestSecondItemID, 1781)
	case eternalRestThirdNPCID:
		return c.eternalRestOfferItem(o, dialogID, variable, eternalRestThirdItemID, 1864)
	case eternalRestFourthNPCID:
		return c.eternalRestOfferItem(o, dialogID, variable, eternalRestFourthItemID, 1949)
	case eternalRestObjectID:
		if variable == 3 && dialogID == -1 {
			return c.eternalRestUseOffering(o, quest)
		}
	}
	return false
}

func (c *conn) eternalRestAdvance(o *object, quest *store.Quest, variable int32) bool {
	if !c.customQuestProgress(eternalRestQuestID, setQuestVar(quest.Vars, 0, variable), "") {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}

func (c *conn) eternalRestOfferItem(o *object, dialogID int32, variable, itemID int32, page uint16) bool {
	if variable == 2 && dialogID == 25 {
		c.send(dialogWindow(o.id, page, eternalRestQuestID))
		return true
	}
	if variable != 2 || dialogID != 10002 {
		return false
	}
	if c.s.countItems(c.player, itemID) == 0 {
		if c.s.questRewardsFit(c.player, []data.QuestItem{{ID: itemID, Count: 1}}) {
			c.s.addItem(c.player, itemID, 1)
		} else {
			c.send(systemMessage(msgInventoryFull))
		}
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}

func (c *conn) eternalRestCollect(o *object, quest *store.Quest, variable int32) bool {
	template := c.s.data.Quests[eternalRestQuestID]
	if quest == nil || template == nil || !c.s.hasQuestItems(c.player, template) {
		c.send(dialogWindow(o.id, 10001, eternalRestQuestID))
		return true
	}
	for _, item := range template.CollectItems {
		c.s.removeItemsByID(c.player, item.ID, item.Count)
	}
	if !c.customQuestProgress(eternalRestQuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
		return false
	}
	if c.s.questRewardsFit(c.player, []data.QuestItem{{ID: eternalRestOfferingItemID, Count: 1}}) {
		c.s.addItem(c.player, eternalRestOfferingItemID, 1)
	} else {
		c.send(systemMessage(msgInventoryFull))
	}
	c.send(dialogWindow(o.id, 10000, eternalRestQuestID))
	return true
}

func (c *conn) eternalRestUseOffering(o *object, quest *store.Quest) bool {
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
		c.s.removeItemsByID(p, eternalRestOfferingItemID, 1)
		c.customQuestProgress(eternalRestQuestID, setQuestVar(quest.Vars, 0, 4), "")
	})
	return false
}
