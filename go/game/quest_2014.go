package game

import (
	"time"

	"aionlightning/game/data"
)

func (c *conn) scoutItOutDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if script == nil || script.ID != scoutItOutQuestID || o == nil || o.npc == nil {
		return false
	}
	quest := c.player.quest(scoutItOutQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID == scoutItOutReportNPCID {
			return c.finishAltgardStartupQuest(o, script, scoutItOutQuestID, dialogID)
		}
		return false
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch o.npc.ID {
	case scoutItOutStartNPCID:
		switch dialogID {
		case 25:
			if variable == 0 {
				c.send(dialogWindow(o.id, 1011, scoutItOutQuestID))
				return true
			}
			if variable == 1 || variable == 2 {
				page := uint16(1352)
				if c.s.countItems(c.player, scoutItOutEvidenceItemID) == 0 {
					page = 1438
				}
				c.send(dialogWindow(o.id, page, scoutItOutQuestID))
				return true
			}
		case 10000, 10001:
			if variable == 0 || variable == 1 || variable == 2 {
				next := int32(1)
				if variable == 1 || variable == 2 {
					c.s.removeItemsByID(c.player, scoutItOutEvidenceItemID, c.s.countItems(c.player, scoutItOutEvidenceItemID))
					next = 3
				}
				if c.beginAltgardStartupQuest(scoutItOutQuestID, setQuestVar(quest.Vars, 0, next), "") {
					c.send(dialogWindow(o.id, 10, 0))
					return true
				}
			}
		}
	case scoutItOutGraveNPCID:
		if dialogID == -1 && variable == 1 && o.useTask == nil && !o.dead {
			p := c.player
			c.send(useObject(p.ID, o.id, 1))
			p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
			o.useTask = c.s.later(3*time.Second, func() {
				o.useTask = nil
				current := p.quest(scoutItOutQuestID)
				if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead || p.targetID != o.id || current == nil || current.Status != "START" || questVar(current.Vars, 0) != 1 {
					return
				}
				c.send(useObject(p.ID, o.id, 0))
				p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
				item := []data.QuestItem{{ID: scoutItOutEvidenceItemID, Count: 1}}
				if c.s.questRewardsFit(p, item) && c.s.addItem(p, scoutItOutEvidenceItemID, 1) {
					c.beginAltgardStartupQuest(scoutItOutQuestID, setQuestVar(current.Vars, 0, 2), "")
				}
			})
			return true
		}
	case scoutItOutScoutNPCID:
		switch dialogID {
		case 25:
			if variable == 3 {
				c.send(dialogWindow(o.id, 1693, scoutItOutQuestID))
				return true
			}
		case 10002:
			if variable == 3 && c.beginAltgardStartupQuest(scoutItOutQuestID, setQuestVar(quest.Vars, 0, 4), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	}
	return false
}

func (c *conn) scoutItOutKill(npcID int32) bool {
	if c == nil || c.player == nil || npcID != scoutItOutNamedNPCID {
		return false
	}
	quest := c.player.quest(scoutItOutQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 4 {
		return false
	}
	return c.beginAltgardStartupQuest(scoutItOutQuestID, quest.Vars, "REWARD")
}
