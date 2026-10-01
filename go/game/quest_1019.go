package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	flyingReconnaissanceQuestID int32 = 1019
	flyingReconnaissanceItemID  int32 = 182200023
	flyingReconnaissancePassID  int32 = 182200505
	flyingReconnaissanceNPC     int32 = 203146
	flyingReconnaissanceTursin  int32 = 203098
	flyingReconnaissanceGuide   int32 = 203147
	flyingReconnaissanceScout   int32 = 210158
	flyingReconnaissanceTotem   int32 = 700037
	flyingReconnaissanceBoss    int32 = 210697
)

// flyingReconnaissanceLevelUp ports the Java LOCKED-to-START level-up event.
func (c *conn) flyingReconnaissanceLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	template := c.s.data.Quests[flyingReconnaissanceQuestID]
	quest := c.player.quest(flyingReconnaissanceQuestID)
	if template == nil || quest == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Flying Reconnaissance", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

// flyingReconnaissanceDialog handles the three NPC conversations, the totem
// use action, and the final selectable reward dialog.
func (c *conn) flyingReconnaissanceDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != flyingReconnaissanceQuestID {
		return false
	}
	quest := c.player.quest(flyingReconnaissanceQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID == flyingReconnaissanceTursin && dialogID == -1 {
			c.send(dialogWindow(o.id, 2034, flyingReconnaissanceQuestID))
			return true
		}
		if o.npc.ID != flyingReconnaissanceTursin {
			return false
		}
		switch {
		case dialogID == 1009:
			c.send(dialogWindow(o.id, 5, flyingReconnaissanceQuestID))
		case dialogID >= 8 && dialogID <= 11:
			c.finishQuestReward(script, o.id, uint16(dialogID), 0)
		default:
			return false
		}
		return true
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch o.npc.ID {
	case flyingReconnaissanceNPC:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, flyingReconnaissanceQuestID))
				return true
			}
		case 10000:
			if variable != 0 {
				return false
			}
			if !c.s.questRewardsFit(c.player, []data.QuestItem{{ID: flyingReconnaissancePassID, Count: 1}}) {
				c.send(systemMessage(msgInventoryFull))
				return true
			}
			if c.customQuestProgress(flyingReconnaissanceQuestID, setQuestVar(quest.Vars, 0, 1), "") {
				c.s.addItem(c.player, flyingReconnaissancePassID, 1)
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case flyingReconnaissanceTursin:
		switch dialogID {
		case 25:
			if variable == 2 {
				c.send(dialogWindow(o.id, 1352, flyingReconnaissanceQuestID))
				return true
			}
		case 10001:
			if variable == 2 && c.customQuestProgress(flyingReconnaissanceQuestID, setQuestVar(quest.Vars, 0, 3), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case flyingReconnaissanceGuide:
		switch dialogID {
		case 25:
			if variable == 3 {
				c.send(dialogWindow(o.id, 1438, flyingReconnaissanceQuestID))
				return true
			}
			if variable == 5 {
				c.send(dialogWindow(o.id, 1693, flyingReconnaissanceQuestID))
				return true
			}
		case 10002, 10003:
			if variable != 3 && variable != 5 {
				return false
			}
			if variable == 5 && !c.s.questRewardsFit(c.player, []data.QuestItem{{ID: flyingReconnaissanceItemID, Count: 1}}) {
				c.send(systemMessage(msgInventoryFull))
				return true
			}
			if !c.customQuestProgress(flyingReconnaissanceQuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
				return false
			}
			if variable == 5 {
				c.s.addItem(c.player, flyingReconnaissanceItemID, 1)
			}
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case flyingReconnaissanceTotem:
		if dialogID == -1 && variable >= 6 && variable < 9 {
			return c.flyingReconnaissanceUseTotem(o, quest)
		}
	}
	return false
}

// flyingReconnaissanceEnterZone handles the outpost report and its entrance
// movie. The source registers the outpost zone but compares the event to the
// entrance subzone in a nested condition; accept either zone to preserve the
// intended movie behavior while advancing only on the outpost event.
func (c *conn) flyingReconnaissanceEnterZone(zoneName string) bool {
	if c == nil || c.player == nil || c.player.WorldID != 210030000 {
		return false
	}
	quest := c.player.quest(flyingReconnaissanceQuestID)
	if quest == nil {
		return false
	}
	if zoneName == "TURSIN_OUTPOST_ENTRANCE" {
		c.send(playMovie(18))
		return true
	}
	if zoneName != "TURSIN_OUTPOST" {
		return false
	}
	if quest.Status != "START" || questVar(quest.Vars, 0) != 1 {
		return false
	}
	return c.customQuestProgress(flyingReconnaissanceQuestID, setQuestVar(quest.Vars, 0, 2), "")
}

// flyingReconnaissanceAttack ports the attack event on the named scout. The
// event only kills it during the scripted attack near its marked position.
func (c *conn) flyingReconnaissanceAttack(o *object) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.npc.ID != flyingReconnaissanceScout || o.dead {
		return false
	}
	quest := c.player.quest(flyingReconnaissanceQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 4 || distance3D(o.x, o.y, o.z, 1552.74, 1160.36, 114) >= 6 {
		return false
	}
	c.send(playMovie(13))
	o.dead, o.hp = true, 0
	if o.ai != nil {
		c.s.npcDied(o, nil)
	}
	return c.customQuestProgress(flyingReconnaissanceQuestID, setQuestVar(quest.Vars, 0, 5), "")
}

// flyingReconnaissanceKill handles the final named Tursin kill.
func (c *conn) flyingReconnaissanceKill(npcID int32) bool {
	if c == nil || c.player == nil || npcID != flyingReconnaissanceBoss {
		return false
	}
	quest := c.player.quest(flyingReconnaissanceQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 10 {
		return false
	}
	return c.customQuestProgress(flyingReconnaissanceQuestID, quest.Vars, "REWARD")
}

// flyingReconnaissanceItemUse handles the delayed totem item activation.
func (c *conn) flyingReconnaissanceItemUse(item *store.Item) bool {
	if c == nil || c.player == nil || item == nil || item.ItemID != flyingReconnaissanceItemID || c.player.cubeItem(item.UniqueID) != item {
		return false
	}
	p := c.player
	quest := p.quest(flyingReconnaissanceQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 9 || p.zone == nil || p.zone.Name != "TURSIN_TOTEM_POLE" {
		return false
	}
	p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 3000, 0, 0), true)
	c.s.later(3*time.Second, func() {
		current := p.quest(flyingReconnaissanceQuestID)
		if p.conn != c || p.cubeItem(item.UniqueID) != item || p.zone == nil || p.zone.Name != "TURSIN_TOTEM_POLE" || current == nil || current.Status != "START" || questVar(current.Vars, 0) != 9 {
			return
		}
		p.broadcast(itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0), true)
		if c.customQuestProgress(flyingReconnaissanceQuestID, setQuestVar(current.Vars, 0, 10), "") {
			c.s.removeItemsByID(p, item.ItemID, 1)
		}
	})
	return true
}

func (c *conn) flyingReconnaissanceUseTotem(o *object, quest *store.Quest) bool {
	p := c.player
	if o.useTask != nil || o.dead || p.targetID != o.id {
		return false
	}
	variable := questVar(quest.Vars, 0)
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		current := p.quest(flyingReconnaissanceQuestID)
		if p.conn != c || !p.spawned || p.seen[o.id] != o || p.targetID != o.id || o.dead || current == nil || current.Status != "START" || questVar(current.Vars, 0) != variable {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
		o.dead, o.hp = true, 0
		if o.ai != nil {
			c.s.npcDied(o, nil)
		}
		c.customQuestProgress(flyingReconnaissanceQuestID, setQuestVar(current.Vars, 0, variable+1), "")
	})
	return true
}
