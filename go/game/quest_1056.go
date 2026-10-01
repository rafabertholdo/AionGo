package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	lepharistPoisonResearchQuestID int32 = 1056
	lepharistPoisonStartNPCID      int32 = 204504
	lepharistPoisonSecondNPCID     int32 = 204574
	lepharistPoisonReportNPCID     int32 = 203705
	lepharistPoisonEndNPCID        int32 = 203707
	lepharistPoisonTargetNPCID     int32 = 212151
	lepharistPoisonItemID          int32 = 182201614
)

func (c *conn) lepharistPoisonResearchLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(lepharistPoisonResearchQuestID)
	template := c.s.data.Quests[lepharistPoisonResearchQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	for _, prerequisiteID := range []int32{1500, 1016, 1039} {
		prerequisite := c.player.quest(prerequisiteID)
		if prerequisite == nil || prerequisite.Status != "COMPLETE" {
			return false
		}
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Lepharist Poison Research", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) lepharistPoisonResearchDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != lepharistPoisonResearchQuestID {
		return false
	}
	quest := c.player.quest(lepharistPoisonResearchQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != lepharistPoisonEndNPCID {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, lepharistPoisonResearchQuestID))
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
	case lepharistPoisonStartNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, lepharistPoisonResearchQuestID))
				return true
			}
			// Java falls through to 10000 when the opening page is not selected.
			fallthrough
		case 10000:
			if variable == 0 {
				return c.lepharistPoisonResearchAdvance(o, quest, 1)
			}
		}
	case lepharistPoisonSecondNPCID:
		switch dialogID {
		case 25:
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, lepharistPoisonResearchQuestID))
				return true
			}
			fallthrough
		case 10001:
			if variable == 1 {
				return c.lepharistPoisonResearchAdvance(o, quest, 2)
			}
		}
	case lepharistPoisonReportNPCID:
		switch dialogID {
		case 25:
			if variable == 2 {
				c.send(dialogWindow(o.id, 1693, lepharistPoisonResearchQuestID))
				return true
			}
			if variable == 5 {
				c.send(dialogWindow(o.id, 2716, lepharistPoisonResearchQuestID))
				return true
			}
			// Java falls through to the item hand-in branch for other variables.
			fallthrough
		case 33:
			if variable == 2 && c.s.countItems(c.player, lepharistPoisonItemID) == 1 {
				c.send(playMovie(101))
				c.send(dialogWindow(o.id, 10000, lepharistPoisonResearchQuestID))
				return true
			}
			c.send(dialogWindow(o.id, 10001, lepharistPoisonResearchQuestID))
			return true
		case 10003:
			if variable == 2 {
				if !c.customQuestProgress(lepharistPoisonResearchQuestID, setQuestVar(quest.Vars, 0, 4), "") {
					return false
				}
				c.s.removeItemsByID(c.player, lepharistPoisonItemID, 1)
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
			// Java falls through into 10255 when var is five.
			fallthrough
		case 10255:
			if variable == 5 {
				if !c.customQuestProgress(lepharistPoisonResearchQuestID, quest.Vars, "REWARD") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	}
	return false
}

func (c *conn) lepharistPoisonResearchAdvance(o *object, quest *store.Quest, variable int32) bool {
	if !c.customQuestProgress(lepharistPoisonResearchQuestID, setQuestVar(quest.Vars, 0, variable), "") {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}

func (c *conn) lepharistPoisonResearchKill(npcID int32) bool {
	if c == nil || c.player == nil || npcID != lepharistPoisonTargetNPCID {
		return false
	}
	quest := c.player.quest(lepharistPoisonResearchQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 4 {
		return false
	}
	return c.customQuestProgress(lepharistPoisonResearchQuestID, setQuestVar(quest.Vars, 0, 5), "")
}
