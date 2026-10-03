package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	ordersFromTelemachusQuestID int32  = 1300
	ordersFromTelemachusNPCID   int32  = 203901
	ordersFromTelemachusZone    string = "ELTNEN_FORTRESS"
)

var ordersFromTelemachusFollowUps = []int32{
	1031, 1032, 1033, 1034, 1035, 1036, 1037, 1038, 1039, 1040, 1041, 1042, 1043,
}

// ordersFromTelemachusLevelUp changes a pre-existing locked state to START,
// matching the Java onLvlUp handler.
func (c *conn) ordersFromTelemachusLevelUp() bool {
	if c == nil || c.player == nil || c.s == nil {
		return false
	}
	quest := c.player.quest(ordersFromTelemachusQuestID)
	template := c.s.data.Quests[ordersFromTelemachusQuestID]
	if quest == nil || quest.Status != "LOCKED" || template == nil || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Orders from Telemachus", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

// ordersFromTelemachusEnterZone starts the quest when the player enters the
// Eltnen Fortress zone and has no quest state.
func (c *conn) ordersFromTelemachusEnterZone(zoneName string) bool {
	if c == nil || c.player == nil || c.s == nil || c.player.WorldID != 210020000 || zoneName != ordersFromTelemachusZone || c.player.quest(ordersFromTelemachusQuestID) != nil {
		return false
	}
	script := c.s.data.QuestScripts[ordersFromTelemachusQuestID]
	template := c.s.data.Quests[ordersFromTelemachusQuestID]
	if script == nil || template == nil || c.player.level < template.MinLevel || !c.s.canStartQuest(c.player, script) {
		return false
	}
	started := store.Quest{ID: ordersFromTelemachusQuestID, Status: "START"}
	if err := c.s.quests.SaveQuest(c.player.ID, started); err != nil {
		c.s.log.Error("starting Orders from Telemachus", "err", err)
		return false
	}
	c.player.quests = append(c.player.quests, started)
	c.send(questAccepted(1, started))
	c.send(c.s.nearbyQuests(c.player))
	return true
}

func (c *conn) ordersFromTelemachusDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || c.s == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != ordersFromTelemachusNPCID || script == nil || script.ID != ordersFromTelemachusQuestID {
		return false
	}
	quest := c.player.quest(ordersFromTelemachusQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "START" {
		if dialogID == 25 {
			if !c.customQuestProgress(ordersFromTelemachusQuestID, setQuestVar(quest.Vars, 0, 1), "REWARD") {
				return false
			}
			c.send(dialogWindow(o.id, 1011, ordersFromTelemachusQuestID))
			return true
		}
		switch dialogID {
		case 1007:
			c.send(dialogWindow(o.id, 4, ordersFromTelemachusQuestID))
			return true
		case 1003:
			c.send(dialogWindow(o.id, 1004, ordersFromTelemachusQuestID))
			return true
		default:
			return false
		}
	}
	if quest.Status != "REWARD" {
		return false
	}
	if dialogID == 17 {
		c.ordersFromTelemachusLockFollowUps()
		c.finishQuest(script, o.id, uint16(dialogID))
		return true
	}
	if dialogID == -1 || dialogID == 1009 {
		c.send(dialogWindow(o.id, 5, ordersFromTelemachusQuestID))
		return true
	}
	if dialogID >= 8 && dialogID <= 16 {
		c.finishQuest(script, o.id, uint16(dialogID))
		return true
	}
	return false
}

func (c *conn) ordersFromTelemachusLockFollowUps() {
	for _, id := range ordersFromTelemachusFollowUps {
		if c.player.quest(id) != nil || c.s.data.Quests[id] == nil {
			continue
		}
		locked := store.Quest{ID: id, Status: "LOCKED"}
		if err := c.s.quests.SaveQuest(c.player.ID, locked); err != nil {
			c.s.log.Error("locking Orders from Telemachus follow-up", "quest", id, "err", err)
			continue
		}
		c.player.quests = append(c.player.quests, locked)
		c.send(questAccepted(1, locked))
	}
	c.send(c.s.nearbyQuests(c.player))
}
