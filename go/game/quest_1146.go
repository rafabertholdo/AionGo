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
			template := c.s.data.Quests[delicateMandrakeQuestID]
			if template == nil || player.level < template.MinLevel || !c.s.canStartQuest(player, script) {
				return false
			}
			if !c.s.questRewardsFit(player, []data.QuestItem{{ID: delicateMandrakeWorkItemID, Count: 1}}) {
				c.send(systemMessage(msgInventoryFull))
				return true
			}
			c.startQuest(script, o.id)
			quest = player.quest(delicateMandrakeQuestID)
			if quest == nil || quest.Status != "START" {
				return false
			}
			if !c.s.addItem(player, delicateMandrakeWorkItemID, 1) {
				return false
			}
			c.send(delicateMandrakeTimerPacket(delicateMandrakeTimerSeconds))
			c.s.later(time.Duration(delicateMandrakeTimerSeconds)*time.Second, func() {
				c.delicateMandrakeTimerEnd()
			})
			return true
		default:
			return false
		}
	}
	if o.npc.ID != delicateMandrakeEndNPCID {
		return false
	}
	if quest.Status == "START" {
		if dialogID != -1 || questVar(quest.Vars, 0) != 0 {
			return false
		}
		if c.s.countItems(player, delicateMandrakeTurnInItemID) == 0 {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
		if !c.customQuestProgress(delicateMandrakeQuestID, setQuestVar(quest.Vars, 0, 2), "REWARD") {
			return false
		}
		c.s.removeItemsByID(player, delicateMandrakeTurnInItemID, 1)
		c.send(delicateMandrakeTimerPacket(0))
		c.send(dialogWindow(o.id, 5, delicateMandrakeQuestID))
		return true
	}
	if quest.Status != "REWARD" {
		return false
	}
	switch {
	case dialogID == -1 || dialogID == 1009:
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
