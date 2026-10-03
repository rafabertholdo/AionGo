package game

import (
	"aionlightning/game/data"
)

const (
	reducingTursinStrengthQuestID       int32 = 1194
	reducingTursinStrengthStartNPCID    int32 = 203098
	reducingTursinStrengthFirstMobID    int32 = 210185
	reducingTursinStrengthSecondMobID   int32 = 210186
	reducingTursinStrengthRequiredKills int32 = 10
	reducingTursinStrengthZoneName            = "TURSIN_GARRISON"
)

// reducingTursinStrengthDialog ports the starter's offer and reward pages.
// The Java handler intentionally leaves every in-progress conversation to the
// framework's default dialog echo.
func (c *conn) reducingTursinStrengthDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || c.s == nil || o == nil || o.npc == nil || o.dead ||
		script == nil || script.ID != reducingTursinStrengthQuestID || o.npc.ID != reducingTursinStrengthStartNPCID {
		return false
	}

	quest := c.player.quest(reducingTursinStrengthQuestID)
	if quest == nil || quest.Status == "NONE" {
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 1011, reducingTursinStrengthQuestID))
			return true
		case 1007:
			c.send(dialogWindow(o.id, 4, reducingTursinStrengthQuestID))
			return true
		case 1003:
			c.send(dialogWindow(o.id, 1004, reducingTursinStrengthQuestID))
			return true
		case 1002:
			if !c.s.canStartQuest(c.player, script) {
				return false
			}
			c.startQuest(script, o.id)
			started := c.player.quest(reducingTursinStrengthQuestID)
			return started != nil && started.Status == "START"
		default:
			return false
		}
	}

	if quest.Status != "REWARD" {
		return false
	}
	switch dialogID {
	case -1:
		c.send(dialogWindow(o.id, 1352, reducingTursinStrengthQuestID))
		return true
	case 1009:
		c.send(dialogWindow(o.id, 5, reducingTursinStrengthQuestID))
		return true
	default:
		if dialogID < 8 || dialogID > 17 {
			return false
		}
		c.finishQuest(script, o.id, uint16(dialogID))
		return quest.Status == "COMPLETE"
	}
}

// reducingTursinStrengthEnterZone matches the Java zone event: it changes
// variable zero only once, and preserves the quest's current status.
func (c *conn) reducingTursinStrengthEnterZone(zoneName string) bool {
	if c == nil || c.player == nil || c.s == nil || c.player.WorldID != 210030000 || zoneName != reducingTursinStrengthZoneName {
		return false
	}
	quest := c.player.quest(reducingTursinStrengthQuestID)
	if quest == nil || questVar(quest.Vars, 0) != 0 {
		return false
	}

	updated := *quest
	updated.Vars = setQuestVar(updated.Vars, 0, 1)
	if err := c.s.quests.SaveQuest(c.player.ID, updated); err != nil {
		c.s.log.Error("updating Reducing Tursin Strength zone progress", "err", err)
		return false
	}
	*quest = updated
	c.send(questAccepted(2, updated))
	return true
}

// reducingTursinStrengthKill ports the two NPC kill events. The zone entry
// sets variable zero to one; nine kills reach ten, and the following kill
// changes the status to REWARD without advancing the variable.
func (c *conn) reducingTursinStrengthKill(npcID int32) bool {
	if c == nil || c.player == nil || c.s == nil ||
		(npcID != reducingTursinStrengthFirstMobID && npcID != reducingTursinStrengthSecondMobID) {
		return false
	}
	quest := c.player.quest(reducingTursinStrengthQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}

	variable := questVar(quest.Vars, 0)
	if variable >= 1 && variable < reducingTursinStrengthRequiredKills {
		return c.customQuestProgress(reducingTursinStrengthQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
	}
	if variable == reducingTursinStrengthRequiredKills {
		return c.customQuestProgress(reducingTursinStrengthQuestID, quest.Vars, "REWARD")
	}
	return false
}
