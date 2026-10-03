package game

import "aionlightning/game/data"

const (
	krallBookQuestID  int32 = 1197
	krallBookNPCID    int32 = 700004
	krallBookReportID int32 = 203129
	krallBookItemID   int32 = 182200558
)

// krallBookDialog ports the book giver and the report to Pernos.
func (c *conn) krallBookDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || c.s == nil || o == nil || o.npc == nil || script == nil || script.ID != krallBookQuestID {
		return false
	}

	quest := c.player.quest(krallBookQuestID)
	switch o.npc.ID {
	case krallBookNPCID:
		if (quest == nil || quest.Status == "NONE") && c.s.countItems(c.player, krallBookItemID) == 0 {
			if c.s.addItem(c.player, krallBookItemID, 1) {
				c.s.despawnNpc(o, true)
			}
		}
		return true
	case krallBookReportID:
		if quest == nil {
			return false
		}
		if quest.Status == "START" {
			switch dialogID {
			case 25:
				c.send(dialogWindow(o.id, 2375, krallBookQuestID))
				return true
			case 1009:
				c.s.removeItemsByID(c.player, krallBookItemID, c.s.countItems(c.player, krallBookItemID))
				if !c.customQuestProgress(krallBookQuestID, setQuestVar(quest.Vars, 0, 1), "REWARD") {
					return false
				}
			}
		}
		if quest.Status == "REWARD" {
			switch {
			case dialogID == -1 || dialogID == 1009:
				c.send(dialogWindow(o.id, 5, krallBookQuestID))
				return true
			case dialogID >= 8 && dialogID <= 17:
				c.finishQuest(script, o.id, uint16(dialogID))
				return true
			}
		}
	}
	return false
}
