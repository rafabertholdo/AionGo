package game

import "aionlightning/game/data"

const (
	secretDeliveryQuestID  int32 = 1220
	secretDeliveryStartID  int32 = 203172
	secretDeliveryMiddleID int32 = 798004
	secretDeliveryEndID    int32 = 798046
)

// secretDeliveryDialog preserves Java's fall-through from the middle report
// into the final-NPC branch for every dialog except the two handled directly.
func (c *conn) secretDeliveryDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || c.s == nil || o == nil || o.npc == nil || script == nil || script.ID != secretDeliveryQuestID {
		return false
	}
	quest := c.player.quest(secretDeliveryQuestID)
	if quest == nil || quest.Status == "NONE" {
		if o.npc.ID != secretDeliveryStartID {
			return false
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 1011, secretDeliveryQuestID))
			return true
		case 1007:
			c.send(dialogWindow(o.id, 4, secretDeliveryQuestID))
			return true
		case 1003:
			c.send(dialogWindow(o.id, 1004, secretDeliveryQuestID))
			return true
		case 1002:
			return c.customQuestStart(o, script, uint16(dialogID), 0)
		default:
			return false
		}
	}

	if quest.Status == "REWARD" {
		if o.npc.ID != secretDeliveryEndID {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, secretDeliveryQuestID))
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuest(script, o.id, uint16(dialogID))
		default:
			return false
		}
		return true
	}
	if quest.Status != "START" {
		return false
	}

	if o.npc.ID == secretDeliveryMiddleID {
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 1352, secretDeliveryQuestID))
			return true
		case 10000:
			if !c.customQuestProgress(secretDeliveryQuestID, setQuestVar(quest.Vars, 0, questVar(quest.Vars, 0)+1), "") {
				return false
			}
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
		// Java's switch has no break here: unmatched middle-NPC dialogs enter
		// the 798046 branch using this same visible NPC object.
	} else if o.npc.ID != secretDeliveryEndID {
		return false
	}

	switch dialogID {
	case 25:
		c.send(dialogWindow(o.id, 2375, secretDeliveryQuestID))
		return true
	case 1009:
		if !c.customQuestProgress(secretDeliveryQuestID, setQuestVar(quest.Vars, 0, 2), "REWARD") {
			return false
		}
		c.send(dialogWindow(o.id, 5, secretDeliveryQuestID))
		return true
	case -1, 1007, 1003, 1002:
		return false
	default:
		if dialogID >= 8 && dialogID <= 17 {
			if quest.Status == "REWARD" {
				c.finishQuest(script, o.id, uint16(dialogID))
			} else {
				c.send(dialogWindow(o.id, 10, 0))
			}
			return true
		}
		return false
	}
}
