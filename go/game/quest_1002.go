package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// requestOfTheElimLevelUp is Java's LOCKED quest level-up event.
func (c *conn) requestOfTheElimLevelUp() bool {
	if c.player == nil {
		return false
	}
	q := c.player.quest(1002)
	if q == nil || q.Status != "LOCKED" {
		return false
	}
	next := *q
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Request of the Elim", "err", err)
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

// requestOfTheElimEnterWorld is Java's ascension morph in the trial instance.
func (c *conn) requestOfTheElimEnterWorld() bool {
	if c.player == nil || c.player.WorldID != 310010000 {
		return false
	}
	q := c.player.quest(1002)
	if q == nil || q.Status != "START" {
		return false
	}
	w := wire.Packet(smAscensionMorph)
	w.D(1)
	c.send(w)
	return true
}

func (c *conn) requestOfTheElimDialog(o *object, script *data.QuestScript, dialogID int32) {
	if c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != 1002 {
		return
	}
	p := c.player
	q := p.quest(1002)
	if q == nil {
		return
	}
	if q.Status == "REWARD" {
		if o.npc.ID != 203067 {
			return
		}
		switch {
		case dialogID == -1:
			c.send(dialogWindow(o.id, 2716, 1002))
		case dialogID == 1009:
			c.send(dialogWindow(o.id, 5, 1002))
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuest(script, o.id, uint16(dialogID))
		}
		return
	}
	if q.Status != "START" {
		return
	}
	varID := questVar(q.Vars, 0)
	send := func(page uint16) { c.send(dialogWindow(o.id, page, 1002)) }
	advance := func(next int32, status string, page uint16) {
		if c.customQuestProgress(1002, setQuestVar(q.Vars, 0, next), status) {
			c.send(dialogWindow(o.id, page, 0))
		}
	}
	switch o.npc.ID {
	case 203076:
		if varID == 0 {
			if dialogID == 25 {
				send(1011)
			} else if dialogID == 10000 {
				advance(1, "", 10)
			}
		}
	case 730007:
		switch dialogID {
		case 25:
			switch varID {
			case 1:
				send(1352)
			case 5:
				c.s.removeItemsByID(p, 182200002, 1)
				send(1693)
			case 6:
				send(2034)
			}
		case 33:
			if varID == 6 {
				if !c.s.hasQuestItems(p, c.s.data.Quests[1002]) {
					send(2205)
				} else if c.customQuestProgress(1002, setQuestVar(q.Vars, 0, 12), "") {
					for _, item := range c.s.data.Quests[1002].CollectItems {
						c.s.removeItemsByID(p, item.ID, item.Count)
					}
					send(2120)
				}
			} else if varID == 12 {
				send(2120)
			}
		case 1353:
			if varID == 1 {
				w := wire.Packet(smPlayMovie)
				w.C(0)
				w.D(0)
				w.D(0)
				w.H(20)
				w.D(0)
				c.send(w)
				c.dialogNotHandled() // Java returns false: window 1353 follows the movie
			}
		case 10001:
			if varID == 1 {
				needsItem := c.s.countItems(p, 182200002) == 0
				if needsItem {
					if !c.s.questRewardsFit(p, []data.QuestItem{{ID: 182200002, Count: 1}}) {
						c.send(systemMessage(msgInventoryFull))
						return
					}
				}
				if c.customQuestProgress(1002, setQuestVar(q.Vars, 0, 2), "") {
					if needsItem {
						c.s.addItem(p, 182200002, 1)
					}
					c.send(dialogWindow(o.id, 10, 0))
				}
			}
		case 10002:
			if varID == 5 {
				advance(6, "", 10)
			}
		case 10003:
			if varID == 12 {
				advance(13, "", 10)
			}
		}
	case 730010:
		if varID < 2 || varID >= 5 {
			return
		}
		if dialogID == -1 {
			if c.s.countItems(p, 182200002) == 0 || o.useTask != nil {
				return
			}
			c.send(useObject(p.ID, o.id, 1))
			p.broadcast(c.s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
			o.useTask = c.s.later(3*time.Second, func() {
				o.useTask = nil
				current := p.quest(1002)
				if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead || p.targetID != o.id || current == nil || current.Status != "START" || c.s.countItems(p, 182200002) == 0 {
					return
				}
				c.send(useObject(p.ID, o.id, 0))
				p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
				c.send(dialogWindow(o.id, 10, 0))
			})
		} else if dialogID == 25 {
			next := varID + 1
			if varID == 2 {
				next++
			}
			if c.customQuestProgress(1002, setQuestVar(q.Vars, 0, next), "") {
				c.send(dialogWindow(o.id, 0, 0))
				o.dead, o.hp = true, 0
				if o.ai != nil {
					c.s.npcDied(o, nil)
				}
			}
		}
	case 730008:
		switch dialogID {
		case 25:
			if varID == 13 {
				send(2375)
			} else if varID == 14 {
				send(2461)
			}
		case 10004:
			if varID == 13 && c.customQuestProgress(1002, setQuestVar(q.Vars, 0, 20), "") {
				c.send(dialogWindow(o.id, 0, 0))
				in := c.s.newInstance(310010000)
				in.registered[p.ID] = true
				c.s.teleportToInstance(p, 310010000, in.id, 52, 174, 229, 0, 0)
			}
		case 10005:
			if varID == 14 {
				advance(14, "REWARD", 10)
			}
		}
	case 205000:
		if varID == 20 && dialogID == 25 {
			p.broadcast(c.s.playerEmotionTo(p, emoteStartFlyTele, 1001, 0, 0, 0, 0, 0), true)
			c.s.later(43*time.Second, func() {
				current := p.quest(1002)
				if p.conn != c || current == nil || current.Status != "START" || questVar(current.Vars, 0) != 20 {
					return
				}
				if c.customQuestProgress(1002, setQuestVar(current.Vars, 0, 14), "") {
					c.s.teleportToInstance(p, 210010000, 1, 603, 1537, 116, 20, 0)
				}
			})
		}
	}
}
