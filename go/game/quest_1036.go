package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	kaidanPrisonerQuestID int32 = 1036
	kaidanPrisonerStartID int32 = 203904
	kaidanPrisonerNPCID   int32 = 204045
	kaidanOfficerNPCID    int32 = 204003
	kaidanQuartermasterID int32 = 204004
	kaidanHandlerNPCID    int32 = 204020
	kaidanRewardNPCID     int32 = 203901
	kaidanProofItemID     int32 = 182201003
	kaidanKeyItemID       int32 = 182201004
	kaidanReportItemID    int32 = 182201005
)

func (c *conn) kaidanPrisonerLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(kaidanPrisonerQuestID)
	template := c.s.data.Quests[kaidanPrisonerQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Kaidan Prisoner", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) kaidanPrisonerDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != kaidanPrisonerQuestID {
		return false
	}
	quest := c.player.quest(kaidanPrisonerQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != kaidanRewardNPCID {
			return false
		}
		switch dialogID {
		case -1, 1009:
			c.send(dialogWindow(o.id, 5, kaidanPrisonerQuestID))
			return true
		default:
			if dialogID >= 8 && dialogID <= 9 || dialogID == 17 {
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
	case kaidanPrisonerStartID:
		if dialogID == 25 && variable == 0 {
			c.send(dialogWindow(o.id, 1011, kaidanPrisonerQuestID))
			return true
		}
		if dialogID == 10000 && variable == 0 && c.customQuestProgress(kaidanPrisonerQuestID, setQuestVar(quest.Vars, 0, 1), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case kaidanPrisonerNPCID:
		switch dialogID {
		case 25:
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, kaidanPrisonerQuestID))
				return true
			}
		case 1354:
			if variable == 1 {
				c.send(ascensionMovie(32))
				return false
			}
		case 10001:
			if variable == 1 && c.customQuestProgress(kaidanPrisonerQuestID, setQuestVar(quest.Vars, 0, 2), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case kaidanOfficerNPCID:
		switch dialogID {
		case 25:
			switch variable {
			case 2:
				c.send(dialogWindow(o.id, 1693, kaidanPrisonerQuestID))
				return true
			case 3:
				if c.s.hasQuestItems(c.player, c.s.data.Quests[kaidanPrisonerQuestID]) {
					for _, required := range c.s.data.Quests[kaidanPrisonerQuestID].CollectItems {
						c.s.removeItemsByID(c.player, required.ID, required.Count)
					}
					c.send(dialogWindow(o.id, 2034, kaidanPrisonerQuestID))
				} else {
					c.send(dialogWindow(o.id, 2120, kaidanPrisonerQuestID))
				}
				return true
			default:
				c.send(dialogWindow(o.id, 2120, kaidanPrisonerQuestID))
				return true
			}
		case 10002:
			if variable == 2 && c.customQuestProgress(kaidanPrisonerQuestID, setQuestVar(quest.Vars, 0, 3), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		case 10003:
			if variable == 3 {
				c.send(ascensionMovie(50))
				if c.customQuestProgress(kaidanPrisonerQuestID, setQuestVar(quest.Vars, 0, 4), "") {
					c.send(dialogWindow(o.id, 10, 0))
					return true
				}
			}
		}
	case kaidanQuartermasterID:
		if dialogID == 25 && variable == 4 {
			c.send(dialogWindow(o.id, 2375, kaidanPrisonerQuestID))
			return true
		}
		if dialogID == 10004 && variable == 4 {
			return c.giveKaidanItem(o, quest, kaidanKeyItemID, 5, "")
		}
	case kaidanHandlerNPCID:
		if dialogID == 25 && variable == 5 {
			c.send(dialogWindow(o.id, 2716, kaidanPrisonerQuestID))
			return true
		}
		if dialogID == 2717 {
			c.s.removeItemsByID(c.player, kaidanKeyItemID, 1)
		}
		if (dialogID == 2717 || dialogID == 10004) && variable == 5 {
			return c.giveKaidanItem(o, quest, kaidanReportItemID, 6, "REWARD")
		}
	case kaidanRewardNPCID:
		if dialogID == 25 && variable == 6 {
			c.send(dialogWindow(o.id, 3057, kaidanPrisonerQuestID))
			return true
		}
		if dialogID == 1009 && variable == 6 {
			c.s.removeItemsByID(c.player, kaidanReportItemID, 1)
			if c.customQuestProgress(kaidanPrisonerQuestID, setQuestVar(quest.Vars, 0, 7), "REWARD") {
				c.send(dialogWindow(o.id, 5, kaidanPrisonerQuestID))
				return true
			}
		}
	}
	return false
}

func (c *conn) giveKaidanItem(o *object, quest *store.Quest, itemID int32, nextVariable int32, status string) bool {
	if !c.s.questRewardsFit(c.player, []data.QuestItem{{ID: itemID, Count: 1}}) {
		c.send(systemMessage(msgInventoryFull))
		return false
	}
	if !c.customQuestProgress(kaidanPrisonerQuestID, setQuestVar(quest.Vars, 0, nextVariable), status) {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	c.s.addItem(c.player, itemID, 1)
	return true
}
