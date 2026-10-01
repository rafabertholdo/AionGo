package game

import (
	"aionlightning/game/data"
	"aionlightning/wire"
)

const frillneckHuntQuestID int32 = 1015

// frillneckHuntLevelUp unlocks Frillneck Hunt when its minimum level is met.
func (c *conn) frillneckHuntLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(frillneckHuntQuestID)
	template := c.s.data.Quests[frillneckHuntQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Frillneck Hunt", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

// frillneckHuntKill applies the two kill counters from the Java handler.
func (c *conn) frillneckHuntKill(npcID int32) bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(frillneckHuntQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch npcID {
	case 210126:
		if variable >= 1 && variable <= 7 {
			return c.customQuestProgress(frillneckHuntQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
		}
	case 210200, 210201:
		if variable >= 9 && variable <= 20 {
			if variable == 20 {
				return c.customQuestProgress(frillneckHuntQuestID, quest.Vars, "REWARD")
			}
			return c.customQuestProgress(frillneckHuntQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
		}
	}
	return false
}

// frillneckHuntDialog handles Leto's progress conversation and reward turn-in.
func (c *conn) frillneckHuntDialog(o *object, script *data.QuestScript, dialogID int32) {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != frillneckHuntQuestID || o.npc.ID != 203129 {
		return
	}
	quest := c.player.quest(frillneckHuntQuestID)
	if quest == nil {
		return
	}
	if quest.Status == "REWARD" {
		if dialogID == -1 {
			c.send(dialogWindow(o.id, 1693, frillneckHuntQuestID))
			return
		}
		c.customQuestEnd(o, script, uint16(dialogID))
		return
	}
	if quest.Status != "START" {
		return
	}
	variable := questVar(quest.Vars, 0)
	switch dialogID {
	case 25:
		switch variable {
		case 0:
			c.send(dialogWindow(o.id, 1011, frillneckHuntQuestID))
		case 8:
			c.send(dialogWindow(o.id, 1352, frillneckHuntQuestID))
		default:
			c.send(frillneckHuntMovie())
			c.send(dialogWindow(o.id, 1012, frillneckHuntQuestID))
		}
	case 1012:
		c.send(frillneckHuntMovie())
		c.send(dialogWindow(o.id, 1012, frillneckHuntQuestID))
	case 10000, 10001:
		if variable == 0 || variable == 8 {
			if c.customQuestProgress(frillneckHuntQuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
				c.send(dialogWindow(o.id, 10, 0))
			}
		}
	}
}

// frillneckHuntShowDialog is Java's reward preview when the player clicks Leto.
func (c *conn) frillneckHuntShowDialog(o *object, script *data.QuestScript) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != frillneckHuntQuestID || o.npc.ID != 203129 {
		return false
	}
	quest := c.player.quest(frillneckHuntQuestID)
	if quest == nil || quest.Status != "REWARD" {
		return false
	}
	c.send(dialogWindow(o.id, 1693, frillneckHuntQuestID))
	return true
}

func frillneckHuntMovie() *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(0)
	w.D(0)
	w.D(0)
	w.H(27)
	w.D(0)
	return w
}
