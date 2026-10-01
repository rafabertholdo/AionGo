package game

import (
	"aionlightning/game/data"
)

const (
	markOfVengeanceQuestID int32 = 1018
	markOfVengeanceNPCID   int32 = 203098
)

func (c *conn) markOfVengeanceLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(markOfVengeanceQuestID)
	template := c.s.data.Quests[markOfVengeanceQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Mark of Vengeance", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) markOfVengeanceDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != markOfVengeanceNPCID || script == nil || script.ID != markOfVengeanceQuestID {
		return false
	}
	player := c.player
	quest := player.quest(markOfVengeanceQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, markOfVengeanceQuestID))
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
	switch dialogID {
	case 25:
		if questVar(quest.Vars, 0) == 0 {
			c.send(dialogWindow(o.id, 1011, markOfVengeanceQuestID))
			return true
		}
		fallthrough // Java's case 25 has no return for var != 0 and runs case 33
	case 33:
		template := c.s.data.Quests[markOfVengeanceQuestID]
		if !c.s.hasQuestItems(player, template) {
			c.send(dialogWindow(o.id, 1097, markOfVengeanceQuestID))
			return true
		}
		if !c.customQuestProgress(markOfVengeanceQuestID, setQuestVar(quest.Vars, 0, questVar(quest.Vars, 0)+1), "REWARD") {
			return false
		}
		for _, item := range template.CollectItems {
			c.s.removeItemsByID(player, item.ID, item.Count)
		}
		c.send(dialogWindow(o.id, 5, markOfVengeanceQuestID))
		return true
	default:
		return false
	}
}
