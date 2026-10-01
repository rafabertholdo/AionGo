package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	fragmentOfMemory2QuestID         int32  = 1076
	fragmentOfMemory2StartNPCID      int32  = 278500
	fragmentOfMemory2ReportNPCID     int32  = 203834
	fragmentOfMemory2CollectionNPCID int32  = 203786
	fragmentOfMemory2EndNPCID        int32  = 203754
	fragmentOfMemory2RewardNPCID     int32  = 203704
	fragmentOfMemory2DropNPCID       int32  = 255160
	fragmentOfMemory2QuestItemID     int32  = 182202006
	fragmentOfMemory2MonsterFragment int32  = 182202003
	fragmentOfMemory2ItemMovieID     uint16 = 170
)

func (c *conn) fragmentOfMemory2LevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(fragmentOfMemory2QuestID)
	prerequisite := c.player.quest(1701)
	template := c.s.data.Quests[fragmentOfMemory2QuestID]
	if quest == nil || prerequisite == nil || prerequisite.Status != "COMPLETE" || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Fragment of Memory 2", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) fragmentOfMemory2Dialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != fragmentOfMemory2QuestID {
		return false
	}
	quest := c.player.quest(fragmentOfMemory2QuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != fragmentOfMemory2RewardNPCID {
			return false
		}
		switch dialogID {
		case -1:
			c.send(dialogWindow(o.id, 10002, fragmentOfMemory2QuestID))
			return true
		case 1009:
			c.send(dialogWindow(o.id, 5, fragmentOfMemory2QuestID))
			return true
		default:
			if dialogID >= 8 && dialogID <= 17 {
				c.finishQuest(script, o.id, uint16(dialogID))
				return true
			}
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch o.npc.ID {
	case fragmentOfMemory2StartNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, fragmentOfMemory2QuestID))
				return true
			}
			fallthrough
		case 10000:
			if variable == 0 {
				return c.fragmentOfMemory2Advance(o, quest, 1)
			}
		}
	case fragmentOfMemory2ReportNPCID:
		switch dialogID {
		case 25:
			switch variable {
			case 1:
				c.send(dialogWindow(o.id, 1352, fragmentOfMemory2QuestID))
				return true
			case 3:
				c.send(dialogWindow(o.id, 2034, fragmentOfMemory2QuestID))
				return true
			case 5:
				c.send(dialogWindow(o.id, 2716, fragmentOfMemory2QuestID))
				return true
			}
			// Java falls through from unmatched dialog 25 to case 1353.
			fallthrough
		case 1353:
			c.send(playMovie(102))
			return false
		case 10001:
			if variable == 1 {
				return c.fragmentOfMemory2Advance(o, quest, 2)
			}
			// Java falls through to 10003 when var is not one.
			fallthrough
		case 10003:
			if variable == 3 {
				return c.fragmentOfMemory2Advance(o, quest, 4)
			}
			// Java falls through to 10005 when var is not three.
			fallthrough
		case 10005:
			if variable == 5 {
				if !c.customQuestProgress(fragmentOfMemory2QuestID, setQuestVar(quest.Vars, 0, 6), "") {
					return false
				}
				c.s.removeItemsByID(c.player, fragmentOfMemory2QuestItemID, c.s.countItems(c.player, fragmentOfMemory2QuestItemID))
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case fragmentOfMemory2CollectionNPCID:
		switch dialogID {
		case 25:
			if variable == 2 {
				c.send(dialogWindow(o.id, 1693, fragmentOfMemory2QuestID))
				return true
			}
			fallthrough
		case 33:
			return c.fragmentOfMemory2Collect(o, quest, variable)
		}
	case fragmentOfMemory2EndNPCID:
		switch dialogID {
		case 25:
			if variable == 6 {
				c.send(dialogWindow(o.id, 3057, fragmentOfMemory2QuestID))
				return true
			}
			fallthrough
		case 10255:
			if variable == 6 {
				if !c.customQuestProgress(fragmentOfMemory2QuestID, quest.Vars, "REWARD") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	}
	return false
}

func (c *conn) fragmentOfMemory2Advance(o *object, quest *store.Quest, variable int32) bool {
	if !c.customQuestProgress(fragmentOfMemory2QuestID, setQuestVar(quest.Vars, 0, variable), "") {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	return true
}

func (c *conn) fragmentOfMemory2Collect(o *object, quest *store.Quest, variable int32) bool {
	template := c.s.data.Quests[fragmentOfMemory2QuestID]
	if quest == nil || template == nil || !c.s.hasQuestItems(c.player, template) {
		c.send(dialogWindow(o.id, 10001, fragmentOfMemory2QuestID))
		return true
	}
	for _, item := range template.CollectItems {
		c.s.removeItemsByID(c.player, item.ID, item.Count)
	}
	if !c.customQuestProgress(fragmentOfMemory2QuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
		return false
	}
	if c.s.questRewardsFit(c.player, []data.QuestItem{{ID: fragmentOfMemory2QuestItemID, Count: 1}}) {
		c.s.addItem(c.player, fragmentOfMemory2QuestItemID, 1)
	}
	c.send(dialogWindow(o.id, 10000, fragmentOfMemory2QuestID))
	return true
}

func (c *conn) fragmentOfMemory2ItemUse(item *store.Item) bool {
	if c == nil || c.player == nil || item == nil || item.ItemID != fragmentOfMemory2QuestItemID || c.player.cubeItem(item.UniqueID) != item {
		return false
	}
	p := c.player
	quest := p.quest(fragmentOfMemory2QuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 4 {
		return false
	}
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 1000, 0, 0), true)
	c.s.later(time.Second, func() {
		current := p.quest(fragmentOfMemory2QuestID)
		if p.conn != c || p.cubeItem(item.UniqueID) != item || current == nil || current.Status != "START" || questVar(current.Vars, 0) != 4 {
			return
		}
		p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0), true)
		c.send(playMovie(fragmentOfMemory2ItemMovieID))
		c.customQuestProgress(fragmentOfMemory2QuestID, setQuestVar(current.Vars, 0, 5), "")
	})
	return true
}
