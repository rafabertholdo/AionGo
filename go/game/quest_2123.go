package game

import (
	"time"

	"aionlightning/game/data"
)

const (
	imprisonedGourmetQuestID  int32 = 2123
	imprisonedGourmetStartNPC int32 = 203550
	imprisonedGourmetTalkNPC  int32 = 700128
)

// imprisonedGourmetDialog ports _2123TheImprisonedGourmet.onDialogEvent.
func (c *conn) imprisonedGourmetDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != imprisonedGourmetQuestID || c.player.level < 7 {
		return false
	}
	player := c.player
	quest := player.quest(imprisonedGourmetQuestID)
	variable := int32(0)
	if quest != nil {
		variable = questVar(quest.Vars, 0)
	}

	switch o.npc.ID {
	case imprisonedGourmetStartNPC:
		if quest == nil || quest.Status == "NONE" {
			switch dialogID {
			case 25:
				c.send(dialogWindow(o.id, 1011, imprisonedGourmetQuestID))
				return true
			case 1007:
				c.send(dialogWindow(o.id, 4, imprisonedGourmetQuestID))
				return true
			case 1002:
				c.startQuest(script, o.id)
				started := player.quest(imprisonedGourmetQuestID)
				return started != nil && started.Status == "START"
			case 1003:
				c.send(dialogWindow(o.id, 1004, imprisonedGourmetQuestID))
				return true
			default:
				return false
			}
		}
		if quest.Status == "START" {
			if dialogID == 25 && variable == 0 {
				c.send(dialogWindow(o.id, 1352, imprisonedGourmetQuestID))
				return true
			}
			if variable != 0 {
				return false
			}
			itemID := int32(0)
			questVariable := int32(0)
			rewardPage := uint16(0)
			switch dialogID {
			case 10000:
				itemID, questVariable, rewardPage = 182203121, 5, 5
			case 10001:
				itemID, questVariable, rewardPage = 182203122, 6, 6
			case 10002:
				itemID, questVariable, rewardPage = 182203123, 7, 7
			default:
				return false
			}
			if c.s.countItems(player, itemID) == 0 {
				c.send(dialogWindow(o.id, 1693, imprisonedGourmetQuestID))
				return true
			}
			// Java removes 182004687 in the first branch, although it checks for 182203121.
			removeID := itemID
			if dialogID == 10000 {
				removeID = 182004687
			}
			if count := c.s.countItems(player, removeID); count > 0 {
				c.s.removeItemsByID(player, removeID, count)
			}
			if dialogID == 10002 {
				if !c.customQuestProgress(imprisonedGourmetQuestID, quest.Vars, "REWARD") {
					return false
				}
				if !c.saveImprisonedGourmetVars(questVariable) {
					return false
				}
			} else {
				if !c.customQuestProgress(imprisonedGourmetQuestID, setQuestVar(quest.Vars, 0, questVariable), "") ||
					!c.customQuestProgress(imprisonedGourmetQuestID, setQuestVar(quest.Vars, 0, questVariable), "REWARD") {
					return false
				}
			}
			c.send(dialogWindow(o.id, rewardPage, imprisonedGourmetQuestID))
			return true
		}
		if quest.Status == "REWARD" {
			if dialogID == 25 {
				switch variable {
				case 5:
					c.send(dialogWindow(o.id, 5, imprisonedGourmetQuestID))
					return true
				case 6:
					c.send(dialogWindow(o.id, 6, imprisonedGourmetQuestID))
					return true
				case 7:
					c.send(dialogWindow(o.id, 7, imprisonedGourmetQuestID))
					return true
				}
			}
			return c.imprisonedGourmetDefaultEnd(o, script, dialogID)
		}
		return c.imprisonedGourmetDefaultEnd(o, script, dialogID)
	case imprisonedGourmetTalkNPC:
		if quest != nil && quest.Status == "START" && variable == 0 {
			player.broadcast(c.s.playerEmotionTo(player, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
			c.s.later(3*time.Second, func() {
				c.customQuestProgress(imprisonedGourmetQuestID, quest.Vars, "")
			})
			return true
		}
		return c.imprisonedGourmetDefaultEnd(o, script, dialogID)
	default:
		return c.imprisonedGourmetDefaultEnd(o, script, dialogID)
	}
}

func (c *conn) saveImprisonedGourmetVars(variable int32) bool {
	quest := c.player.quest(imprisonedGourmetQuestID)
	if quest == nil || quest.Status != "REWARD" {
		return false
	}
	next := *quest
	next.Vars = setQuestVar(next.Vars, 0, variable)
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("updating Imprisoned Gourmet", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) imprisonedGourmetDefaultEnd(o *object, script *data.QuestScript, dialogID int32) bool {
	quest := c.player.quest(imprisonedGourmetQuestID)
	if quest == nil || quest.Status != "REWARD" {
		return false
	}
	switch {
	case dialogID == -1 || dialogID == 1009:
		c.send(dialogWindow(o.id, 5, imprisonedGourmetQuestID))
	case dialogID >= 8 && dialogID <= 17:
		c.finishQuest(script, o.id, uint16(dialogID))
	default:
		return false
	}
	return true
}
