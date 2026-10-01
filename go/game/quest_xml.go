package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// xmlQuestDialog runs the event operations declared by the five xml_quest
// scripts. Its result is Java's operations.override: false continues to the
// template's default start or end dialog after the operations run.
func (c *conn) xmlQuestDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	p := c.player
	q := p.quest(script.ID)
	if q == nil || q.Status == "NONE" || q.Status == "COMPLETE" {
		return false
	}
	for _, event := range script.TalkEvents {
		if required := event.Conditions.QuestStatus.Value; required != "" && required != q.Status {
			continue
		}
		for _, variable := range event.Vars {
			if q.Vars != variable.Value {
				continue
			}
			for _, npc := range variable.NPCs {
				if npc.ID != o.npc.ID {
					continue
				}
				for _, dialog := range npc.Dialogs {
					if dialog.ID != dialogID {
						continue
					}
					c.xmlQuestOperations(o, script, dialog.Operations.Actions)
					return dialog.Operations.Override == nil || *dialog.Operations.Override
				}
			}
		}
	}
	return false
}

func (c *conn) xmlQuestOperations(o *object, script *data.QuestScript, actions []data.QuestXMLAction) {
	for _, action := range actions {
		switch action.XMLName.Local {
		case "npc_dialog":
			questID := script.ID
			if action.QuestID != nil {
				questID = *action.QuestID
			}
			c.send(dialogWindow(o.id, uint16(action.ID), questID))
		case "set_quest_var":
			c.xmlQuestUpdate(script.ID, func(q *store.Quest) { q.Vars = setQuestVar(q.Vars, action.VarID, action.Value) })
		case "set_quest_status":
			c.xmlQuestUpdate(script.ID, func(q *store.Quest) { q.Status = action.Status })
		case "give_item":
			c.s.addItem(c.player, action.ItemID, action.Count)
		case "collect_items":
			if c.s.hasQuestItems(c.player, c.s.data.Quests[script.ID]) {
				for _, item := range c.s.data.Quests[script.ID].CollectItems {
					c.s.removeItemsByID(c.player, item.ID, item.Count)
				}
				for _, branch := range action.Actions {
					if branch.XMLName.Local == "true" {
						c.xmlQuestOperations(o, script, branch.Actions)
					}
				}
			} else {
				for _, branch := range action.Actions {
					if branch.XMLName.Local == "false" {
						c.xmlQuestOperations(o, script, branch.Actions)
					}
				}
			}
		case "npc_use":
			c.xmlQuestUse(o, script, action.Actions)
		}
	}
}

func (c *conn) xmlQuestUpdate(id int32, change func(*store.Quest)) {
	q := c.player.quest(id)
	if q == nil || q.Status == "COMPLETE" || q.Status == "NONE" {
		return
	}
	next := *q
	change(&next)
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("updating XML quest", "quest", id, "err", err)
		return
	}
	*q = next
	c.send(questAccepted(2, next))
	if next.Status == "COMPLETE" {
		c.send(c.s.nearbyQuests(c.player))
	}
}

func (c *conn) xmlQuestUse(o *object, script *data.QuestScript, actions []data.QuestXMLAction) {
	p := c.player
	if o.useTask != nil || o.dead {
		return
	}
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead {
			return
		}
		q := p.quest(script.ID)
		if q == nil || q.Status != "START" {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		for _, branch := range actions {
			if branch.XMLName.Local == "finish" {
				c.xmlQuestOperations(o, script, branch.Actions)
			}
		}
	})
}
