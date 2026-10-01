package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	scoutingTheScoutsQuestID int32 = 1040
	scoutingTheScoutsNPCID   int32 = 203989
	scoutingScoutsReportNPC  int32 = 203901
	scoutingScoutsGuideNPC   int32 = 204020
	scoutingScoutsScoutNPC   int32 = 204024
	scoutingScoutsWorldID    int32 = 210020000
)

func (c *conn) scoutingTheScoutsLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(scoutingTheScoutsQuestID)
	template := c.s.data.Quests[scoutingTheScoutsQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	for _, prerequisiteID := range template.FinishedQuestConditions {
		prerequisite := c.player.quest(prerequisiteID)
		if prerequisite == nil || prerequisite.Status != "COMPLETE" {
			return false
		}
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Scouting the Scouts", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) scoutingTheScoutsDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != scoutingTheScoutsQuestID {
		return false
	}
	quest := c.player.quest(scoutingTheScoutsQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID == scoutingTheScoutsNPCID {
			switch {
			case dialogID == -1 || dialogID == 1009:
				c.send(dialogWindow(o.id, 5, scoutingTheScoutsQuestID))
				return true
			case dialogID >= 8 && dialogID <= 17:
				c.finishQuest(script, o.id, uint16(dialogID))
				return true
			}
		}
		return false
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	advance := func(nextVariable int32) bool {
		if !c.customQuestProgress(scoutingTheScoutsQuestID, setQuestVar(quest.Vars, 0, nextVariable), "") {
			return false
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	}
	switch o.npc.ID {
	case scoutingTheScoutsNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, scoutingTheScoutsQuestID))
				return true
			}
			if variable == 4 {
				c.send(dialogWindow(o.id, 1352, scoutingTheScoutsQuestID))
				return true
			}
		case 1013:
			if variable == 0 {
				c.send(ascensionMovie(183))
			}
		case 10000:
			if variable == 0 {
				return advance(1)
			}
			// The Java switch falls through from case 10000 into case 10001.
			if variable == 4 {
				return advance(5)
			}
		case 10001:
			if variable == 4 {
				return advance(5)
			}
		}
	case scoutingScoutsReportNPC:
		switch dialogID {
		case 25:
			if variable == 5 {
				c.send(dialogWindow(o.id, 1693, scoutingTheScoutsQuestID))
				return true
			}
		case 10002:
			if variable == 5 {
				return advance(6)
			}
		}
	case scoutingScoutsGuideNPC:
		switch dialogID {
		case 25:
			switch variable {
			case 6:
				c.send(dialogWindow(o.id, 2034, scoutingTheScoutsQuestID))
				return true
			case 10:
				c.send(dialogWindow(o.id, 3057, scoutingTheScoutsQuestID))
				return true
			}
		case 10003:
			if variable == 6 {
				if !c.s.teleportTo(c.player, scoutingScoutsWorldID, 2211, 811, 513, 0, 0) {
					return false
				}
				return c.customQuestProgress(scoutingTheScoutsQuestID, setQuestVar(quest.Vars, 0, 7), "")
			}
			// The Java switch falls through from case 10003 into case 10006.
			if variable == 10 {
				return c.scoutingTheScoutsMakeReward(o, quest)
			}
		case 10006:
			if variable == 10 {
				return c.scoutingTheScoutsMakeReward(o, quest)
			}
		}
	case scoutingScoutsScoutNPC:
		switch dialogID {
		case 25:
			switch variable {
			case 7:
				c.send(dialogWindow(o.id, 2375, scoutingTheScoutsQuestID))
				return true
			case 9:
				c.send(dialogWindow(o.id, 2716, scoutingTheScoutsQuestID))
				return true
			}
		case 10004:
			if variable == 7 {
				return advance(8)
			}
			// The Java switch falls through from case 10004 into case 10005.
			if variable == 9 {
				if !c.s.teleportTo(c.player, scoutingScoutsWorldID, 1606, 1529, 318, 0, 0) {
					return false
				}
				return c.customQuestProgress(scoutingTheScoutsQuestID, setQuestVar(quest.Vars, 0, 10), "")
			}
		case 10005:
			if variable == 9 {
				if !c.s.teleportTo(c.player, scoutingScoutsWorldID, 1606, 1529, 318, 0, 0) {
					return false
				}
				return c.customQuestProgress(scoutingTheScoutsQuestID, setQuestVar(quest.Vars, 0, 10), "")
			}
		}
	}
	return false
}

func (c *conn) scoutingTheScoutsMakeReward(o *object, quest *store.Quest) bool {
	if !c.customQuestProgress(scoutingTheScoutsQuestID, quest.Vars, "REWARD") {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}

func (c *conn) scoutingTheScoutsKill(npcID int32) bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(scoutingTheScoutsQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch npcID {
	case 212010:
		if variable <= 0 || variable >= 4 {
			return false
		}
		return c.customQuestProgress(scoutingTheScoutsQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
	case 204046:
		if variable <= 7 || variable >= 9 {
			return false
		}
		c.send(ascensionMovie(36))
		return c.customQuestProgress(scoutingTheScoutsQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
	default:
		return false
	}
}
