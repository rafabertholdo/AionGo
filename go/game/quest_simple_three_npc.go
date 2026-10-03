package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

// simpleThreeNPCQuestDialog ports the Java start, report, and final NPC
// conversations shared by thirteen quests. Quest 2651 advances to variable 3;
// the other twelve advance to variable 2 at the final NPC.
func (c *conn) simpleThreeNPCQuestDialog(o *object, script *data.QuestScript, d int32) bool {
	endVars := int32(2)
	if script.ID == 2651 {
		endVars = 3
	}
	noneStarts := script.ID >= 1324 && script.ID <= 1909
	return c.reportsQuestDialog(o, script, d, reportsShape{noneStarts: noneStarts, reports: []questReport{{script.MiddleNPC, 1352, 10000, 10}}, end: script.EndNPC, endVars: endVars})
}

// questReport is one report npc of reportsQuestDialog: talked to at variable 0 = its index, it shows page on 25,
// and on dialog raises the variable and sends window after (without a quest id).
type questReport struct {
	npc    int32
	page   uint16
	dialog int32
	after  uint16
}

// reportsShape is one Java handler of the shape "start npc, report npcs in order, end npc".
type reportsShape struct {
	noneStarts bool // the start npc also offers the quest in status NONE (not only without a row)
	handsIn    bool // the start npc also takes the quest in: START 25 -> 2375, 1009 -> REWARD; REWARD -> end dialog
	reports    []questReport
	end        int32
	endVars    int32
}

// reportsQuestDialog ports that shape: 25 -> 1011 at the start, each report on its variable (anything else there is
// defaultQuestStartDialog), and at the end 25 -> 2375, 1009 -> variable endVars and REWARD, everything else
// defaultQuestEndDialog.
func (c *conn) reportsQuestDialog(o *object, script *data.QuestScript, d int32, shape reportsShape) bool {
	q := c.player.quest(script.ID)
	if o.npc.ID == script.StartNPC {
		switch {
		case q == nil || shape.noneStarts && q.Status == "NONE":
			if d == 25 {
				c.send(dialogWindow(o.id, 1011, script.ID))
				return true
			}
			return c.defaultQuestStartDialog(o, script, d)
		case shape.handsIn && q.Status == "START":
			switch d {
			case 25:
				c.send(dialogWindow(o.id, 2375, script.ID))
				return true
			case 1009:
				c.updateQuest(script.ID, func(q *store.Quest) { q.Status = "REWARD" })
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
			return c.defaultQuestEndDialog(o, script, d)
		case shape.handsIn && q.Status == "REWARD":
			return c.defaultQuestEndDialog(o, script, d)
		}
		return false
	}
	for i, r := range shape.reports {
		if o.npc.ID != r.npc {
			continue
		}
		if q == nil || q.Status != "START" || questVar(q.Vars, 0) != int32(i) {
			return false
		}
		switch d {
		case 25:
			c.send(dialogWindow(o.id, r.page, script.ID))
			return true
		case r.dialog:
			c.updateQuest(script.ID, func(q *store.Quest) { q.Vars = setQuestVar(q.Vars, 0, int32(i+1)) })
			c.send(dialogWindow(o.id, r.after, 0))
			return true
		}
		return c.defaultQuestStartDialog(o, script, d)
	}
	if o.npc.ID != shape.end {
		return false
	}
	// Java takes any status; a finished quest must not become REWARD again (it would pay out twice).
	if q == nil || q.Status != "START" && q.Status != "REWARD" {
		return false
	}
	if d == 25 && q.Status == "START" {
		c.send(dialogWindow(o.id, 2375, script.ID))
		return true
	}
	if d == 1009 {
		c.updateQuest(script.ID, func(q *store.Quest) { q.Vars, q.Status = shape.endVars, "REWARD" })
	}
	return c.defaultQuestEndDialog(o, script, d)
}
