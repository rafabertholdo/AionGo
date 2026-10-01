package game

import (
	"aionlightning/game/data"
	"aionlightning/wire"
)

const (
	aNestOfLepharistsQuestID int32 = 1023
	aNestLepharistProofID    int32 = 182200026
	aNestMaskItemID          int32 = 182200010
)

// aNestOfLepharistsLevelUp unlocks the level 15 quest.
func (c *conn) aNestOfLepharistsLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(aNestOfLepharistsQuestID)
	template := c.s.data.Quests[aNestOfLepharistsQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking A Nest of Lepharists", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func aNestOfLepharistsMovie(id uint16) *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(0)
	w.D(0)
	w.D(0)
	w.H(id)
	w.D(0)
	return w
}

// aNestOfLepharistsEnterZone records entry into the Mysterious Shipwreck at
// stage 2. The shared zone dispatcher scopes this to Verteron.
func (c *conn) aNestOfLepharistsEnterZone(zoneName string) bool {
	if c == nil || c.player == nil || zoneName != "MYSTERIOUS_SHIPWRECK" {
		return false
	}
	quest := c.player.quest(aNestOfLepharistsQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 2 {
		return false
	}
	return c.customQuestProgress(aNestOfLepharistsQuestID, setQuestVar(quest.Vars, 0, 3), "")
}

// aNestOfLepharistsDialog ports the captain's report, shipwreck movie, special
// proof-item condition, and selectable reward turn-in.
func (c *conn) aNestOfLepharistsDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != aNestOfLepharistsQuestID {
		return false
	}
	quest := c.player.quest(aNestOfLepharistsQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch quest.Status {
	case "REWARD":
		if o.npc.ID != 203098 {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, aNestOfLepharistsQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			if dialogID > 11 {
				// Java's default end handler accepts 8–17; out-of-range option
				// indices complete with the fixed reward only.
				c.finishQuest(script, o.id, 17)
				return true
			}
			c.finishQuestReward(script, o.id, uint16(dialogID), 0)
			return true
		default:
			return false
		}
	case "START":
	default:
		return false
	}
	switch o.npc.ID {
	case 203098:
		if variable != 0 {
			return false
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 1011, aNestOfLepharistsQuestID))
			return true
		case 10000:
			if c.customQuestProgress(aNestOfLepharistsQuestID, setQuestVar(quest.Vars, 0, 1), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case 203183:
		if dialogID == 1012 {
			c.send(aNestOfLepharistsMovie(30))
			return true
		}
		if dialogID == 25 && variable == 1 {
			c.send(dialogWindow(o.id, 1011, aNestOfLepharistsQuestID))
			return true
		}
		if dialogID == 10000 && variable == 1 {
			if c.customQuestProgress(aNestOfLepharistsQuestID, setQuestVar(quest.Vars, 0, 2), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
		if dialogID == 10000 && variable == 3 {
			c.send(dialogWindow(o.id, 1352, aNestOfLepharistsQuestID))
			return true
		}
		if dialogID == 10001 && variable == 3 {
			if c.customQuestProgress(aNestOfLepharistsQuestID, setQuestVar(quest.Vars, 0, 4), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
		if dialogID == 10001 && variable == 4 {
			if c.s.countItems(c.player, aNestLepharistProofID) < 1 {
				c.send(dialogWindow(o.id, 1779, aNestOfLepharistsQuestID))
				return true
			}
			if !c.customQuestProgress(aNestOfLepharistsQuestID, setQuestVar(quest.Vars, 0, 5), "REWARD") {
				return false
			}
			c.send(aNestOfLepharistsMovie(23))
			c.s.removeItemsByID(c.player, aNestMaskItemID, 1)
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	}
	return false
}

// aNestOfLepharistsShowDialog shows the default reward preview at Latius.
func (c *conn) aNestOfLepharistsShowDialog(o *object, script *data.QuestScript) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != 203098 ||
		script == nil || script.ID != aNestOfLepharistsQuestID {
		return false
	}
	quest := c.player.quest(aNestOfLepharistsQuestID)
	if quest == nil || quest.Status != "REWARD" {
		return false
	}
	c.send(dialogWindow(o.id, 5, aNestOfLepharistsQuestID))
	return true
}
