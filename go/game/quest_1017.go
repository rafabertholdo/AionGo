package game

import "aionlightning/game/data"

const (
	heldSacredQuestID int32 = 1017
	heldSacredNPCID   int32 = 203178
	heldSacredItemID  int32 = 182200025
)

// heldSacredLevelUp ports Held Sacred's Java level-up event, which changes
// the pre-created LOCKED quest into START once the character reaches level 13.
func (c *conn) heldSacredLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	template := c.s.data.Quests[heldSacredQuestID]
	quest := c.player.quest(heldSacredQuestID)
	if template == nil || quest == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Held Sacred", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

// heldSacredDialog ports Sister Rina's conversation, five-item turn-in, and
// selectable reward. Quest drops are supplied by the quest drop data.
func (c *conn) heldSacredDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != heldSacredQuestID || o.npc.ID != heldSacredNPCID {
		return false
	}
	quest := c.player.quest(heldSacredQuestID)
	if quest == nil {
		return false
	}
	if quest.Status == "REWARD" {
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, heldSacredQuestID))
		case dialogID >= 8 && dialogID <= 11:
			c.finishQuestReward(script, o.id, uint16(dialogID), 0)
		default:
			return false
		}
		return true
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	switch dialogID {
	case 25:
		if variable == 0 {
			c.send(dialogWindow(o.id, 1011, heldSacredQuestID))
			return true
		}
		if variable == 1 {
			c.send(dialogWindow(o.id, 1352, heldSacredQuestID))
			return true
		}
	case 10000:
		if variable == 0 && c.customQuestProgress(heldSacredQuestID, setQuestVar(quest.Vars, 0, 1), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case 33:
		if variable != 1 {
			return false
		}
		template := c.s.data.Quests[heldSacredQuestID]
		if !c.s.hasQuestItems(c.player, template) {
			c.send(dialogWindow(o.id, 1353, heldSacredQuestID))
			return true
		}
		if !c.customQuestProgress(heldSacredQuestID, setQuestVar(quest.Vars, 0, variable+1), "REWARD") {
			return false
		}
		for _, required := range template.CollectItems {
			c.s.removeItemsByID(c.player, required.ID, required.Count)
		}
		c.send(dialogWindow(o.id, 5, heldSacredQuestID))
		return true
	}
	return false
}
