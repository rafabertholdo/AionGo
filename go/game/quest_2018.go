package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func (c *conn) impetusiumDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != impetusiumQuestID {
		return false
	}
	quest := c.player.quest(impetusiumQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		if o.npc.ID != impetusiumReportNPCID {
			return false
		}
		return c.finishAltgardStartupQuest(o, script, impetusiumQuestID, dialogID)
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch o.npc.ID {
	case impetusiumReportNPCID:
		switch dialogID {
		case 25:
			pages := map[int32]uint16{0: 1011, 4: 1352, 7: 2034}
			if page, ok := pages[variable]; ok {
				c.send(dialogWindow(o.id, page, impetusiumQuestID))
				return true
			}
		case 1012:
			c.send(playMovie(22))
			return false
		case 10000, 10001:
			if variable == 0 || variable == 4 {
				if c.beginAltgardStartupQuest(impetusiumQuestID, setQuestVar(quest.Vars, 0, variable+1), "") {
					c.send(dialogWindow(o.id, 10, 0))
					return true
				}
			}
			// Java falls through into case 33 at var 7 for these dialog ids.
			if (dialogID == 10000 || dialogID == 10001) && variable == 7 {
				return c.impetusiumCollect(o, quest)
			}
		case 33:
			if variable == 7 {
				return c.impetusiumCollect(o, quest)
			}
		}
	case impetusiumJewelBoxNPCID:
		if variable == 5 {
			// Java's true return lets ActionitemController run its three-second
			// default use animation and quest-drop registration.
			c.useQuestObject(o, script)
			return true
		}
	case impetusiumGraveNPCID:
		if dialogID == -1 && variable == 5 && o.useTask == nil && !o.dead && c.s.hasQuestItems(c.player, c.s.data.Quests[impetusiumQuestID]) {
			p := c.player
			c.send(useObject(p.ID, o.id, 1))
			p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
			o.useTask = c.s.later(3*time.Second, func() {
				o.useTask = nil
				if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead || p.targetID != o.id {
					return
				}
				c.send(useObject(p.ID, o.id, 0))
				p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
				spawned := c.spawnImpetusiumNamed(o)
				o.dead, o.hp = true, 0
				if o.ai != nil {
					c.s.npcDied(o, nil)
				}
				if spawned == nil {
					c.s.log.Error("spawning Umkata the Restless", "quest", impetusiumQuestID)
				}
			})
			return true
		}
	}
	return false
}

func (c *conn) impetusiumCollect(o *object, quest *store.Quest) bool {
	template := c.s.data.Quests[impetusiumQuestID]
	if !c.s.hasQuestItems(c.player, template) {
		c.send(dialogWindow(o.id, 2120, impetusiumQuestID))
		return true
	}
	for _, item := range template.CollectItems {
		c.s.removeItemsByID(c.player, item.ID, item.Count)
	}
	if !c.beginAltgardStartupQuest(impetusiumQuestID, quest.Vars, "REWARD") {
		return false
	}
	c.send(dialogWindow(o.id, 1693, impetusiumQuestID))
	return true
}

func (c *conn) impetusiumKill(npcID int32) bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(impetusiumQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch npcID {
	case 210588:
		if variable >= 4 {
			return false
		}
		return c.beginAltgardStartupQuest(impetusiumQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
	case impetusiumNamedNPCID:
		if variable != 5 || !c.s.hasQuestItems(c.player, c.s.data.Quests[impetusiumQuestID]) {
			return false
		}
		return c.beginAltgardStartupQuest(impetusiumQuestID, setQuestVar(quest.Vars, 0, 7), "")
	default:
		return false
	}
}

func (c *conn) spawnImpetusiumNamed(source *object) *object {
	template := c.s.data.Npcs[impetusiumNamedNPCID]
	if template == nil {
		return nil
	}
	spawned := &object{
		id: c.s.ids.nextID(), worldID: source.worldID, instance: source.instance,
		x: source.x, y: source.y, z: source.z, heading: source.heading,
		homeX: source.x, homeY: source.y, homeZ: source.z, npc: template, noRespawn: true,
	}
	c.s.initNpc(spawned)
	c.s.byID[spawned.id] = spawned
	c.s.addObject(spawned)
	c.s.addDamage(spawned, c.player, 1000)
	return spawned
}
