package game

import "aionlightning/game/data"

const (
	swordTranscendenceQuestID int32 = 1097
	swordTranscendencePernos  int32 = 790001
	swordTranscendenceAnusis  int32 = 798316
	swordTranscendenceShugo   int32 = 279034
	swordTranscendenceItemID  int32 = 182206058
)

// swordTranscendenceLevelUp ports the Java prerequisite-gated level-up event.
func (c *conn) swordTranscendenceLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(swordTranscendenceQuestID)
	prerequisite := c.player.quest(1096)
	if quest == nil || quest.Status != "LOCKED" || prerequisite == nil || prerequisite.Status != "COMPLETE" {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Sword of Transcendence", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

// swordTranscendenceDialog handles Pernos, Anusis, and Baoninerk's parts of
// the quest, including the work item given at the final Shugo conversation.
func (c *conn) swordTranscendenceDialog(o *object, script *data.QuestScript, dialogID uint16) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != swordTranscendenceQuestID {
		return false
	}
	if o.npc.ID != swordTranscendencePernos && o.npc.ID != swordTranscendenceAnusis && o.npc.ID != swordTranscendenceShugo {
		return false
	}
	quest := c.player.quest(swordTranscendenceQuestID)
	if o.npc.ID == swordTranscendencePernos && (quest == nil || quest.Status == "NONE" || quest.Status == "COMPLETE") {
		if dialogID != 25 && dialogID != 1002 && dialogID != 1003 && dialogID != 1007 {
			return false
		}
		return c.customQuestStart(o, script, dialogID, 0)
	}
	if quest == nil || quest.Status == "LOCKED" {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != swordTranscendencePernos {
			return false
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 10002, swordTranscendenceQuestID))
		case 1009: // Java re-sends the REWARD state (var 3) before defaultQuestEndDialog's window 5
			c.send(questAccepted(2, *quest))
			c.send(dialogWindow(o.id, 5, swordTranscendenceQuestID))
		case ^uint16(0): // defaultQuestEndDialog answers a plain click in REWARD with window 5
			c.send(dialogWindow(o.id, 5, swordTranscendenceQuestID))
		case 8, 9, 10, 11, 12, 13, 14, 15, 16, 17:
			c.finishQuest(script, o.id, dialogID)
		default:
			return false
		}
		return true
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch o.npc.ID {
	case swordTranscendencePernos:
		if variable == 0 {
			switch dialogID {
			case 25:
				c.send(dialogWindow(o.id, 1011, swordTranscendenceQuestID))
				return true
			case 10000:
				if c.customQuestProgress(swordTranscendenceQuestID, setQuestVar(quest.Vars, 0, 1), "") {
					c.send(dialogWindow(o.id, 10, 0))
					return true
				}
			}
		}
		if variable == 3 {
			switch dialogID {
			case 25:
				c.send(dialogWindow(o.id, 10002, swordTranscendenceQuestID))
				return true
			case 1009:
				if c.customQuestProgress(swordTranscendenceQuestID, quest.Vars, "REWARD") {
					c.send(dialogWindow(o.id, 5, swordTranscendenceQuestID))
					return true
				}
			}
		}
	case swordTranscendenceAnusis:
		if variable == 1 {
			switch dialogID {
			case 25:
				c.send(dialogWindow(o.id, 1352, swordTranscendenceQuestID))
				return true
			case 10001:
				if c.customQuestProgress(swordTranscendenceQuestID, setQuestVar(quest.Vars, 0, 2), "") {
					c.send(dialogWindow(o.id, 10, 0))
					return true
				}
			}
		}
	case swordTranscendenceShugo:
		if variable == 2 {
			switch dialogID {
			case 25:
				c.send(dialogWindow(o.id, 1693, swordTranscendenceQuestID))
				return true
			case 33:
				if !c.s.questRewardsFit(c.player, []data.QuestItem{{ID: swordTranscendenceItemID, Count: 1}}) {
					c.send(systemMessage(msgInventoryFull))
					return true
				}
				if c.customQuestProgress(swordTranscendenceQuestID, setQuestVar(quest.Vars, 0, 3), "") {
					c.s.addItem(c.player, swordTranscendenceItemID, 1)
					c.send(dialogWindow(o.id, 10, 0))
					return true
				}
			}
		}
	}
	return false
}
