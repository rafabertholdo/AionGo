package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

// kaliosCallEnterZone starts the quest on entry into Akarios Village.
func (c *conn) kaliosCallEnterZone(zoneName string) bool {
	if zoneName != "AKARIOS_VILLAGE" || c.player == nil || c.player.quest(1100) != nil {
		return false
	}
	script := c.s.data.QuestScripts[1100]
	if script == nil || !c.s.canStartQuest(c.player, script) || c.player.level < c.s.data.Quests[1100].MinLevel {
		return false
	}
	started := store.Quest{ID: 1100, Status: "START"}
	if err := c.s.quests.SaveQuest(c.player.ID, started); err != nil {
		c.s.log.Error("starting Kalio's Call", "err", err)
		return false
	}
	c.player.quests = append(c.player.quests, started)
	c.send(questAccepted(1, started))
	c.send(c.s.nearbyQuests(c.player))
	return true
}

// kaliosCallDialog handles Kalio's conversation and the five locked follow-ups.
func (c *conn) kaliosCallDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if c.player == nil || o == nil || o.npc == nil || o.npc.ID != 203067 || script == nil || script.ID != 1100 {
		return
	}
	q := c.player.quest(1100)
	if q == nil {
		return
	}
	if q.Status == "START" {
		switch dialogID {
		case 25:
			if c.customQuestProgress(1100, 1, "REWARD") {
				c.send(dialogWindow(o.id, 1011, 1100))
			}
		case 1007:
			c.send(dialogWindow(o.id, 4, 1100))
		case 1003:
			c.send(dialogWindow(o.id, 1004, 1100))
		}
		return
	}
	if q.Status != "REWARD" {
		return
	}
	if dialogID == 1009 {
		c.send(dialogWindow(o.id, 5, 1100))
	} else if dialogID == 17 {
		c.kaliosCallLockFollowUps()
		c.finishQuest(script, o.id, dialogID)
	}
}

// kaliosCallShowDialog is Java's reward preview for dialog ID -1.
func (c *conn) kaliosCallShowDialog(o *object, script *data.QuestScript) bool {
	if c.player == nil || o == nil || o.npc == nil || o.npc.ID != 203067 || script == nil || script.ID != 1100 {
		return false
	}
	q := c.player.quest(1100)
	if q == nil || q.Status != "REWARD" {
		return false
	}
	c.send(dialogWindow(o.id, 5, 1100))
	return true
}

func (c *conn) kaliosCallLockFollowUps() {
	for _, id := range []int32{1001, 1002, 1003, 1004, 1005} {
		if c.player.quest(id) != nil || c.s.data.Quests[id] == nil {
			continue
		}
		locked := store.Quest{ID: id, Status: "LOCKED"}
		if err := c.s.quests.SaveQuest(c.player.ID, locked); err != nil {
			c.s.log.Error("locking Kalio follow-up", "quest", id, "err", err)
			continue
		}
		c.player.quests = append(c.player.quests, locked)
		c.send(questAccepted(1, locked))
	}
	c.send(c.s.nearbyQuests(c.player))
}
