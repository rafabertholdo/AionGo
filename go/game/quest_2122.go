package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// ashesToAshesEvent ports the item, NPC, and urn events of Ashes to Ashes.
// An item event has item != nil and o == nil; a client show-dialog request uses -1.
func (c *conn) ashesToAshesEvent(o *object, item *store.Item, script *data.QuestScript, dialogID int32) {
	if c.player == nil || script == nil || script.ID != 2122 {
		return
	}
	p := c.player
	q := p.quest(script.ID)
	if item != nil {
		if o != nil {
			return
		}
		c.questStartItemUse(item, script)
		return
	}
	if o == nil {
		c.itemStartedQuestDialog(script, uint16(dialogID))
		return
	}
	if o.npc == nil {
		return
	}
	switch o.npc.ID {
	case 203551:
		if q == nil {
			return
		}
		if q.Status == "REWARD" {
			if dialogID == -1 {
				c.send(dialogWindow(o.id, 2375, script.ID))
			} else if dialogID == 1009 {
				c.send(dialogWindow(o.id, 5, script.ID))
			} else if dialogID >= 8 && dialogID <= 17 {
				c.finishQuest(script, o.id, uint16(dialogID))
			}
			return
		}
		if q.Status != "START" || questVar(q.Vars, 0) != 0 {
			return
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 1011, script.ID))
		case 1012:
			c.s.removeItemsByID(p, 182203120, c.s.countItems(p, 182203120))
		case 10000:
			if c.customQuestProgress(script.ID, setQuestVar(q.Vars, 0, 1), "") {
				c.send(dialogWindow(o.id, 10, 0))
			}
		}
	case 700148:
		// Talking to the ash source is accepted by Java but changes no state.
	case 730029:
		if q == nil || q.Status != "START" {
			return
		}
		switch dialogID {
		case -1:
			if c.s.countItems(p, 182203133) < 3 {
				c.send(dialogWindow(o.id, 1693, script.ID))
				return
			}
			if o.useTask != nil || o.dead {
				return
			}
			c.send(useObject(p.ID, o.id, 1))
			p.broadcast(c.s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
			o.useTask = c.s.later(3*time.Second, func() {
				o.useTask = nil
				if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead || p.targetID != o.id {
					return
				}
				current := p.quest(script.ID)
				if current == nil || current.Status != "START" || c.s.countItems(p, 182203133) < 3 {
					return
				}
				c.send(useObject(p.ID, o.id, 0))
				p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
				c.send(dialogWindow(o.id, 1352, script.ID))
			})
		case 10001:
			if c.s.countItems(p, 182203133) < 3 {
				return
			}
			if c.customQuestProgress(script.ID, q.Vars, "REWARD") {
				c.s.removeItemsByID(p, 182203133, 3)
				c.send(dialogWindow(0, 0, 0))
			}
		}
	}
}
