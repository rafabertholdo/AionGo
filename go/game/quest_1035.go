package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	refreshingSpringsQuestID int32 = 1035
	springsGuideNPCID        int32 = 203917
	springGuideTwoNPCID      int32 = 203992
	springObjectOneNPCID     int32 = 700158
	springGuideThreeNPCID    int32 = 203965
	springGuideFourNPCID     int32 = 203968
	springGuideFiveNPCID     int32 = 203987
	springObjectTwoNPCID     int32 = 700160
	springGuideSixNPCID      int32 = 203934
	springObjectThreeNPCID   int32 = 700159
	springWaterItemID        int32 = 182201014
	springProofOneItemID     int32 = 182201024
	springProofTwoItemID     int32 = 182201025
)

func (c *conn) refreshingSpringsLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(refreshingSpringsQuestID)
	template := c.s.data.Quests[refreshingSpringsQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Refreshing the Springs", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) refreshingSpringsDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != refreshingSpringsQuestID {
		return false
	}
	quest := c.player.quest(refreshingSpringsQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != springsGuideNPCID {
			return false
		}
		switch dialogID {
		case -1, 1009:
			c.send(dialogWindow(o.id, 5, refreshingSpringsQuestID))
			return true
		default:
			if dialogID >= 8 && dialogID <= 14 || dialogID == 17 {
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
	case springsGuideNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, refreshingSpringsQuestID))
				return true
			}
			if variable == 4 {
				c.send(dialogWindow(o.id, 1352, refreshingSpringsQuestID))
				return true
			}
		case 10000:
			if variable == 0 && c.customQuestProgress(refreshingSpringsQuestID, setQuestVar(quest.Vars, 0, 1), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
			fallthrough
		case 10001:
			if variable == 4 && c.customQuestProgress(refreshingSpringsQuestID, setQuestVar(quest.Vars, 0, 5), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case springGuideTwoNPCID:
		switch dialogID {
		case 25:
			if variable == 1 {
				c.send(dialogWindow(o.id, 1352, refreshingSpringsQuestID))
				return true
			}
			if variable == 3 {
				c.send(dialogWindow(o.id, 1693, refreshingSpringsQuestID))
				return true
			}
			fallthrough
		case 10001:
			if variable == 1 && c.customQuestProgress(refreshingSpringsQuestID, setQuestVar(quest.Vars, 0, 2), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
			fallthrough
		case 10002:
			if variable == 3 && c.customQuestProgress(refreshingSpringsQuestID, setQuestVar(quest.Vars, 0, 4), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case springObjectOneNPCID:
		if variable == 2 && dialogID == -1 {
			return c.refreshingSpringsUseItemObject(o, springWaterItemID, 2, 3, 0)
		}
	case springGuideThreeNPCID:
		if dialogID == 25 && variable == 4 {
			c.send(dialogWindow(o.id, 2034, refreshingSpringsQuestID))
			return true
		}
		if dialogID == 10003 && variable == 4 && c.customQuestProgress(refreshingSpringsQuestID, setQuestVar(quest.Vars, 0, 5), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case springGuideFourNPCID:
		if dialogID == 25 && variable == 5 {
			c.send(dialogWindow(o.id, 2375, refreshingSpringsQuestID))
			return true
		}
		if dialogID == 10004 && variable == 5 && c.customQuestProgress(refreshingSpringsQuestID, setQuestVar(quest.Vars, 0, 6), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case springGuideFiveNPCID:
		switch {
		case dialogID == 25 && variable == 6:
			c.send(dialogWindow(o.id, 2716, refreshingSpringsQuestID))
			return true
		case dialogID == 25 && variable == 8:
			c.send(dialogWindow(o.id, 3057, refreshingSpringsQuestID))
			return true
		case dialogID == 10005 && variable == 6:
			if c.giveRefreshingSpringsProof(o, quest, springProofOneItemID, 7) {
				return true
			}
		case dialogID == 10006 && variable == 8:
			if c.giveRefreshingSpringsProof(o, quest, springProofTwoItemID, 9) {
				return true
			}
		}
	case springObjectTwoNPCID:
		if variable == 7 && dialogID == -1 {
			return c.refreshingSpringsUseItemObject(o, springProofOneItemID, 7, 8, 31)
		}
	case springGuideSixNPCID:
		if dialogID == 25 && variable == 9 {
			c.send(dialogWindow(o.id, 3398, refreshingSpringsQuestID))
			return true
		}
		if dialogID == 25 && variable == 11 {
			c.send(dialogWindow(o.id, 3739, refreshingSpringsQuestID))
			return true
		}
		if dialogID == 10007 {
			switch variable {
			case 9:
				c.customQuestProgress(refreshingSpringsQuestID, setQuestVar(quest.Vars, 0, 10), "")
				c.send(dialogWindow(o.id, 10, 0))
				return false
			case 11:
				c.send(dialogWindow(o.id, 10, 0))
				return c.customQuestProgress(refreshingSpringsQuestID, quest.Vars, "REWARD")
			}
		}
	case springObjectThreeNPCID:
		if variable == 10 && dialogID == -1 {
			return c.refreshingSpringsUseItemObject(o, springProofTwoItemID, 10, 11, 0)
		}
	}
	return false
}

func (c *conn) giveRefreshingSpringsProof(o *object, quest *store.Quest, itemID int32, nextVariable int32) bool {
	if !c.s.questRewardsFit(c.player, []data.QuestItem{{ID: itemID, Count: 1}}) {
		c.send(systemMessage(msgInventoryFull))
		return false
	}
	if !c.customQuestProgress(refreshingSpringsQuestID, setQuestVar(quest.Vars, 0, nextVariable), "") {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	c.s.addItem(c.player, itemID, 1)
	return true
}

func (c *conn) refreshingSpringsUseItemObject(o *object, itemID int32, variable, nextVariable int32, movieID uint16) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.useTask != nil || c.s.countItems(c.player, itemID) != 1 {
		return false
	}
	p := c.player
	quest := p.quest(refreshingSpringsQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != variable {
		return false
	}
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	if movieID != 0 {
		c.send(ascensionMovie(movieID))
	}
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead {
			return
		}
		current := p.quest(refreshingSpringsQuestID)
		if current == nil || current.Status != "START" || questVar(current.Vars, 0) != variable || c.s.countItems(p, itemID) != 1 {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		c.s.removeItemsByID(p, itemID, 1)
		c.customQuestProgress(refreshingSpringsQuestID, setQuestVar(current.Vars, 0, nextVariable), "")
	})
	return false
}
