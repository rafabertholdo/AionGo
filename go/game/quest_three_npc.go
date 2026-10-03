package game

import "aionlightning/game/data"

// threeNPCQuestDialog ports thirty-four Java handlers with the same
// start, one to three reports, and optional final-NPC branches. Several repeat
// the start NPC as the final branch; that branch is unreachable there, too.
// reportsQuests are the Java handlers of the reportsQuestDialog shape, read from their onDialogEvent (npcs, pages and
// dialog ids per report, the end npc and its variable).
var reportsQuests = map[int32]reportsShape{
	2501: {noneStarts: true, handsIn: true, end: 204701, endVars: 3, reports: []questReport{{204713, 1352, 10000, 10}, {204719, 1693, 10001, 10}, {204733, 2034, 10002, 10}}},
	2512: {noneStarts: true, handsIn: true, end: 204753, endVars: 3, reports: []questReport{{204801, 1352, 10000, 10}}},
	2514: {noneStarts: true, handsIn: true, end: 204702, endVars: 3, reports: []questReport{{204700, 1352, 10000, 10}, {204763, 1693, 10001, 10}}},
	2515: {noneStarts: true, handsIn: true, end: 790015, endVars: 3, reports: []questReport{{204192, 1352, 10000, 10}, {204205, 1693, 10001, 10}, {798081, 2034, 10002, 10}}},
	2523: {noneStarts: true, handsIn: true, end: 204734, endVars: 3, reports: []questReport{{798117, 1352, 10000, 10}, {798118, 1693, 10001, 10}, {798119, 2034, 10002, 10}}},
	2539: {noneStarts: true, handsIn: true, end: 790022, endVars: 3, reports: []questReport{{204433, 1352, 10000, 10}, {204112, 1693, 10001, 10}, {204056, 2034, 10002, 10}}},
	2553: {noneStarts: true, handsIn: true, end: 798119, endVars: 3, reports: []questReport{{204702, 1352, 10000, 10}}},
	2569: {noneStarts: true, handsIn: true, end: 204768, endVars: 3, reports: []questReport{{204701, 1352, 10000, 10}}},
	2583: {noneStarts: true, handsIn: true, end: 204715, endVars: 3, reports: []questReport{{204805, 1352, 10000, 10}, {204361, 1693, 10001, 10}}},
	2611: {noneStarts: true, handsIn: true, end: 204763, endVars: 3, reports: []questReport{{204773, 1352, 10000, 10}, {204772, 1693, 10001, 10}, {204700, 2034, 10002, 10}}},
	2692: {noneStarts: true, handsIn: true, end: 212164, endVars: 3, reports: []questReport{{204108, 1352, 10000, 10}, {279027, 1693, 10001, 10}, {279029, 2034, 10002, 10}}},
	2767: {noneStarts: true, handsIn: true, end: 279004, endVars: 3, reports: []questReport{{279024, 1352, 10000, 10}, {279022, 1693, 10001, 10}}},
	2773: {noneStarts: true, handsIn: true, end: 204734, endVars: 3, reports: []questReport{{798110, 1352, 10000, 10}}},
	2912: {noneStarts: true, handsIn: true, end: 204236, endVars: 3, reports: []questReport{{204089, 1352, 10000, 10}, {204088, 2034, 10002, 10}, {204240, 1693, 10001, 10}}},
	2913: {noneStarts: true, handsIn: true, end: 204173, endVars: 3, reports: []questReport{{204170, 1352, 10000, 10}, {798065, 1693, 10001, 10}}},
	2914: {noneStarts: true, handsIn: true, end: 204147, endVars: 3, reports: []questReport{{204236, 1352, 10000, 10}}},
	2917: {noneStarts: true, handsIn: true, end: 204241, endVars: 3, reports: []questReport{{798029, 1352, 10000, 10}, {204108, 1693, 10001, 10}}},
	2928: {noneStarts: true, handsIn: true, end: 204261, endVars: 3, reports: []questReport{{204235, 1352, 10000, 10}}},
	2953: {noneStarts: true, handsIn: true, end: 204191, endVars: 3, reports: []questReport{{204071, 1352, 10000, 10}}},
	2954: {noneStarts: true, handsIn: true, end: 204191, endVars: 3, reports: []questReport{{204221, 1352, 10000, 10}}},
	2965: {noneStarts: true, handsIn: true, end: 278109, endVars: 3, reports: []questReport{{204055, 1352, 10000, 10}, {278002, 1693, 10001, 10}}},
	3020: {noneStarts: true, handsIn: true, end: 798143, endVars: 3, reports: []questReport{{798149, 1352, 10000, 10}}},
	3023: {noneStarts: true, handsIn: true, end: 798138, endVars: 3, reports: []questReport{{203785, 1352, 10000, 10}, {798222, 1693, 10000, 10}}},
	3035: {noneStarts: true, handsIn: true, end: 798155, endVars: 3, reports: []questReport{{203830, 1352, 10000, 10}, {279029, 1693, 10001, 10}}},
	3037: {noneStarts: true, handsIn: true, end: 798166, endVars: 3, reports: []questReport{{798199, 1352, 10000, 10}}},
	3076: {noneStarts: true, handsIn: true, end: 798155, endVars: 3, reports: []questReport{{278503, 1352, 10000, 10}, {278556, 1693, 10001, 10}}},
	3081: {noneStarts: true, handsIn: true, end: 798116, endVars: 3, reports: []questReport{{203830, 1352, 10000, 10}}},
	3083: {noneStarts: true, handsIn: true, end: 798156, endVars: 3, reports: []questReport{{730024, 1352, 10000, 10}, {798215, 1693, 10001, 10}}},
	4101: {noneStarts: true, handsIn: true, end: 205193, endVars: 3, reports: []questReport{{205194, 1352, 10000, 10}, {205195, 1693, 10001, 10}, {205196, 2375, 1009, 10}}},
}

