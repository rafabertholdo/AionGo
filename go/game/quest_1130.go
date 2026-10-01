package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	summonsToCitadelQuestID int32 = 1130
	summonsToCitadelNPC     int32 = 203098
)

// summonsToCitadelEnterZone starts the quest the first time the player enters
// Verteron Citadel, matching Java's null-state check for this zone event.
func (c *conn) summonsToCitadelEnterZone(zoneName string) bool {
	if c == nil || c.player == nil || c.player.WorldID != 210030000 || zoneName != "VERTERON_CITADEL" || c.player.quest(summonsToCitadelQuestID) != nil {
		return false
	}
	started := store.Quest{ID: summonsToCitadelQuestID, Status: "START"}
	if err := c.s.quests.SaveQuest(c.player.ID, started); err != nil {
		c.s.log.Error("starting Summons to the Citadel", "err", err)
		return false
	}
	c.player.quests = append(c.player.quests, started)
	c.send(questAccepted(1, started))
	c.send(c.s.nearbyQuests(c.player))
	return true
}

// summonsToCitadelDialog completes the brief report to Aegir and locks the
// subsequent Elyos Verteron quest chain when the reward is selected.
func (c *conn) summonsToCitadelDialog(o *object, script *data.QuestScript, dialogID uint16) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != summonsToCitadelNPC || script == nil || script.ID != summonsToCitadelQuestID {
		return false
	}
	quest := c.player.quest(summonsToCitadelQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "START" {
		if dialogID != 25 {
			return false
		}
		if c.customQuestProgress(summonsToCitadelQuestID, setQuestVar(quest.Vars, 0, 1), "REWARD") {
			c.send(dialogWindow(o.id, 1011, summonsToCitadelQuestID))
			return true
		}
		return false
	}
	if quest.Status != "REWARD" {
		return false
	}
	switch dialogID {
	case 17:
		c.summonsToCitadelLockFollowups()
		c.finishQuest(script, o.id, dialogID)
	case 1009, ^uint16(0):
		c.send(dialogWindow(o.id, 5, summonsToCitadelQuestID))
	default:
		return false
	}
	return true
}

func (c *conn) summonsToCitadelLockFollowups() {
	for id := int32(1011); id <= 1023; id++ {
		if c.player.quest(id) != nil || c.s.data.Quests[id] == nil {
			continue
		}
		locked := store.Quest{ID: id, Status: "LOCKED"}
		if err := c.s.quests.SaveQuest(c.player.ID, locked); err != nil {
			c.s.log.Error("locking Verteron follow-up quest", "quest", id, "err", err)
			continue
		}
		c.player.quests = append(c.player.quests, locked)
		c.send(questAccepted(1, locked))
	}
}
