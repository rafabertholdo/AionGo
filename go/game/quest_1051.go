package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	ruinsOfRoahQuestID     int32 = 1051
	ruinsOfRoahStartNPC    int32 = 204501
	ruinsOfRoahSecondNPC   int32 = 204582
	ruinsOfRoahThirdNPC    int32 = 203882
	ruinsOfRoahEndNPC      int32 = 278503
	ruinsOfRoahTablet      int32 = 700217
	ruinsOfRoahStonePlate  int32 = 700303
	ruinsOfRoahArtifact    int32 = 182201601
	ruinsOfRoahCollectible int32 = 182201602
)

func (c *conn) ruinsOfRoahLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(ruinsOfRoahQuestID)
	prerequisite := c.player.quest(1500)
	template := c.s.data.Quests[ruinsOfRoahQuestID]
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking The Ruins of Roah", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) ruinsOfRoahDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != ruinsOfRoahQuestID {
		return false
	}
	quest := c.player.quest(ruinsOfRoahQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != ruinsOfRoahStartNPC {
			return false
		}
		switch dialogID {
		case -1, 1009:
			c.send(dialogWindow(o.id, 5, ruinsOfRoahQuestID))
			return true
		default:
			if dialogID >= 8 && dialogID <= 11 || dialogID == 17 {
				c.finishQuestReward(script, o.id, uint16(dialogID), 0)
				return true
			}
		}
		return false
	}
	if quest.Status != "START" {
		return false
	}
	advance := func(nextVariable int32) bool {
		if !c.customQuestProgress(ruinsOfRoahQuestID, setQuestVar(quest.Vars, 0, nextVariable), "") {
			return false
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	}
	switch o.npc.ID {
	case ruinsOfRoahStartNPC:
		switch dialogID {
		case 25:
			switch variable {
			case 0:
				c.send(dialogWindow(o.id, 1011, ruinsOfRoahQuestID))
				return true
			case 4:
				c.send(dialogWindow(o.id, 2375, ruinsOfRoahQuestID))
				return true
			}
		case 10000:
			if variable == 0 {
				return advance(1)
			}
			// Java falls through from case 10000 into case 10004 when var is four.
			if variable == 4 {
				return advance(5)
			}
		case 10004:
			if variable == 4 {
				return advance(5)
			}
		}
	case ruinsOfRoahSecondNPC:
		switch dialogID {
		case 25:
			switch variable {
			case 1:
				c.send(dialogWindow(o.id, 1352, ruinsOfRoahQuestID))
				return true
			case 3:
				c.send(dialogWindow(o.id, 2034, ruinsOfRoahQuestID))
				return true
			}
		case 10001:
			if variable == 1 {
				return advance(2)
			}
			// Java falls through from case 10001 into case 10003 at var three.
			if variable == 3 {
				if !c.customQuestProgress(ruinsOfRoahQuestID, setQuestVar(quest.Vars, 0, 4), "") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				c.s.removeItemsByID(c.player, ruinsOfRoahArtifact, 1)
				return true
			}
		case 10003:
			if variable == 3 {
				if !c.customQuestProgress(ruinsOfRoahQuestID, setQuestVar(quest.Vars, 0, 4), "") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				c.s.removeItemsByID(c.player, ruinsOfRoahArtifact, 1)
				return true
			}
		}
	case ruinsOfRoahThirdNPC:
		switch dialogID {
		case 25:
			if variable == 5 {
				c.send(dialogWindow(o.id, 2716, ruinsOfRoahQuestID))
				return true
			}
		case 10005:
			if variable == 5 {
				return advance(6)
			}
		}
	case ruinsOfRoahEndNPC:
		switch dialogID {
		case 25:
			switch variable {
			case 6:
				c.send(dialogWindow(o.id, 3057, ruinsOfRoahQuestID))
				return true
			case 7:
				c.send(dialogWindow(o.id, 3398, ruinsOfRoahQuestID))
				return true
			}
			return c.ruinsOfRoahCollectItem(o, quest)
		case 33:
			return c.ruinsOfRoahCollectItem(o, quest)
		case 10006:
			if variable == 6 {
				return advance(7)
			}
			// Java falls through from case 10006 into case 10007 when var is seven.
			if variable == 7 {
				return advance(8)
			}
		case 10007:
			if variable == 7 {
				return advance(8)
			}
		}
	case ruinsOfRoahTablet:
		if variable == 2 && dialogID == -1 {
			return c.ruinsOfRoahUseTablet(o)
		}
		if variable == 2 && dialogID == 10002 {
			c.send(dialogWindow(o.id, 0, 0))
			if !c.s.questRewardsFit(c.player, []data.QuestItem{{ID: ruinsOfRoahArtifact, Count: 1}}) || !c.s.addItem(c.player, ruinsOfRoahArtifact, 1) {
				c.send(systemMessage(msgInventoryFull))
				return true
			}
			return c.customQuestProgress(ruinsOfRoahQuestID, setQuestVar(quest.Vars, 0, 3), "")
		}
	case ruinsOfRoahStonePlate:
		if variable == 7 && dialogID == -1 {
			return c.ruinsOfRoahUseStonePlate(o)
		}
	}
	return false
}

func (c *conn) ruinsOfRoahCollectItem(o *object, quest *store.Quest) bool {
	template := c.s.data.Quests[ruinsOfRoahQuestID]
	if quest == nil || template == nil {
		return false
	}
	if !c.s.hasQuestItems(c.player, template) {
		c.send(dialogWindow(o.id, 10001, ruinsOfRoahQuestID))
		return true
	}
	for _, item := range template.CollectItems {
		c.s.removeItemsByID(c.player, item.ID, item.Count)
	}
	if !c.customQuestProgress(ruinsOfRoahQuestID, setQuestVar(quest.Vars, 0, questVar(quest.Vars, 0)+1), "REWARD") {
		return false
	}
	c.send(dialogWindow(o.id, 10000, ruinsOfRoahQuestID))
	return true
}

func (c *conn) ruinsOfRoahUseTablet(o *object) bool {
	if o == nil || o.useTask != nil {
		return false
	}
	p := c.player
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		c.send(dialogWindow(o.id, 1693, ruinsOfRoahQuestID))
	})
	return false
}

func (c *conn) ruinsOfRoahUseStonePlate(o *object) bool {
	if o == nil || o.useTask != nil {
		return false
	}
	p := c.player
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		if !c.s.questRewardsFit(p, []data.QuestItem{{ID: ruinsOfRoahCollectible, Count: 1}}) {
			c.send(systemMessage(msgInventoryFull))
			return
		}
		c.s.addItem(p, ruinsOfRoahCollectible, 1)
	})
	return false
}
