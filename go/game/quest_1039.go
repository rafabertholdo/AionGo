package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	somethingInTheWaterQuestID    int32 = 1039
	somethingInTheWaterNPCID      int32 = 203946
	somethingInTheWaterJumentis   int32 = 203705
	somethingInTheWaterBottle     int32 = 182201009
	somethingInTheWaterFullBottle int32 = 182201010
	somethingInTheWaterZone             = "MYSTIC_SPRING_OF_AGAIRON"
)

func (c *conn) somethingInTheWaterLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(somethingInTheWaterQuestID)
	template := c.s.data.Quests[somethingInTheWaterQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	for _, prerequisiteID := range template.FinishedQuestConditions {
		prerequisite := c.player.quest(prerequisiteID)
		if prerequisite == nil || prerequisite.Status != "COMPLETE" {
			return false
		}
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Something in the Water", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) somethingInTheWaterDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != somethingInTheWaterQuestID {
		return false
	}
	quest := c.player.quest(somethingInTheWaterQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != somethingInTheWaterNPCID {
			return false
		}
		switch dialogID {
		case -1, 1009:
			c.send(dialogWindow(o.id, 5, somethingInTheWaterQuestID))
			return true
		default:
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
	switch o.npc.ID {
	case somethingInTheWaterNPCID:
		switch dialogID {
		case -1:
			if variable == 0 || variable == 3 {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		case 25:
			switch variable {
			case 0:
				c.send(dialogWindow(o.id, 1011, somethingInTheWaterQuestID))
				return true
			case 3:
				c.send(dialogWindow(o.id, 1693, somethingInTheWaterQuestID))
				return true
			}
		case 10000:
			if variable == 0 && c.s.questRewardsFit(c.player, []data.QuestItem{{ID: somethingInTheWaterBottle, Count: 1}}) && c.s.addItem(c.player, somethingInTheWaterBottle, 1) && c.customQuestProgress(somethingInTheWaterQuestID, setQuestVar(quest.Vars, 0, 1), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		case 10002:
			if variable == 3 && c.customQuestProgress(somethingInTheWaterQuestID, setQuestVar(quest.Vars, 0, 4), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case somethingInTheWaterJumentis:
		switch dialogID {
		case 25:
			if variable == 2 {
				c.send(dialogWindow(o.id, 1352, somethingInTheWaterQuestID))
				return true
			}
		case 10001:
			if variable == 2 && c.s.countItems(c.player, somethingInTheWaterFullBottle) > 0 && c.customQuestProgress(somethingInTheWaterQuestID, setQuestVar(quest.Vars, 0, 3), "") {
				c.s.removeItemsByID(c.player, somethingInTheWaterFullBottle, 1)
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	}
	return false
}

func (c *conn) somethingInTheWaterItemUse(item *store.Item) {
	if c == nil || c.player == nil || item == nil || item.ItemID != somethingInTheWaterBottle || c.player.cubeItem(item.UniqueID) != item {
		return
	}
	p := c.player
	quest := p.quest(somethingInTheWaterQuestID)
	if p.zone == nil || p.zone.Name != somethingInTheWaterZone || quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 1 {
		return
	}
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 3000, 0, 0), true)
	c.s.later(3*time.Second, func() {
		current := p.quest(somethingInTheWaterQuestID)
		if p.conn != c || p.cubeItem(item.UniqueID) != item || p.zone == nil || p.zone.Name != somethingInTheWaterZone || current == nil || current.Status != "START" || questVar(current.Vars, 0) != 1 {
			return
		}
		if !c.s.questRewardsFit(p, []data.QuestItem{{ID: somethingInTheWaterFullBottle, Count: 1}}) {
			c.send(systemMessage(msgInventoryFull))
			return
		}
		p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0), true)
		c.s.removeItemsByID(p, somethingInTheWaterBottle, 1)
		if !c.s.addItem(p, somethingInTheWaterFullBottle, 1) {
			return
		}
		c.customQuestProgress(somethingInTheWaterQuestID, setQuestVar(current.Vars, 0, 2), "")
	})
}

func (c *conn) somethingInTheWaterKill(npcID int32) bool {
	if c == nil || c.player == nil || npcID != 210946 && npcID != 210947 {
		return false
	}
	quest := c.player.quest(somethingInTheWaterQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 4 {
		return false
	}
	field := 1
	if npcID == 210947 {
		field = 2
	}
	count := questVar(quest.Vars, field)
	if count >= 3 {
		return false
	}
	nextVariables := setQuestVar(quest.Vars, field, count+1)
	status := ""
	if field == 1 && count+1 == 3 && questVar(nextVariables, 2) == 3 || field == 2 && count+1 == 3 && questVar(nextVariables, 1) == 3 {
		status = "REWARD"
	}
	return c.customQuestProgress(somethingInTheWaterQuestID, nextVariables, status)
}
