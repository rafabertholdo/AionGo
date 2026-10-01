package game

import "aionlightning/game/data"

// insectProblemDialog handles the two mutually exclusive hunts offered by 203533.
func (c *conn) insectProblemDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if o.npc == nil || o.npc.ID != 203533 {
		return
	}
	q := c.player.quest(script.ID)
	if q == nil || q.Status == "NONE" {
		if !c.s.canStartQuest(c.player, script) {
			return
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 1011, script.ID))
		case 10000, 10001:
			c.startQuest(script, o.id)
			if q = c.player.quest(script.ID); q != nil && q.Status == "START" {
				vars := int32(1)
				if dialogID == 10001 {
					vars = 11
				}
				if c.customQuestProgress(script.ID, vars, "") {
					c.send(dialogWindow(o.id, 10, 0))
				}
			}
		}
		return
	}
	if q.Status != "REWARD" {
		return
	}
	switch dialogID {
	case 1009:
		if q.Vars == 10 || q.Vars == 20 {
			c.send(dialogWindow(o.id, uint16(q.Vars/10+4), script.ID))
		}
	case 17:
		if q.Vars == 10 || q.Vars == 20 {
			c.finishQuestReward(script, o.id, dialogID, int(q.Vars/10-1))
		}
	}
}

// insectProblemShowDialog covers Java's dialog ID -1 reward preview.
func (c *conn) insectProblemShowDialog(o *object, script *data.QuestScript) bool {
	if o.npc == nil || o.npc.ID != 203533 {
		return false
	}
	q := c.player.quest(script.ID)
	if q == nil || q.Status != "REWARD" || q.Vars != 10 && q.Vars != 20 {
		return false
	}
	c.send(dialogWindow(o.id, uint16(q.Vars/10+4), script.ID))
	return true
}

// insectProblemKill preserves Java's tenth counter increment and next-kill
// reward transition as separate events.
func (c *conn) insectProblemKill(npcID int32) bool {
	q := c.player.quest(2114)
	if q == nil || q.Status != "START" {
		return false
	}
	valid := npcID == 210734 && q.Vars >= 1 && q.Vars <= 10 ||
		(npcID == 210380 || npcID == 210381) && q.Vars >= 11 && q.Vars <= 20
	if !valid {
		return false
	}
	if q.Vars == 10 || q.Vars == 20 {
		return c.customQuestProgress(2114, q.Vars, "REWARD")
	}
	return c.customQuestProgress(2114, q.Vars+1, "")
}
