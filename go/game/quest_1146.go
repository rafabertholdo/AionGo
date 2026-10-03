package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

const (
	delicateMandrakeQuestID      int32 = 1146
	delicateMandrakeStartNPCID   int32 = 203123
	delicateMandrakeEndNPCID     int32 = 203139
	delicateMandrakeWorkItemID   int32 = 182200519
	delicateMandrakeTurnInItemID int32 = 182200011
	delicateMandrakeTimerSeconds       = 900
)

func delicateMandrakeTimerPacket(seconds int32) *wire.Writer {
	w := wire.Packet(smQuestAccepted)
	w.C(4)
	w.D(delicateMandrakeQuestID)
	w.D(seconds)
	w.C(1)
	w.H(0)
	w.C(1)
	return w
}

func (c *conn) delicateMandrakeDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != delicateMandrakeQuestID {
		return false
	}
	if o.npc.ID != delicateMandrakeStartNPCID && o.npc.ID != delicateMandrakeEndNPCID {
		return false
	}
	player := c.player
	quest := player.quest(delicateMandrakeQuestID)
	if quest == nil || quest.Status == "NONE" {
		if o.npc.ID != delicateMandrakeStartNPCID {
			return false
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 1011, delicateMandrakeQuestID))
			return true
		case 1007:
			c.send(dialogWindow(o.id, 4, delicateMandrakeQuestID))
			return true
		case 1003:
			c.send(dialogWindow(o.id, 1004, delicateMandrakeQuestID))
			return true
		case 1002:
			// Java: the work item (its result ignored) and the timer come first, whether or not the quest starts.
			c.addQuestItems([]data.QuestItem{{ID: delicateMandrakeWorkItemID, Count: 1}})
			c.send(delicateMandrakeTimerPacket(delicateMandrakeTimerSeconds))
			c.s.later(time.Duration(delicateMandrakeTimerSeconds)*time.Second, func() {
				c.delicateMandrakeTimerEnd()
			})
			return c.startQuest(script, o.id)
		default:
			return false
		}
	}
	if o.npc.ID != delicateMandrakeEndNPCID {
		return false
	}
	// Java checks only the variable here, in any status.
	if dialogID == -1 && questVar(quest.Vars, 0) == 0 {
		if c.jRemoveAll(delicateMandrakeTurnInItemID) {
			quest.Vars = setQuestVar(quest.Vars, 0, 2)
			c.send(delicateMandrakeTimerPacket(0)) // QuestService.questTimerEnd
			quest.Status = "REWARD"
			c.jSave(quest)
			c.jUpdate(quest)
		} else {
			c.send(dialogWindow(o.id, 10, 0))
		}
		return c.defaultQuestEndDialog(o, script, -1)
	}
	// Java returns false for everything else, so its reward can never be taken (8-17 is echoed). Go keeps the turn-in
	// working: 1009 shows the reward, 17 finishes (see parityDeviation).
	if quest.Status != "REWARD" {
		return false
	}
	switch {
	case dialogID == 1009:
		c.send(dialogWindow(o.id, 5, delicateMandrakeQuestID))
	case dialogID == 17:
		c.finishQuest(script, o.id, uint16(dialogID))
	default:
		return false
	}
	return true
}

// delicateMandrakeTimerEnd mirrors QuestEngine.deleteQuest after the source
// timer expires. The Java handler never registers its timer-end callback.
func (c *conn) delicateMandrakeTimerEnd() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(delicateMandrakeQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	next := *quest
	next.Status = "NONE"
	next.Vars = 0
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("expiring Delicate Mandrake", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(3, store.Quest{ID: delicateMandrakeQuestID}))
	c.send(c.s.nearbyQuests(c.player))
	return true
}
