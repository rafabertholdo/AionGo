package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	speakingBalaurQuestID      int32 = 1071
	speakingBalaurStartNPCID   int32 = 278532
	speakingBalaurMerchantID   int32 = 798026
	speakingBalaurTranslatorID int32 = 798025
	speakingBalaurReportNPCID  int32 = 279019
	speakingBalaurPhraseItemID int32 = 182202001
	speakingBalaurCipherItemID int32 = 182202002
)

func (c *conn) speakingBalaurLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(speakingBalaurQuestID)
	prerequisite := c.player.quest(1701)
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || quest.Status != "LOCKED" {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Speaking Balaur", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) speakingBalaurDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != speakingBalaurQuestID {
		return false
	}
	quest := c.player.quest(speakingBalaurQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != speakingBalaurStartNPCID {
			return false
		}
		switch {
		case dialogID == -1:
			c.send(dialogWindow(o.id, 10002, speakingBalaurQuestID))
			return true
		case dialogID == 1009:
			c.send(dialogWindow(o.id, 5, speakingBalaurQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			// Java's defaultQuestEndDialog finishes this fixed-reward quest for any reward selection.
			c.finishQuestReward(script, o.id, 17, 0)
			return true
		default:
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	switch o.npc.ID {
	case speakingBalaurStartNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, speakingBalaurQuestID))
				return true
			}
			fallthrough
		case 10000:
			if variable == 0 {
				return c.speakingBalaurAdvance(o, quest, 1)
			}
		}
	case speakingBalaurMerchantID:
		switch dialogID {
		case 25:
			switch variable {
			case 1:
				c.send(dialogWindow(o.id, 1352, speakingBalaurQuestID))
				return true
			case 4:
				c.send(dialogWindow(o.id, 2375, speakingBalaurQuestID))
				return true
			case 6, 8:
				c.send(dialogWindow(o.id, 3057, speakingBalaurQuestID))
				return true
			}
			fallthrough
		case 10004:
			if variable == 4 {
				if !c.speakingBalaurSetVariable(quest, 5, "") {
					return false
				}
				c.s.removeItemsByID(c.player, speakingBalaurCipherItemID, 1)
				c.s.addItem(c.player, speakingBalaurPhraseItemID, 1)
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
			return false
		case 10006:
			if variable == 6 || variable == 8 {
				if !c.customQuestProgress(speakingBalaurQuestID, quest.Vars, "REWARD") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
			return false
		case 10010:
			if variable == 1 && c.s.decreaseKinah(c.player, 20000) {
				c.s.addItem(c.player, speakingBalaurPhraseItemID, 1)
				if !c.customQuestProgress(speakingBalaurQuestID, setQuestVar(quest.Vars, 0, 7), "") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
			c.send(dialogWindow(o.id, 1355, speakingBalaurQuestID))
			return true
		case 10011:
			if variable == 1 {
				return c.speakingBalaurAdvance(o, quest, 2)
			}
		}
	case speakingBalaurTranslatorID:
		switch dialogID {
		case 25:
			if variable == 2 {
				c.send(dialogWindow(o.id, 1693, speakingBalaurQuestID))
				return true
			}
			fallthrough
		case 10002:
			if variable == 2 {
				return c.speakingBalaurAdvance(o, quest, 3)
			}
		}
	case speakingBalaurReportNPCID:
		switch dialogID {
		case 25:
			if variable == 3 {
				c.send(dialogWindow(o.id, 2034, speakingBalaurQuestID))
				return true
			}
		case 10003:
			if variable == 3 {
				if !c.speakingBalaurSetVariable(quest, 4, "") {
					return false
				}
				c.s.addItem(c.player, speakingBalaurCipherItemID, 1)
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	}
	return false
}

func (c *conn) speakingBalaurAdvance(o *object, quest *store.Quest, variable int32) bool {
	if !c.speakingBalaurSetVariable(quest, variable, "") {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}

func (c *conn) speakingBalaurSetVariable(quest *store.Quest, variable int32, status string) bool {
	return c.customQuestProgress(speakingBalaurQuestID, setQuestVar(quest.Vars, 0, variable), status)
}

func (c *conn) speakingBalaurItemUse(item *store.Item) bool {
	if c == nil || c.player == nil || item == nil || item.ItemID != speakingBalaurPhraseItemID {
		return false
	}
	quest := c.player.quest(speakingBalaurQuestID)
	if quest == nil {
		return false
	}
	p := c.player
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 1, 1, 0), true)
	c.s.removeItemsByID(p, speakingBalaurPhraseItemID, 1)
	next := *quest
	next.Vars = setQuestVar(next.Vars, 0, questVar(next.Vars, 0)+1)
	if err := c.s.quests.SaveQuest(p.ID, next); err != nil {
		c.s.log.Error("updating Speaking Balaur after item use", "err", err)
		return true
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}
