package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

func (c *conn) wheresRaeLevelUp() bool {
	if c.player == nil {
		return false
	}
	q := c.player.quest(2002)
	if q == nil || q.Status != "LOCKED" {
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

func (c *conn) wheresRaeEvent(o *object, script *data.QuestScript, dialogID int32, kill bool) bool {
	if c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != 2002 {
		return false
	}
	q := c.player.quest(2002)
	if q == nil {
		return false
	}
	id, variable := o.npc.ID, questVar(q.Vars, 0)
	if kill {
		if q.Status != "START" || (id != 210377 && id != 210378) || variable < 3 || variable >= 10 {
			return false
		}
		return c.customQuestProgress(2002, setQuestVar(q.Vars, 0, variable+1), "")
	}
	if q.Status == "REWARD" && id == 203516 {
		switch {
		case dialogID == -1:
			c.send(dialogWindow(o.id, 3398, 2002))
		case dialogID == 10007:
			c.send(dialogWindow(o.id, 5, 2002))
		case dialogID >= 8 && dialogID <= 11:
			c.finishQuest(script, o.id, uint16(dialogID))
		default:
			return false
		}
		return true
	}
	if q.Status != "START" {
		return false
	}
	send := func(page uint16) bool { c.send(dialogWindow(o.id, page, 2002)); return true }
	advance := func(next int32, status string) bool {
		if !c.customQuestProgress(2002, setQuestVar(q.Vars, 0, next), status) {
			return false
		}
		c.send(dialogWindow(o.id, 10, 0))
		return true
	}
	switch id {
	case 203519:
		if variable == 0 {
			if dialogID == 25 {
				return send(1011)
			}
			if dialogID == 10000 {
				return advance(1, "")
			}
		}
	case 203534:
		if variable == 1 {
			if dialogID == 25 {
				return send(1352)
			}
			if dialogID == 10001 {
				return advance(2, "")
			}
		}
		if dialogID == 1353 {
			c.send(wheresRaeMovie(52))
			return false
		}
	case 790002:
		switch dialogID {
		case 25:
			switch variable {
			case 2:
				return send(1693)
			case 10:
				return send(2034)
			case 11:
				return send(2375)
			case 12:
				return send(2462)
			case 13:
				return send(2716)
			}
		case 10002, 10003, 10005:
			if variable == 2 || variable == 10 {
				return advance(variable+1, "")
			}
			if variable == 13 {
				return advance(14, "")
			}
		case 33:
			if variable == 11 {
				if !c.s.hasQuestItems(c.player, c.s.data.Quests[2002]) {
					return send(2376)
				}
				if !c.customQuestProgress(2002, setQuestVar(q.Vars, 0, 12), "") {
					return false
				}
				c.s.removeItemsByID(c.player, 182203003, 1)
				return send(2461)
			}
		case 10004:
			if variable == 12 && advance(99, "") {
				in := c.s.newInstance(320010000)
				in.registered[c.player.ID] = true
				c.s.teleportToInstance(c.player, 320010000, in.id, 457.65, 426.8, 230.4, 75, 0)
				return true
			}
		}
	case 205020:
		if dialogID == 25 {
			p := c.player
			p.broadcast(c.s.playerEmotionTo(p, emoteStartFlyTele, 3001, 0, 0, 0, 0, 0), true)
			c.s.later(38*time.Second, func() {
				current := p.quest(2002)
				if p.conn != c || current == nil || current.Status != "START" {
					return
				}
				if c.customQuestProgress(2002, setQuestVar(current.Vars, 0, 13), "") {
					c.s.teleportToInstance(p, 220010000, 1, 940.15, 2295.64, 265.7, 43, 0)
				}
			})
			return true
		}
	case 700045:
		if variable == 11 && dialogID == -1 && o.useTask == nil {
			p := c.player
			c.send(useObject(p.ID, o.id, 1))
			p.broadcast(c.s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
			o.useTask = c.s.later(3*time.Second, func() {
				o.useTask = nil
				current := p.quest(2002)
				if p.conn != c || current == nil || current.Status != "START" || questVar(current.Vars, 0) != 11 || o.dead {
					return
				}
				c.send(useObject(p.ID, o.id, 0))
				p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
				o.dead, o.hp = true, 0
				c.s.npcDied(o, p)
				c.s.registerDrop(o, p)
				c.s.openLoot(p, o.id)
			})
		}
		return true
	case 203537:
		if variable == 14 && dialogID == -1 {
			if !c.customQuestProgress(2002, setQuestVar(q.Vars, 0, 15), "") {
				return false
			}
			if template := c.s.data.Npcs[203553]; template != nil {
				rae := &object{id: c.s.ids.nextID(), worldID: o.worldID, instance: o.instance, x: o.x, y: o.y, z: o.z, heading: o.heading, homeX: o.x, homeY: o.y, homeZ: o.z, npc: template}
				c.s.initNpc(rae)
				c.s.byID[rae.id] = rae
				c.s.addObject(rae)
			}
			o.dead, o.hp = true, 0
			c.s.npcDied(o, nil)
			c.send(wheresRaeMovie(256))
			return true
		}
	case 203553:
		if variable == 15 {
			if dialogID == 25 {
				return send(3057)
			}
			if dialogID == 10006 {
				if !c.customQuestProgress(2002, q.Vars, "REWARD") {
					return false
				}
				c.s.despawnNpc(o, true)
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	}
	return false
}

func wheresRaeMovie(id uint16) *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(0)
	w.D(0)
	w.D(0)
	w.H(id)
	w.D(0)
	return w
}
