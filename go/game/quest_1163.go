package game

import "aionlightning/game/data"

const (
	arachnaAntidoteQuestID     int32 = 1163
	arachnaAntidoteStartNPCID  int32 = 203096
	arachnaAntidoteMiddleNPCID int32 = 203151
	arachnaAntidoteEndNPCID    int32 = 203155
)

// arachnaAntidoteDialog ports the three NPC dialogue flow from the Java
// handler. The middle NPC advances on dialog 10000; Java does not guard that
// transition by quest variable.
func (c *conn) arachnaAntidoteDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != arachnaAntidoteQuestID {
		return false
	}
	quest := c.player.quest(arachnaAntidoteQuestID)
	if quest == nil || quest.Status == "NONE" {
		if o.npc.ID != arachnaAntidoteStartNPCID {
			return false
		}
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1011, arachnaAntidoteQuestID))
			return true
		}
		if dialogID != 1002 && dialogID != 1003 && dialogID != 1007 {
			return false
		}
		return c.customQuestStart(o, script, uint16(dialogID), 0)
	}

	switch quest.Status {
	case "START":
		switch o.npc.ID {
		case arachnaAntidoteMiddleNPCID:
			switch dialogID {
			case 25:
				c.send(dialogWindow(o.id, 1352, arachnaAntidoteQuestID))
				return true
			case 10000:
				if !c.customQuestProgress(arachnaAntidoteQuestID, setQuestVar(quest.Vars, 0, questVar(quest.Vars, 0)+1), "") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				return true
			default:
				return false
			}
		case arachnaAntidoteEndNPCID:
			switch dialogID {
			case 25:
				c.send(dialogWindow(o.id, 2375, arachnaAntidoteQuestID))
				return true
			case 1009:
				if !c.customQuestProgress(arachnaAntidoteQuestID, quest.Vars, "REWARD") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				return true
			default:
				return false
			}
		default:
			return false
		}
	case "REWARD":
		if o.npc.ID != arachnaAntidoteEndNPCID {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, arachnaAntidoteQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			// The Java handler accepts every default choice. This quest has no
			// selectable reward, so all choices resolve to the fixed reward.
			c.finishQuestReward(script, o.id, 17, 0)
			return true
		default:
			return false
		}
	default:
		return false
	}
}
