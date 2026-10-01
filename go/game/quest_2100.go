package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

// orderOfTheCaptainEnterZone starts the quest on entry into Aldelle Village.
func (c *conn) orderOfTheCaptainEnterZone(zoneName string) bool {
	if c.player == nil || zoneName != "ALDELLE_VILLAGE" || c.player.quest(2100) != nil {
		return false
	}
	script := c.s.data.QuestScripts[2100]
	if script == nil || !c.s.canStartQuest(c.player, script) || c.player.level < c.s.data.Quests[2100].MinLevel {
		return false
	}
	started := store.Quest{ID: 2100, Status: "START"}
	if err := c.s.quests.SaveQuest(c.player.ID, started); err != nil {
		c.s.log.Error("starting Order of the Captain", "err", err)
		return false
	}
	c.player.quests = append(c.player.quests, started)
	c.send(questAccepted(1, started))
	c.send(c.s.nearbyQuests(c.player))
	return true
}

// orderOfTheCaptainDialog preserves Java's locked follow-up quests at turn-in.
func (c *conn) orderOfTheCaptainDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if c.player == nil || o == nil || o.npc == nil || o.npc.ID != 203516 || script == nil || script.ID != 2100 {
		return
	}
	q := c.player.quest(2100)
	if q == nil {
		return
	}
	switch q.Status {
	case "START":
		if dialogID == 25 {
			if c.customQuestProgress(2100, 1, "REWARD") {
				c.send(dialogWindow(o.id, 1011, 2100))
			}
		} else if dialogID == 1009 {
			c.send(dialogWindow(o.id, 5, 2100))
		}
	case "REWARD":
		if dialogID == 1009 {
			c.send(dialogWindow(o.id, 5, 2100))
			return
		}
		if dialogID != 17 {
			return
		}
		// QuestService.startQuest(..., LOCKED) bypasses normal start conditions.
		for id := int32(2001); id <= 2007; id++ {
			if existing := c.player.quest(id); existing != nil && existing.Status != "NONE" {
				continue
			}
			locked := store.Quest{ID: id, Status: "LOCKED"}
			if err := c.s.quests.SaveQuest(c.player.ID, locked); err != nil {
				c.s.log.Error("locking follow-up quest", "quest", id, "err", err)
				return
			}
			if existing := c.player.quest(id); existing == nil {
				c.player.quests = append(c.player.quests, locked)
			} else {
				*existing = locked
			}
			c.send(questAccepted(1, locked))
		}
		c.finishQuest(script, o.id, dialogID)
	}
}
