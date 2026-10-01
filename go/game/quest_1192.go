package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	verteronReinforcementsQuestID      int32 = 1192
	verteronReinforcementsStartNPCID    int32 = 203098
	verteronReinforcementsFirstNPCID    int32 = 203701
	verteronReinforcementsSecondNPCID   int32 = 203833
)

func (c *conn) verteronReinforcementsDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != verteronReinforcementsQuestID {
		return false
	}
	quest := c.player.quest(verteronReinforcementsQuestID)
	if quest == nil || quest.Status == "NONE" {
		if o.npc.ID != verteronReinforcementsStartNPCID {
			return false
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 1011, verteronReinforcementsQuestID))
			return true
		case 1002:
			if !c.s.canStartQuest(c.player, script) {
				return false
			}
			c.startQuest(script, o.id)
			return c.player.quest(verteronReinforcementsQuestID) != nil
		default:
			return false
		}
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != verteronReinforcementsStartNPCID {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, verteronReinforcementsQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuestReward(script, o.id, 17, 0)
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
	case verteronReinforcementsFirstNPCID:
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1352, verteronReinforcementsQuestID))
			return true
		}
		if dialogID == 10000 {
			return c.verteronReinforcementsAdvance(o, quest, variable+1)
		}
		// Java's NPC switch falls through from the first reporter to the second
		// and final reporters when their dialog IDs are sent to this NPC.
		if dialogID == 10001 {
			return c.verteronReinforcementsAdvance(o, quest, variable+1)
		}
	case verteronReinforcementsSecondNPCID:
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1693, verteronReinforcementsQuestID))
			return true
		}
		if dialogID == 10001 {
			return c.verteronReinforcementsAdvance(o, quest, variable+1)
		}
	case verteronReinforcementsStartNPCID:
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 2375, verteronReinforcementsQuestID))
			return true
		}
		if dialogID == 1009 {
			return c.verteronReinforcementsReward(o, quest)
		}
	}
	return false
}

func (c *conn) verteronReinforcementsAdvance(o *object, quest *store.Quest, variable int32) bool {
	if !c.customQuestProgress(verteronReinforcementsQuestID, setQuestVar(quest.Vars, 0, variable), "") {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}

func (c *conn) verteronReinforcementsReward(o *object, quest *store.Quest) bool {
	if !c.customQuestProgress(verteronReinforcementsQuestID, setQuestVar(quest.Vars, 0, 3), "REWARD") {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}
