package game

import "aionlightning/game/data"

const (
	balaurConspiracyQuestID   int32 = 1043
	balaurConspiracyStartNPC  int32 = 203901
	balaurConspiracySecondNPC int32 = 204020
	balaurConspiracyFinalNPC  int32 = 204044
	balaurConspiracyTargetID  int32 = 211629
	balaurConspiracyWorldID   int32 = 210020000
)

var balaurConspiracyPrerequisites = [...]int32{1300, 1031, 1032, 1033, 1034, 1036, 1037, 1035, 1038, 1039, 1040, 1041, 1042}

func (c *conn) balaurConspiracyLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(balaurConspiracyQuestID)
	if quest == nil || quest.Status != "LOCKED" {
		return false
	}
	for _, prerequisiteID := range balaurConspiracyPrerequisites {
		prerequisite := c.player.quest(prerequisiteID)
		if prerequisite == nil || prerequisite.Status != "COMPLETE" {
			return false
		}
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Balaur Conspiracy", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) balaurConspiracyDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != balaurConspiracyQuestID {
		return false
	}
	quest := c.player.quest(balaurConspiracyQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != balaurConspiracyStartNPC {
			return false
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 2375, balaurConspiracyQuestID))
			return true
		case 1009:
			c.send(dialogWindow(o.id, 5, balaurConspiracyQuestID))
			return true
		default:
			if dialogID == -1 {
				c.send(dialogWindow(o.id, 5, balaurConspiracyQuestID))
				return true
			}
			if dialogID >= 8 && dialogID <= 17 {
				c.finishQuestReward(script, o.id, uint16(dialogID), 0)
				return true
			}
		}
		return false
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if dialogID == 25 {
		switch {
		case variable == 0 && o.npc.ID == balaurConspiracyStartNPC:
			c.send(dialogWindow(o.id, 1011, balaurConspiracyQuestID))
			return true
		case variable == 1 && (o.npc.ID == balaurConspiracyStartNPC || o.npc.ID == balaurConspiracySecondNPC):
			c.send(dialogWindow(o.id, 1352, balaurConspiracyQuestID))
			return true
		case variable == 2 && (o.npc.ID == balaurConspiracyStartNPC || o.npc.ID == balaurConspiracySecondNPC || o.npc.ID == balaurConspiracyFinalNPC):
			c.send(dialogWindow(o.id, 1693, balaurConspiracyQuestID))
			return true
		case variable == 4 && (o.npc.ID == balaurConspiracyStartNPC || o.npc.ID == balaurConspiracySecondNPC || o.npc.ID == balaurConspiracyFinalNPC):
			c.send(dialogWindow(o.id, 2034, balaurConspiracyQuestID))
			return true
		}
		return false
	}
	advance := func() bool {
		return c.customQuestProgress(balaurConspiracyQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
	}
	switch dialogID {
	case 10000:
		if o.npc.ID == balaurConspiracyStartNPC {
			if !advance() {
				return false
			}
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case 10001:
		if o.npc.ID == balaurConspiracyStartNPC || o.npc.ID == balaurConspiracySecondNPC {
			if !advance() {
				return false
			}
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case 10002:
		if o.npc.ID == balaurConspiracyStartNPC || o.npc.ID == balaurConspiracySecondNPC || o.npc.ID == balaurConspiracyFinalNPC {
			if !advance() {
				return false
			}
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case 10003:
		if o.npc.ID == balaurConspiracyStartNPC || o.npc.ID == balaurConspiracySecondNPC || o.npc.ID == balaurConspiracyFinalNPC {
			if !c.customQuestProgress(balaurConspiracyQuestID, setQuestVar(quest.Vars, 0, 4), "REWARD") {
				return false
			}
			c.s.teleportTo(c.player, balaurConspiracyWorldID, 2502.1948, 782.9152, 408.97723, 0, 0)
			return true
		}
	}
	return false
}

func (c *conn) balaurConspiracyKill(npcID int32) bool {
	if c == nil || c.player == nil || npcID != balaurConspiracyTargetID {
		return false
	}
	quest := c.player.quest(balaurConspiracyQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 3 {
		return false
	}
	return c.customQuestProgress(balaurConspiracyQuestID, setQuestVar(quest.Vars, 0, 4), "")
}
