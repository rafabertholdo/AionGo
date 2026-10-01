package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	atroposRequestQuestID int32  = 1091
	atroposRequestNPCID   int32  = 798155
	atroposRequestMapID   int32  = 210060000
	atroposRequestZone    string = "Q1091"
)

func (c *conn) atroposRequestEnterZone(zoneName string) bool {
	if c == nil || c.player == nil || zoneName != atroposRequestZone || c.player.WorldID != atroposRequestMapID || c.player.quest(atroposRequestQuestID) != nil {
		return false
	}
	questScript := c.s.data.QuestScripts[atroposRequestQuestID]
	template := c.s.data.Quests[atroposRequestQuestID]
	if questScript == nil || template == nil || !c.s.canStartQuest(c.player, questScript) || c.player.level < template.MinLevel {
		return false
	}
	started := store.Quest{ID: atroposRequestQuestID, Status: "START"}
	if err := c.s.quests.SaveQuest(c.player.ID, started); err != nil {
		c.s.log.Error("starting A Request From Atropos in zone", "err", err)
		return false
	}
	c.player.quests = append(c.player.quests, started)
	c.send(questAccepted(1, started))
	c.send(c.s.nearbyQuests(c.player))
	return true
}

func (c *conn) atroposRequestDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.npc.ID != atroposRequestNPCID || script == nil || script.ID != atroposRequestQuestID {
		return false
	}
	quest := c.player.quest(atroposRequestQuestID)
	if quest == nil {
		return false
	}
	switch quest.Status {
	case "START":
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 10002, atroposRequestQuestID))
			return true
		case 1009:
			if !c.customQuestProgress(atroposRequestQuestID, setQuestVar(quest.Vars, 0, 1), "REWARD") {
				return false
			}
			c.send(dialogWindow(o.id, 5, atroposRequestQuestID))
			return true
		}
	case "REWARD":
		if dialogID == 17 {
			for _, childQuestID := range []int32{1092, 1093, 1094} {
				c.atroposRequestLockFollowUp(childQuestID)
			}
		}
		switch dialogID {
		case -1, 1009:
			c.send(dialogWindow(o.id, 5, atroposRequestQuestID))
			return true
		case 17:
			c.finishQuestReward(script, o.id, uint16(dialogID), 0)
			return true
		}
	}
	return false
}

func (c *conn) atroposRequestLockFollowUp(questID int32) {
	template := c.s.data.Quests[questID]
	if template == nil {
		return
	}
	locked := store.Quest{ID: questID, Status: "LOCKED"}
	existing := c.player.quest(questID)
	if existing == nil {
		if err := c.s.quests.SaveQuest(c.player.ID, locked); err != nil {
			c.s.log.Error("locking Atropos follow-up", "quest", questID, "err", err)
			return
		}
		c.player.quests = append(c.player.quests, locked)
	} else if template.MaxRepeatCount >= int(existing.CompleteCount) {
		locked.CompleteCount = existing.CompleteCount
		if err := c.s.quests.SaveQuest(c.player.ID, locked); err != nil {
			c.s.log.Error("resetting Atropos follow-up", "quest", questID, "err", err)
			return
		}
		*existing = locked
	}
	c.send(questAccepted(1, locked))
	c.send(c.s.nearbyQuests(c.player))
}