func (c *conn) threeNPCQuestDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if shape, ok := reportsQuests[script.ID]; ok {
		c.dialogResult(c.reportsQuestDialog(o, script, int32(int16(dialogID)), shape))
		return
	}
	if c.player == nil || o == nil || o.npc == nil || script == nil || script.MiddleNPC == 0 {
		return
	}
	q := c.player.quest(script.ID)
	if o.npc.ID == script.StartNPC {
		if q == nil || q.Status == "NONE" || q.Status == "COMPLETE" {
			c.customQuestStart(o, script, dialogID, 0)
			return
		}
		if q.Status == "START" {
			if dialogID == 25 {
				c.send(dialogWindow(o.id, 2375, script.ID))
			} else if dialogID == 1009 && c.customQuestProgress(script.ID, q.Vars, "REWARD") {
				c.send(dialogWindow(o.id, 10, 0))
			}
			return
		}
		if q.Status == "REWARD" {
			c.threeNPCQuestEnd(o, script, dialogID)
		}
		return
	}
	if q == nil {
		return
	}
	if o.npc.ID == script.MiddleNPC && q.Status == "START" && q.Vars == 0 {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1352, script.ID))
		} else if dialogID == 10000 && c.customQuestProgress(script.ID, 1, "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
		return
	}
	if o.npc.ID == script.MiddleNPC2 && q.Status == "START" && q.Vars == 1 {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1693, script.ID))
		} else if dialogID == 10001 && c.customQuestProgress(script.ID, 2, "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
		return
	}
	if o.npc.ID == script.MiddleNPC3 && q.Status == "START" && q.Vars == 2 {
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 2034, script.ID))
		} else if dialogID == 10002 && c.customQuestProgress(script.ID, 3, "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
		return
	}
	if script.FinalNPC != 0 && o.npc.ID == script.FinalNPC {
		if q.Status == "START" {
			if dialogID == 25 {
				c.send(dialogWindow(o.id, 2375, script.ID))
			} else if dialogID == 1009 && c.customQuestProgress(script.ID, 3, "REWARD") {
				c.send(dialogWindow(o.id, 5, script.ID))
			}
		} else if q.Status == "REWARD" {
			c.threeNPCQuestEnd(o, script, dialogID)
		}
	}
}

func (c *conn) threeNPCQuestEnd(o *object, script *data.QuestScript, dialogID uint16) {
	if dialogID == 1009 {
		c.send(dialogWindow(o.id, 5, script.ID))
	} else if dialogID >= 8 && dialogID <= 17 {
		c.finishQuestReward(script, o.id, 17, 0)
	}
}
