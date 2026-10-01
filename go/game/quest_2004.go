package game

import (
	"time"

	"aionlightning/game/data"
)

func (c *conn) aCharmedCubeLevelUp() bool {
	if c.player == nil || c.player.level < 3 {
		return false
	}
	q, previous := c.player.quest(2004), c.player.quest(2003)
	if q == nil || q.Status != "LOCKED" || previous == nil || previous.Status != "COMPLETE" {
		return false
	}
	next := *q
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) aCharmedCubeEvent(o *object, script *data.QuestScript, dialogID int32, kill bool) bool {
	if c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != 2004 {
		return false
	}
	q := c.player.quest(2004)
	if q == nil {
		return false
	}
	id, variable := o.npc.ID, questVar(q.Vars, 0)
	if kill {
		if q.Status != "START" || (id != 210402 && id != 210403) {
			return false
		}
		if variable >= 3 && variable < 8 {
			return c.customQuestProgress(2004, setQuestVar(q.Vars, 0, variable+1), "")
		}
		if variable == 8 {
			return c.customQuestProgress(2004, q.Vars, "REWARD")
		}
		return false
	}
	if q.Status == "REWARD" && id == 203539 {
		if dialogID == -1 {
			c.send(dialogWindow(o.id, 2375, 2004))
			return true
		}
		if dialogID == 1009 {
			c.send(dialogWindow(o.id, 5, 2004))
			return true
		}
		if dialogID >= 8 && dialogID <= 17 {
			c.finishQuest(script, o.id, uint16(dialogID))
			return true
		}
		return false
	}
	if q.Status != "START" {
		return false
	}
	send := func(page uint16) bool { c.send(dialogWindow(o.id, page, 2004)); return true }
	advance := func(next int32, status string) bool {
		if !c.customQuestProgress(2004, setQuestVar(q.Vars, 0, next), status) {
			return false
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	}
	switch id {
	case 203539:
		switch dialogID {
		case 25:
			if variable == 0 {
				return send(1011)
			}
			if variable == 1 {
				return send(1352)
			}
		case 10000, 10001:
			if variable == 0 || variable == 1 {
				return advance(variable+1, "")
			}
		case 33:
			if variable == 1 {
				if c.s.countItems(c.player, 182203005) > 0 {
					return send(1438)
				}
				return send(1353)
			}
		}
	case 203550:
		switch dialogID {
		case 25:
			if variable == 2 {
				return send(1693)
			}
			if variable == 6 {
				return send(2034)
			}
		case 10002:
			if variable == 2 {
				if c.s.countItems(c.player, 182203005) == 0 {
					return false
				}
				if advance(3, "") {
					c.s.removeItemsByID(c.player, 182203005, 1)
					return true
				}
			}
		case 10003:
			if variable == 6 {
				return advance(variable, "REWARD")
			}
		}
	case 700047:
		if dialogID != -1 || variable != 1 || o.useTask != nil || o.dead {
			return false
		}
		p := c.player
		c.send(useObject(p.ID, o.id, 1))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
		o.useTask = c.s.later(3*time.Second, func() {
			o.useTask = nil
			current := p.quest(2004)
			if p.conn != c || !p.spawned || p.seen[o.id] != o || p.targetID != o.id || o.dead || current == nil || current.Status != "START" || questVar(current.Vars, 0) != 1 {
				return
			}
			c.send(useObject(p.ID, o.id, 0))
			p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
			if template := c.s.data.Npcs[211755]; template != nil {
				spawn := &object{id: c.s.ids.nextID(), worldID: o.worldID, instance: o.instance, x: o.x, y: o.y, z: o.z, heading: o.heading, homeX: o.x, homeY: o.y, homeZ: o.z, npc: template}
				c.s.initNpc(spawn)
				c.s.byID[spawn.id] = spawn
				c.s.addObject(spawn)
			}
			o.dead, o.hp = true, 0
			if o.ai != nil {
				c.s.npcDied(o, nil)
			}
		})
		return true
	}
	return false
}
