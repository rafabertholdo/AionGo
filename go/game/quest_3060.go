package game

import "aionlightning/game/data"

import "aionlightning/game/store"

const (
	redJournalQuestID int32 = 3060
	redJournalItemID  int32 = 182208043
	redJournalStart   int32 = 798190
	redJournalSecond  int32 = 798191
	redJournalThird   int32 = 798192
	redJournalEnd     int32 = 798193
)

// redJournalDialog ports The Red Journal's four-NPC conversation and item
// turn-in. Its three-second item-use animation is handled by the shared delay.
func (c *conn) redJournalDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || script == nil || script.ID != redJournalQuestID {
		return false
	}
	if o == nil || o.npc == nil || o.dead {
		if dialogID != 1002 {
			return false
		}
		if c.s.canStartQuest(c.player, script) && c.player.level >= c.s.data.Quests[script.ID].MinLevel {
			started := store.Quest{ID: script.ID, Status: "START"}
			if err := c.s.quests.SaveQuest(c.player.ID, started); err != nil {
				c.s.log.Error("starting Red Journal", "err", err)
			} else {
				c.player.quests = append(c.player.quests, started)
				c.send(questAccepted(1, started))
				c.send(c.s.nearbyQuests(c.player))
			}
		}
		c.send(dialogWindow(0, 0, 0))
		return true
	}
	quest := c.player.quest(redJournalQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != redJournalEnd {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, redJournalQuestID))
			return true
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuest(script, o.id, uint16(dialogID))
			return true
		default:
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	page := map[int32]uint16{redJournalStart: 1352, redJournalSecond: 1693, redJournalThird: 2034}
	accept := map[int32]int32{redJournalStart: 0, redJournalSecond: 1, redJournalThird: 2}
	if expected, ok := accept[o.npc.ID]; ok && variable == expected {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, page[o.npc.ID], redJournalQuestID))
			return true
		}
		if dialogID == 10000+expected {
			if c.customQuestProgress(redJournalQuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
			return false
		}
		switch dialogID {
		case 1002:
			c.startQuest(script, o.id)
			return true
		case 1003:
			c.send(dialogWindow(o.id, 1004, redJournalQuestID))
			return true
		case 1007:
			c.send(dialogWindow(o.id, 4, redJournalQuestID))
			return true
		default:
			return false
		}
	}
	if o.npc.ID == redJournalEnd {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 2375, redJournalQuestID))
			return true
		}
		if dialogID == 1009 {
			if count := c.s.countItems(c.player, redJournalItemID); count > 0 {
				c.s.removeItemsByID(c.player, redJournalItemID, count)
			}
			if c.customQuestProgress(redJournalQuestID, setQuestVar(quest.Vars, 0, 1), "REWARD") {
				c.send(dialogWindow(o.id, 5, redJournalQuestID))
				return true
			}
		}
		return false
	}
	return false
}
