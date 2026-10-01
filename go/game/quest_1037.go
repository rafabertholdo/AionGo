package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	secretsOfTempleQuestID    int32 = 1037
	secretsOfTempleGuideID    int32 = 203965
	secretsOfTempleScribeID   int32 = 203967
	secretsOfTempleItemOne    int32 = 182201020
	secretsOfTempleItemTwo    int32 = 182201021
	secretsOfTempleItemThree  int32 = 182201022
	secretsOfTempleItemFour   int32 = 182201023
	secretsOfTempleRitualItem int32 = 182201027
)

var secretsOfTempleItems = [...]int32{
	secretsOfTempleItemOne,
	secretsOfTempleItemTwo,
	secretsOfTempleItemThree,
	secretsOfTempleItemFour,
}

func (c *conn) secretsOfTempleLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(secretsOfTempleQuestID)
	template := c.s.data.Quests[secretsOfTempleQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Secrets of the Temple", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) secretsOfTempleDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != secretsOfTempleQuestID {
		return false
	}
	quest := c.player.quest(secretsOfTempleQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != secretsOfTempleGuideID {
			return false
		}
		switch dialogID {
		case -1, 1009:
			c.send(dialogWindow(o.id, 5, secretsOfTempleQuestID))
			return true
		default:
			if dialogID >= 8 && dialogID <= 11 || dialogID == 17 {
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
	case secretsOfTempleGuideID:
		if dialogID == 25 && variable == 0 {
			c.send(dialogWindow(o.id, 1011, secretsOfTempleQuestID))
			return true
		}
		if dialogID == 10000 && variable == 0 && c.customQuestProgress(secretsOfTempleQuestID, setQuestVar(quest.Vars, 0, 1), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case secretsOfTempleScribeID:
		switch dialogID {
		case 25:
			switch variable {
			case 1:
				c.send(dialogWindow(o.id, 1352, secretsOfTempleQuestID))
				return true
			case 2:
				c.send(dialogWindow(o.id, 1693, secretsOfTempleQuestID))
				return true
			default:
				return c.secretsOfTempleCollectItems(o)
			}
		case 1694:
			return c.secretsOfTempleCollectItems(o)
		case 10001:
			if variable == 1 && c.customQuestProgress(secretsOfTempleQuestID, setQuestVar(quest.Vars, 0, 2), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
			fallthrough
		case 10002:
			if variable == 2 {
				if !c.s.questRewardsFit(c.player, []data.QuestItem{{ID: secretsOfTempleRitualItem, Count: 1}}) {
					c.send(systemMessage(msgInventoryFull))
					return false
				}
				if c.customQuestProgress(secretsOfTempleQuestID, setQuestVar(quest.Vars, 0, 3), "") {
					c.send(dialogWindow(o.id, 10, 0))
					c.s.addItem(c.player, secretsOfTempleRitualItem, 1)
					return true
				}
			}
		}
	default:
		if dialogID == -1 {
			if nextVariable, ok := secretsOfTempleObjectStage(o.npc.ID, variable); ok {
				return c.secretsOfTempleUseRitualObject(o, quest, nextVariable)
			}
		}
	}
	return false
}

func (c *conn) secretsOfTempleCollectItems(o *object) bool {
	template := c.s.data.Quests[secretsOfTempleQuestID]
	if questVar(c.player.quest(secretsOfTempleQuestID).Vars, 0) == 2 && c.s.hasQuestItems(c.player, template) {
		for _, itemID := range secretsOfTempleItems {
			c.s.removeItemsByID(c.player, itemID, 1)
		}
		c.send(dialogWindow(o.id, 1694, secretsOfTempleQuestID))
		return true
	}
	c.send(dialogWindow(o.id, 1779, secretsOfTempleQuestID))
	return true
}

func secretsOfTempleObjectStage(npcID, variable int32) (int32, bool) {
	switch {
	case npcID == 700151 && variable == 3,
		npcID == 700154 && variable == 4,
		npcID == 700150 && variable == 5,
		npcID == 700153 && variable == 6,
		npcID == 700152 && variable == 7:
		return variable + 1, true
	default:
		return 0, false
	}
}

func (c *conn) secretsOfTempleUseRitualObject(o *object, quest *store.Quest, nextVariable int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.useTask != nil || c.s.countItems(c.player, secretsOfTempleRitualItem) != 1 {
		return false
	}
	p := c.player
	if quest == nil || quest.Status != "START" || nextVariable != questVar(quest.Vars, 0)+1 {
		return false
	}
	c.send(ascensionMovie(33))
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	if !c.customQuestProgress(secretsOfTempleQuestID, setQuestVar(quest.Vars, 0, nextVariable), "REWARD") {
		return false
	}
	c.s.removeItemsByID(p, secretsOfTempleRitualItem, 1)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
	})
	return false
}
