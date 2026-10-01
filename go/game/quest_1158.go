package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	villageSealFoundQuestID int32 = 1158
	villageSealFoundStart   int32 = 798003
	villageSealFoundSearch  int32 = 700003
	villageSealFoundEnd     int32 = 203128
	villageSealFoundItemID  int32 = 182200502
)

// villageSealFoundDialog handles the quest offer, the search near the tree,
// the seal handoff, and reward selection with Ganter.
func (c *conn) villageSealFoundDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != villageSealFoundQuestID {
		return false
	}
	quest := c.player.quest(villageSealFoundQuestID)
	if (quest == nil || quest.Status == "NONE") && o.npc.ID == villageSealFoundStart {
		return c.customQuestStart(o, script, uint16(dialogID), 0)
	}
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != villageSealFoundEnd {
			return false
		}
		switch {
		case dialogID == -1:
			c.send(dialogWindow(o.id, 2375, villageSealFoundQuestID))
		case dialogID == 1009:
			c.send(dialogWindow(o.id, 5, villageSealFoundQuestID))
		case dialogID >= 8 && dialogID <= 11:
			c.finishQuestReward(script, o.id, uint16(dialogID), 0)
			if quest.Status == "COMPLETE" {
				c.s.removeItemsByID(c.player, villageSealFoundItemID, c.s.countItems(c.player, villageSealFoundItemID))
			}
		default:
			return false
		}
		return true
	}
	if quest.Status != "START" || o.npc.ID != villageSealFoundSearch || questVar(quest.Vars, 0) != 0 {
		return false
	}
	switch dialogID {
	case 25:
		c.send(dialogWindow(o.id, 1352, villageSealFoundQuestID))
		return true
	case 10000:
		if c.s.countItems(c.player, villageSealFoundItemID) == 0 && !c.s.questRewardsFit(c.player, []data.QuestItem{{ID: villageSealFoundItemID, Count: 1}}) {
			c.send(systemMessage(msgInventoryFull))
			return true
		}
		if !c.customQuestProgress(villageSealFoundQuestID, setQuestVar(quest.Vars, 0, 1), "REWARD") {
			return false
		}
		if c.s.countItems(c.player, villageSealFoundItemID) == 0 {
			c.s.addItem(c.player, villageSealFoundItemID, 1)
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	case 1353:
		return c.villageSealFoundSearchUse(o, quest)
	default:
		return false
	}
}

func (c *conn) villageSealFoundSearchUse(o *object, quest *store.Quest) bool {
	if o.useTask != nil || o.dead {
		return false
	}
	p := c.player
	variable := questVar(quest.Vars, 0)
	c.send(dialogWindow(0, 0, 0))
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		current := p.quest(villageSealFoundQuestID)
		if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead || current == nil || current.Status != "START" || questVar(current.Vars, 0) != variable {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		c.send(dialogWindow(o.id, 1353, villageSealFoundQuestID))
	})
	return true
}
