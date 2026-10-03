package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func (c *conn) fearThisDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != fearThisQuestID {
		return false
	}
	quest := c.player.quest(fearThisQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != fearThisReportNPCID {
			return false
		}
		if dialogID == -1 {
			c.send(dialogWindow(o.id, 2375, fearThisQuestID))
			return true
		}
		return c.finishAltgardStartupQuest(o, script, fearThisQuestID, dialogID)
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch o.npc.ID {
	case fearThisReportNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, fearThisQuestID))
				return true
			}
			if variable == 6 {
				c.send(dialogWindow(o.id, 1352, fearThisQuestID))
				return true
			}
		case 1012:
			c.send(playMovie(63))
			return false
		case 10000, 10001:
			if variable == 0 || variable == 6 {
				if c.beginAltgardStartupQuest(fearThisQuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
					c.send(dialogWindow(o.id, 10, 0))
					return true
				}
			}
		}
		if dialogID == 25 && variable != 0 && variable != 6 {
			c.send(playMovie(63))
		}
	case fearThisSupplyNPCID:
		switch dialogID {
		case 25:
			if variable == 7 {
				c.send(dialogWindow(o.id, 1693, fearThisQuestID))
				return true
			}
			if variable == 8 {
				c.send(dialogWindow(o.id, 2034, fearThisQuestID))
				return true
			}
		case 10002, 10003:
			if variable == 7 || variable == 9 {
				if variable == 9 {
					item := []data.QuestItem{{ID: fearThisItemID, Count: 1}}
					if !c.s.questRewardsFit(c.player, item) || !c.s.addItem(c.player, fearThisItemID, 1) {
						return true
					}
				}
				if c.beginAltgardStartupQuest(fearThisQuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
					c.send(dialogWindow(o.id, 10, 0))
					return true
				}
			}
		case 33:
			if variable == 8 {
				template := c.s.data.Quests[fearThisQuestID]
				if !c.s.hasQuestItems(c.player, template) {
					c.send(dialogWindow(o.id, 2120, fearThisQuestID))
					return true
				}
				for _, item := range template.CollectItems {
					c.s.removeItemsByID(c.player, item.ID, item.Count)
				}
				if !c.beginAltgardStartupQuest(fearThisQuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
					return false
				}
				c.send(dialogWindow(o.id, 2035, fearThisQuestID))
				return true
			}
		}
	}
	return false
}

func (c *conn) fearThisKill(npcID int32) bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(fearThisQuestID)
	if quest == nil || quest.Status != "START" || (npcID != 210455 && npcID != 210458 && npcID != 214032) {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if variable >= 6 {
		return false
	}
	return c.beginAltgardStartupQuest(fearThisQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
}

func (c *conn) fearThisItemUse(item *store.Item) bool {
	if c == nil || c.player == nil || item == nil || item.ItemID != fearThisItemID || c.player.cubeItem(item.UniqueID) != item ||
		c.player.zone == nil || c.player.zone.Name != fearThisZone || c.player.quest(fearThisQuestID) == nil {
		return false
	}
	p := c.player
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 3000, 0, 0), true)
	c.s.later(3*time.Second, func() {
		if p.conn != c || p.cubeItem(item.UniqueID) != item {
			return
		}
		p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0), true)
		c.s.removeItemsByID(p, fearThisItemID, 1)
		quest := p.quest(fearThisQuestID)
		if quest == nil {
			return
		}
		next := *quest
		next.Status = "REWARD"
		if err := c.s.quests.SaveQuest(p.ID, next); err != nil {
			c.s.log.Error("completing Fear This item use", "err", err)
			return
		}
		*quest = next
		c.send(questAccepted(2, next))
	})
	return true
}
