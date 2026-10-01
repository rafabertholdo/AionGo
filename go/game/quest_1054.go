package game

import "aionlightning/game/data"

const (
	powerOfElimQuestID       int32 = 1054
	powerOfElimStartNPCID    int32 = 730024
	powerOfElimEndNPCID      int32 = 204647
	powerOfElimArtifactNPCID int32 = 730008
	powerOfElimReportNPCID   int32 = 730019
	powerOfElimArtifactID    int32 = 182201606
	powerOfElimReportItemID  int32 = 182201607
	powerOfElimCollectItemID int32 = 182201608
)

var powerOfElimPrerequisiteIDs = [...]int32{1500, 1002, 1032, 1052}

func (c *conn) powerOfElimLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(powerOfElimQuestID)
	template := c.s.data.Quests[powerOfElimQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	for _, prerequisiteID := range powerOfElimPrerequisiteIDs {
		prerequisite := c.player.quest(prerequisiteID)
		if prerequisite == nil || prerequisite.Status != "COMPLETE" {
			return false
		}
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking The Power of Elim", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) powerOfElimDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != powerOfElimQuestID {
		return false
	}
	quest := c.player.quest(powerOfElimQuestID)
	if quest == nil {
		return false
	}
	targetID := o.npc.ID
	if quest.Status == "REWARD" {
		if targetID != powerOfElimEndNPCID {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, powerOfElimQuestID))
			return true
		case dialogID >= 8 && dialogID <= 11 || dialogID == 17:
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
	advance := func(nextVariable int32) bool {
		if !c.customQuestProgress(powerOfElimQuestID, setQuestVar(quest.Vars, 0, nextVariable), "") {
			return false
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	}
	switch targetID {
	case powerOfElimStartNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, powerOfElimQuestID))
				return true
			}
			// Java falls through from dialog 25 into case 10000 when the var is nonzero.
			fallthrough
		case 10000:
			if variable == 0 {
				return advance(1)
			}
		}
	case powerOfElimEndNPCID:
		switch dialogID {
		case 25:
			switch variable {
			case 1:
				c.send(dialogWindow(o.id, 1352, powerOfElimQuestID))
				return true
			case 4:
				c.send(dialogWindow(o.id, 2375, powerOfElimQuestID))
				return true
			case 5:
				c.send(dialogWindow(o.id, 2716, powerOfElimQuestID))
				return true
			}
			// Java falls through from unmatched dialog 25 into the collection check.
			return c.powerOfElimCollect(o, script)
		case 33:
			return c.powerOfElimCollect(o, script)
		case 2377:
			c.send(playMovie(187))
			return false
		case 10001:
			if variable == 1 {
				return advance(2)
			}
			// Java falls through into case 10004 when the variable is four.
			fallthrough
		case 10004:
			if variable == 4 {
				c.s.removeItemsByID(c.player, powerOfElimArtifactID, 1)
				c.s.removeItemsByID(c.player, powerOfElimReportItemID, 1)
				return advance(5)
			}
		}
	case powerOfElimArtifactNPCID:
		switch dialogID {
		case 25:
			if variable == 2 {
				c.send(dialogWindow(o.id, 1693, powerOfElimQuestID))
				return true
			}
			// Java falls through from dialog 25 into case 10002.
			fallthrough
		case 10002:
			if variable == 2 {
				c.s.addItem(c.player, powerOfElimArtifactID, 1)
				return advance(3)
			}
		}
	case powerOfElimReportNPCID:
		switch dialogID {
		case 25:
			if variable == 3 {
				c.send(dialogWindow(o.id, 2034, powerOfElimQuestID))
				return true
			}
			// Java falls through from dialog 25 into case 10003.
			fallthrough
		case 10003:
			if variable == 3 {
				c.s.addItem(c.player, powerOfElimReportItemID, 1)
				return advance(4)
			}
		}
	}
	return false
}

func (c *conn) powerOfElimCollect(o *object, script *data.QuestScript) bool {
	template := c.s.data.Quests[powerOfElimQuestID]
	if template == nil {
		return false
	}
	if !c.s.hasQuestItems(c.player, template) {
		c.send(dialogWindow(o.id, 10001, powerOfElimQuestID))
		return true
	}
	for _, item := range template.CollectItems {
		c.s.removeItemsByID(c.player, item.ID, item.Count)
	}
	if !c.customQuestProgress(powerOfElimQuestID, questVar(c.player.quest(powerOfElimQuestID).Vars, 0), "REWARD") {
		return false
	}
	c.send(dialogWindow(o.id, 5, powerOfElimQuestID))
	return true
}
