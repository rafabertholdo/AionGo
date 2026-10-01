package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	rulersDutyQuestID int32 = 1032
	rulersDutyItemID  int32 = 182201001
	phomonaNPCID      int32 = 203932
	demroNPCID        int32 = 730020
	lodasNPCID        int32 = 730019
	seauKerubienNPCID int32 = 700157
)

func (c *conn) rulersDutyLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(rulersDutyQuestID)
	template := c.s.data.Quests[rulersDutyQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking A Ruler's Duty", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) rulersDutyDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != rulersDutyQuestID {
		return false
	}
	quest := c.player.quest(rulersDutyQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != phomonaNPCID {
			return false
		}
		switch dialogID {
		case -1:
			c.send(dialogWindow(o.id, 2716, rulersDutyQuestID))
			return true
		case 1009:
			c.send(dialogWindow(o.id, 5, rulersDutyQuestID))
			return true
		default:
			if dialogID >= 8 && dialogID <= 11 {
				c.finishQuestReward(script, o.id, uint16(dialogID), 0)
				return true
			}
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	switch o.npc.ID {
	case phomonaNPCID:
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1011, rulersDutyQuestID))
			return true
		}
		if dialogID == 10000 && c.customQuestProgress(rulersDutyQuestID, setQuestVar(quest.Vars, 0, 1), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case demroNPCID:
		switch {
		case dialogID == 25 && variable == 1:
			c.send(dialogWindow(o.id, 1352, rulersDutyQuestID))
			return true
		case dialogID == 10001 && variable == 1 && c.customQuestProgress(rulersDutyQuestID, setQuestVar(quest.Vars, 0, 2), ""):
			c.send(dialogWindow(o.id, 10, 0))
			return true
		case dialogID == 25 && variable == 5:
			c.send(dialogWindow(o.id, 2375, rulersDutyQuestID))
			return true
		case dialogID == 10004 && variable == 5 && c.customQuestProgress(rulersDutyQuestID, quest.Vars, "REWARD"):
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case lodasNPCID:
		switch {
		case dialogID == 25 && variable == 2:
			c.send(dialogWindow(o.id, 1693, rulersDutyQuestID))
			return true
		case dialogID == 10002 && variable == 2:
			c.s.removeItemsByID(c.player, rulersDutyItemID, 1)
			if c.customQuestProgress(rulersDutyQuestID, setQuestVar(quest.Vars, 0, 3), "") {
				c.send(ascensionMovie(49))
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		case dialogID == 25 && variable == 4:
			c.send(dialogWindow(o.id, 2034, rulersDutyQuestID))
			return true
		case dialogID == 10003 && variable == 4 && c.customQuestProgress(rulersDutyQuestID, setQuestVar(quest.Vars, 0, 5), ""):
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case seauKerubienNPCID:
		if dialogID == -1 && variable == 3 {
			return c.rulersDutyCollectFromKerubien(o)
		}
	}
	return false
}

func (c *conn) rulersDutyCollectFromKerubien(o *object) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.useTask != nil || c.player.targetID != o.id {
		return false
	}
	p := c.player
	quest := p.quest(rulersDutyQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 3 {
		return false
	}
	if c.s.countItems(p, rulersDutyItemID) == 0 {
		if !c.s.questRewardsFit(p, []data.QuestItem{{ID: rulersDutyItemID, Count: 1}}) || !c.s.addItem(p, rulersDutyItemID, 1) {
			c.send(systemMessage(msgInventoryFull))
			return false
		}
	}
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead || p.targetID != o.id {
			return
		}
		current := p.quest(rulersDutyQuestID)
		if current == nil || current.Status != "START" || questVar(current.Vars, 0) != 3 {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
	})
	c.dialogNotHandled()
	return false
}

func (c *conn) rulersDutyItemUse(item *store.Item) {
	if c == nil || c.player == nil || item == nil || item.ItemID != rulersDutyItemID || c.player.cubeItem(item.UniqueID) != item {
		return
	}
	p := c.player
	quest := p.quest(rulersDutyQuestID)
	if p.zone == nil || p.zone.Name != "PUTRID_MIRE" || quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 3 {
		return
	}
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 3000, 0, 0), true)
	c.s.later(3*time.Second, func() {
		current := p.quest(rulersDutyQuestID)
		if p.conn != c || p.cubeItem(item.UniqueID) != item || p.zone == nil || p.zone.Name != "PUTRID_MIRE" || current == nil || current.Status != "START" || questVar(current.Vars, 0) != 3 {
			return
		}
		p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0), true)
		c.customQuestProgress(rulersDutyQuestID, setQuestVar(current.Vars, 0, 4), "")
	})
}
