package game

import (
	"aionlightning/game/data"
	"aionlightning/wire"
)

const (
	missingPoppyQuestID  int32 = 1149
	missingPoppyStartNPC int32 = 203145
	missingPoppyNPC      int32 = 203191
)

// missingPoppyMovie plays Java's rescue scene after Poppy reaches safety.
func missingPoppyMovie() *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(0)
	w.D(0)
	w.D(0)
	w.H(12)
	w.D(0)
	return w
}

// missingPoppyDialog handles the quest offer, Poppy's escort, and reward.
func (c *conn) missingPoppyDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != missingPoppyQuestID {
		return false
	}
	p := c.player
	quest := p.quest(missingPoppyQuestID)
	if quest == nil || quest.Status == "NONE" {
		if o.npc.ID != missingPoppyStartNPC {
			return false
		}
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1011, missingPoppyQuestID))
			return true
		}
		return c.customQuestStart(o, script, uint16(dialogID), 0)
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != missingPoppyStartNPC {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, missingPoppyQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			// Java accepts its generic end range for this fixed-reward quest.
			c.finishQuest(script, o.id, 17)
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
	case missingPoppyNPC:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1352, missingPoppyQuestID))
				return true
			}
		case 10000:
			if variable == 0 && c.customQuestProgress(missingPoppyQuestID, setQuestVar(quest.Vars, 0, 1), "") {
				o.move.distance = 4
				o.move.follow = true
				o.targetID = p.ID
				c.s.scheduleMove(o)
				c.send(dialogWindow(o.id, 0, 0))
				return true
			}
		case -1:
			if variable != 1 {
				return false
			}
			if distance3D(o.x, o.y, o.z, 1255, 2223, 144) > 5 {
				o.move.distance = 4
				o.move.follow = true
				o.targetID = p.ID
				c.s.scheduleMove(o)
				return true
			}
			if !c.customQuestProgress(missingPoppyQuestID, quest.Vars, "REWARD") {
				return false
			}
			c.send(missingPoppyMovie())
			o.move.stop()
			o.dead, o.hp = true, 0
			c.s.npcDied(o, nil)
			c.s.despawnNpc(o, false)
			return true
		}
	}
	return false
}

// missingPoppyShowDialog maps the NPC click event to Java's -1 escort event.
func (c *conn) missingPoppyShowDialog(o *object, script *data.QuestScript) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.npc.ID != missingPoppyNPC ||
		script == nil || script.ID != missingPoppyQuestID {
		return false
	}
	return c.missingPoppyDialog(o, script, -1)
}
