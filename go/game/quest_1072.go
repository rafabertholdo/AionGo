package game

import "aionlightning/game/data"

const (
	abyssTrainingQuestID      int32 = 1072
	abyssTrainingStartNPCID   int32 = 278627
	abyssTrainingSecondNPCID  int32 = 278628
	abyssTrainingThirdNPCID   int32 = 278629
	abyssTrainingFourthNPCID  int32 = 278630
	abyssTrainingFifthNPCID   int32 = 278631
	abyssTrainingSixthNPCID   int32 = 278632
	abyssTrainingSeventhNPCID int32 = 278633
	abyssTrainingEndNPCID     int32 = 278554
	abyssTrainingPrerequisite int32 = 1701
)

func (c *conn) abyssTrainingLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(abyssTrainingQuestID)
	prerequisite := c.player.quest(abyssTrainingPrerequisite)
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || quest.Status != "LOCKED" {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Abyss Training", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) abyssTrainingDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != abyssTrainingQuestID {
		return false
	}
	quest := c.player.quest(abyssTrainingQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != abyssTrainingEndNPCID {
			return false
		}
		switch dialogID {
		case -1:
			c.send(dialogWindow(o.id, 10002, abyssTrainingQuestID))
			return true
		case 1009:
			c.send(dialogWindow(o.id, 5, abyssTrainingQuestID))
			return true
		default:
			if dialogID >= 8 && dialogID <= 17 {
				c.finishQuestReward(script, o.id, uint16(dialogID), 0)
				return true
			}
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	advance := func(nextVariable int32) bool {
		if !c.customQuestProgress(abyssTrainingQuestID, setQuestVar(quest.Vars, 0, nextVariable), "") {
			return false
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	}
	switch o.npc.ID {
	case abyssTrainingStartNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, abyssTrainingQuestID))
				return true
			}
			fallthrough // Java falls through from the unmatched offer case into its movie case.
		case 1013:
			c.send(playMovie(262))
			return false
		case 10000:
			if variable == 0 {
				return advance(1)
			}
		}
	case abyssTrainingSecondNPCID:
		switch dialogID {
		case 25:
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, abyssTrainingQuestID))
				return true
			}
			fallthrough
		case 1353:
			c.send(playMovie(263))
			return false
		case 10001:
			if variable == 1 {
				return advance(2)
			}
		}
	case abyssTrainingThirdNPCID:
		switch dialogID {
		case 25:
			if variable == 2 {
				c.send(dialogWindow(o.id, 1693, abyssTrainingQuestID))
				return true
			}
			fallthrough
		case 1694:
			c.send(playMovie(264))
			return false
		case 10002:
			if variable == 2 {
				return advance(3)
			}
		}
	case abyssTrainingFourthNPCID:
		switch dialogID {
		case 25:
			if variable == 3 {
				c.send(dialogWindow(o.id, 2034, abyssTrainingQuestID))
				return true
			}
			fallthrough
		case 2035:
			c.send(playMovie(265))
			return false
		case 10003:
			if variable == 3 {
				return advance(4)
			}
		}
	case abyssTrainingFifthNPCID:
		switch dialogID {
		case 25:
			if variable == 4 {
				c.send(dialogWindow(o.id, 2375, abyssTrainingQuestID))
				return true
			}
			fallthrough
		case 2376:
			c.send(playMovie(266))
			return false
		case 10004:
			if variable == 4 {
				return advance(5)
			}
		}
	case abyssTrainingSixthNPCID:
		switch dialogID {
		case 25:
			if variable == 5 {
				c.send(dialogWindow(o.id, 2716, abyssTrainingQuestID))
				return true
			}
			fallthrough
		case 2717:
			c.send(playMovie(267))
			return false
		case 10005:
			if variable == 5 {
				return advance(6)
			}
		}
	case abyssTrainingSeventhNPCID:
		switch dialogID {
		case 25:
			if variable == 6 {
				c.send(dialogWindow(o.id, 3057, abyssTrainingQuestID))
				return true
			}
			fallthrough
		case 3058:
			c.send(playMovie(268))
			return false
		case 10255:
			if variable == 6 {
				if !c.customQuestProgress(abyssTrainingQuestID, quest.Vars, "REWARD") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	}
	return false
}
