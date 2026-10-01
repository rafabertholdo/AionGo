package game

import (
	"aionlightning/game/data"
)

// delayedItemQuestNPCDialog handles the starter conversation used by the
// delayed item-use handlers that have separate start and end NPCs. The end
// NPC follows the shared delayed-item turn-in flow.
func (c *conn) delayedItemQuestNPCDialog(o *object, script *data.QuestScript, dialogID uint16) bool {
	if c.player == nil || o == nil || o.npc == nil || script == nil || script.ItemUseDelay <= 0 {
		return false
	}
	if o.npc.ID == script.StartNPC {
		q := c.player.quest(script.ID)
		if q == nil {
			return false
		}
		if dialogID == ^uint16(0) && q.Status == "START" {
			c.send(dialogWindow(o.id, 1352, script.ID))
			return true
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 1352, script.ID))
			return true
		case 10000:
			next := *q
			next.Vars = setQuestVar(q.Vars, 0, questVar(q.Vars, 0)+1)
			if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
				c.s.log.Error("updating delayed-item quest", "quest", script.ID, "err", err)
				return false
			}
			*q = next
			c.send(questAccepted(2, next))
			c.send(dialogWindow(o.id, 10, 0))
			return true
		default:
			return false
		}
	}
	if o.npc.ID == script.EndNPC {
		if dialogID == ^uint16(0) {
			q := c.player.quest(script.ID)
			if q == nil {
				return false
			}
			switch q.Status {
			case "START":
				c.send(dialogWindow(o.id, 2375, script.ID))
				return true
			case "REWARD":
				c.send(dialogWindow(o.id, 5, script.ID))
				return true
			default:
				return false
			}
		}
		c.delayedItemQuestDialog(o, script, dialogID)
		return true
	}
	return false
}
