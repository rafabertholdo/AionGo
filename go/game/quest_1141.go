package game

import "aionlightning/game/data"

const (
	belbuasTreasureQuestID int32 = 1141
	belbuasTreasureNola    int32 = 730001
	belbuasTreasureBarrel  int32 = 700122
)

// belbuasTreasureDialog ports Nola's quest offer and the one-click wine
// barrel discovery that completes Belbua's Treasure.
func (c *conn) belbuasTreasureDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != belbuasTreasureQuestID {
		return false
	}
	quest := c.player.quest(belbuasTreasureQuestID)
	if quest == nil || quest.Status == "NONE" {
		if o.npc.ID == belbuasTreasureNola {
			return c.customQuestStart(o, script, uint16(dialogID), 0)
		}
		return false
	}
	if o.npc.ID != belbuasTreasureBarrel {
		return false
	}
	// Java (l.71-81): var 0 and a plain click -> REWARD (var 2) and defaultQuestEndDialog's window 5; anything else
	// first sends window 10 (the else has no braces, so it only guards that packet) and then falls into
	// defaultQuestEndDialog, which answers -1/1009 (REWARD only) with window 5 and 8-17 with the reward.
	if questVar(quest.Vars, 0) == 0 && dialogID == -1 {
		if quest.Status == "START" && !c.customQuestProgress(belbuasTreasureQuestID, 2, "REWARD") {
			return false
		}
		c.send(dialogWindow(o.id, 5, belbuasTreasureQuestID))
		return true
	}
	c.send(dialogWindow(o.id, 10, 0))
	if quest.Status != "REWARD" {
		c.dialogNotHandled() // Java returns false after the window 10: the echo follows
		return false
	}
	switch {
	case dialogID == -1 || dialogID == 1009:
		c.send(dialogWindow(o.id, 5, belbuasTreasureQuestID))
		return true
	case dialogID >= 8 && dialogID <= 17:
		c.finishQuest(script, o.id, uint16(dialogID))
		return quest.Status == "COMPLETE"
	}
	c.dialogNotHandled()
	return false
}
