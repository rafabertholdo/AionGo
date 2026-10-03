package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	altgardDutiesQuestID         int32  = 2200
	altgardDutiesMapID           int32  = 220030000
	altgardDutiesZone            string = "ALTGARD_FORTRESS_220030000"
	altgardDutiesEndNPC          int32  = 203557
	morheimCommandersCallQuestID int32  = 2300
	morheimCommandersCallMapID   int32  = 220020000
	morheimCommandersCallZone    string = "MORHEIM_ICE_FORTRESS_220020000"
	morheimCommandersCallEndNPC  int32  = 204301
)

func (c *conn) altgardDutiesEnterZone(zoneName string) bool {
	return c.startCampaignQuestOnZone(altgardDutiesQuestID, altgardDutiesMapID, altgardDutiesZone, zoneName)
}

func (c *conn) morheimCommandersCallEnterZone(zoneName string) bool {
	return c.startCampaignQuestOnZone(morheimCommandersCallQuestID, morheimCommandersCallMapID, morheimCommandersCallZone, zoneName)
}

func (c *conn) startCampaignQuestOnZone(questID, mapID int32, expectedZone, zoneName string) bool {
	if c == nil || c.player == nil || c.player.WorldID != mapID || zoneName != expectedZone || c.player.quest(questID) != nil {
		return false
	}
	script := c.s.data.QuestScripts[questID]
	template := c.s.data.Quests[questID]
	if script == nil || template == nil || c.player.level < template.MinLevel || !c.s.canStartQuest(c.player, script) {
		return false
	}
	started := store.Quest{ID: questID, Status: "START"}
	if err := c.s.quests.SaveQuest(c.player.ID, started); err != nil {
		c.s.log.Error("starting zone campaign quest", "quest", questID, "err", err)
		return false
	}
	c.player.quests = append(c.player.quests, started)
	c.send(questAccepted(1, started))
	c.send(c.s.nearbyQuests(c.player))
	return true
}

// morheimCommandersCallLevelUp unlocks the campaign step created as LOCKED by
// the preceding level range. Java listens for the level-up event, not a fresh quest start.
func (c *conn) morheimCommandersCallLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(morheimCommandersCallQuestID)
	template := c.s.data.Quests[morheimCommandersCallQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Morheim Commander's Call", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) altgardDutiesDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	return c.campaignQuestDialog(o, script, dialogID, altgardDutiesQuestID, altgardDutiesEndNPC, rangeQuestIDs(2011, 2022))
}

func (c *conn) morheimCommandersCallDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	return c.campaignQuestDialog(o, script, dialogID, morheimCommandersCallQuestID, morheimCommandersCallEndNPC, rangeQuestIDs(2031, 2042))
}

func rangeQuestIDs(first, last int32) []int32 {
	ids := make([]int32, 0, last-first+1)
	for id := first; id <= last; id++ {
		ids = append(ids, id)
	}
	return ids
}

func (c *conn) campaignQuestDialog(o *object, script *data.QuestScript, dialogID int32, questID, endNPC int32, followUps []int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != questID || o.npc.ID != endNPC {
		return false
	}
	quest := c.player.quest(questID)
	if quest == nil {
		return false
	}
	switch quest.Status {
	case "START":
		if dialogID == 25 {
			if !c.customQuestProgress(questID, setQuestVar(quest.Vars, 0, 1), "REWARD") {
				return false
			}
			c.send(dialogWindow(o.id, 1011, questID))
			return true
		}
		switch dialogID {
		case 1007:
			c.send(dialogWindow(o.id, 4, questID))
			return true
		case 1003:
			c.send(dialogWindow(o.id, 1004, questID))
			return true
		default:
			return false
		}
	case "REWARD":
		if dialogID == 17 && !c.lockQuestIDs(followUps) {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, questID))
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuest(script, o.id, uint16(dialogID))
		default:
			return false
		}
		return true
	default:
		return false
	}
}

// lockQuestIDs mirrors QuestService.startQuest(..., LOCKED), including its
// behavior of retaining any already started or completed quest.
func (c *conn) lockQuestIDs(ids []int32) bool {
	if c == nil || c.player == nil {
		return false
	}
	for _, id := range ids {
		if existing := c.player.quest(id); existing != nil && existing.Status != "NONE" {
			continue
		}
		locked := store.Quest{ID: id, Status: "LOCKED"}
		if err := c.s.quests.SaveQuest(c.player.ID, locked); err != nil {
			c.s.log.Error("locking follow-up quest", "quest", id, "err", err)
			return false
		}
		if existing := c.player.quest(id); existing == nil {
			c.player.quests = append(c.player.quests, locked)
		} else {
			*existing = locked
		}
		c.send(questAccepted(1, locked))
	}
	return true
}
