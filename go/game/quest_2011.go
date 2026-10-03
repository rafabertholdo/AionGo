package game

import "aionlightning/game/data"

func (c *conn) fungusAmongUsDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if script == nil || script.ID != fungusAmongUsQuestID || o == nil || o.npc == nil {
		return false
	}
	quest := c.player.quest(fungusAmongUsQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != fungusAmongUsCaptainID {
			return false
		}
		return c.finishAltgardStartupQuest(o, script, fungusAmongUsQuestID, dialogID)
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch o.npc.ID {
	case fungusAmongUsCaptainID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, fungusAmongUsQuestID))
				return true
			}
		case 10000:
			if variable == 0 && c.beginAltgardStartupQuest(fungusAmongUsQuestID, setQuestVar(quest.Vars, 0, 1), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case fungusAmongUsScoutID:
		switch dialogID {
		case 25:
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, fungusAmongUsQuestID))
				return true
			}
		case 1352:
			c.send(playMovie(60))
			return false // Java returns false after the movie, so the framework echoes page 1352.
		case 10001:
			if variable == 1 && c.beginAltgardStartupQuest(fungusAmongUsQuestID, setQuestVar(quest.Vars, 0, 2), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	}
	return false
}

func (c *conn) fungusAmongUsKill(dead *object) bool {
	if c == nil || c.player == nil || dead == nil || dead.npc == nil || dead.npc.ID != fungusAmongUsMushroomID {
		return false
	}
	quest := c.player.quest(fungusAmongUsQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch {
	case variable > 0 && variable < 6:
		return c.beginAltgardStartupQuest(fungusAmongUsQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
	case variable == 6:
		if !c.beginAltgardStartupQuest(fungusAmongUsQuestID, quest.Vars, "REWARD") {
			return false
		}
		c.send(dialogWindow(dead.id, 10, 0))
		return true
	default:
		return false
	}
}
