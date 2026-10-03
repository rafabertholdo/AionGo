package game

import (
	"time"

	"aionlightning/game/data"
)

func (c *conn) dangerousCropDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if script == nil || script.ID != dangerousCropQuestID || o == nil || o.npc == nil {
		return false
	}
	quest := c.player.quest(dangerousCropQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID == dangerousCropReportNPCID {
			return c.finishAltgardStartupQuest(o, script, dangerousCropQuestID, dialogID)
		}
		return false
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch o.npc.ID {
	case dangerousCropReportNPCID:
		switch dialogID {
		case 25:
			pages := map[int32]uint16{0: 1011, 2: 1352, 8: 1693, 9: 2034}
			if page, ok := pages[variable]; ok {
				c.send(dialogWindow(o.id, page, dangerousCropQuestID))
				return true
			}
		case 1012:
			c.send(playMovie(61))
			return false // Java sends the movie and lets the framework echo page 1012.
		case 10000, 10001, 10002:
			if variable == 0 || variable == 2 || variable == 8 {
				if variable == 2 {
					item := []data.QuestItem{{ID: dangerousCropFieldItemID, Count: 1}}
					if !c.s.questRewardsFit(c.player, item) || !c.s.addItem(c.player, dangerousCropFieldItemID, 1) {
						return true
					}
				}
				if variable == 8 {
					c.s.removeItemsByID(c.player, dangerousCropFieldItemID, c.s.countItems(c.player, dangerousCropFieldItemID))
				}
				if !c.beginAltgardStartupQuest(dangerousCropQuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		case 33:
			if variable == 9 {
				template := c.s.data.Quests[dangerousCropQuestID]
				if !c.s.hasQuestItems(c.player, template) {
					c.send(dialogWindow(o.id, 2120, dangerousCropQuestID))
					return true
				}
				for _, item := range template.CollectItems {
					c.s.removeItemsByID(c.player, item.ID, item.Count)
				}
				if !c.beginAltgardStartupQuest(dangerousCropQuestID, quest.Vars, "REWARD") {
					return false
				}
				c.send(dialogWindow(o.id, 5, dangerousCropQuestID))
				return true
			}
		}
	case dangerousCropFieldNPCID:
		if dialogID == -1 && variable >= 3 && variable < 6 && o.useTask == nil && !o.dead {
			p := c.player
			c.send(useObject(p.ID, o.id, 1))
			p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
			o.useTask = c.s.later(3*time.Second, func() {
				o.useTask = nil
				current := p.quest(dangerousCropQuestID)
				if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead || p.targetID != o.id || current == nil || current.Status != "START" || questVar(current.Vars, 0) != variable {
					return
				}
				c.send(useObject(p.ID, o.id, 0))
				p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
				next := variable + 1
				if variable == 5 {
					next = 8
				}
				c.beginAltgardStartupQuest(dangerousCropQuestID, setQuestVar(current.Vars, 0, next), "")
			})
			return true
		}
	}
	return false
}

func (c *conn) dangerousCropEnterZone(zoneName string) bool {
	if c == nil || c.player == nil || c.player.WorldID != altgardDutiesMapID || zoneName != dangerousCropZone {
		return false
	}
	quest := c.player.quest(dangerousCropQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 1 {
		return false
	}
	return c.beginAltgardStartupQuest(dangerousCropQuestID, setQuestVar(quest.Vars, 0, 2), "")
}
