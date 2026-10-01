package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	headlessStoneStatueQuestID      int32  = 1170
	headlessStoneStatueBodyNPCID    int32  = 730000
	headlessStoneStatueHeadNPCID    int32  = 700033
	headlessStoneStatueWorkItemID   int32  = 182200504
	headlessStoneStatueMovieID      uint16 = 16
	headlessStoneStatueRewardExp    int64  = 8410
)

func (c *conn) headlessStoneStatueDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != headlessStoneStatueQuestID {
		return false
	}
	quest := c.player.quest(headlessStoneStatueQuestID)
	if quest == nil || quest.Status == "NONE" {
		if o.npc.ID != headlessStoneStatueBodyNPCID {
			return false
		}
		c.headlessStoneStatueStart(script)
		// Java sends page 1011 with object id zero and returns false. The dialog
		// framework therefore also echoes the original dialog id.
		c.send(dialogWindow(0, 1011, headlessStoneStatueQuestID))
		return false
	}

	switch quest.Status {
	case "START":
		if o.npc.ID != headlessStoneStatueHeadNPCID || dialogID != -1 {
			return false
		}
		if o.useTask != nil {
			return false
		}
		p := c.player
		p.broadcast(c.s.playerEmotionTo(p, emoteSit, 0, o.id, 0, 0, 0, 0), true)
		o.useTask = c.s.later(3*time.Second, func() {
			o.useTask = nil
			if p.targetID != o.id {
				return
			}
			if !c.s.questRewardsFit(p, []data.QuestItem{{ID: headlessStoneStatueWorkItemID, Count: 1}}) || !c.s.addItem(p, headlessStoneStatueWorkItemID, 1) {
				return
			}
			c.s.despawnNpc(o, true)
			current := p.quest(headlessStoneStatueQuestID)
			if current == nil || current.Status != "START" {
				return
			}
			c.customQuestProgress(headlessStoneStatueQuestID, current.Vars, "REWARD")
		})
		return false
	case "REWARD":
		if o.npc.ID != headlessStoneStatueBodyNPCID || c.s.countItems(c.player, headlessStoneStatueWorkItemID) == 0 {
			return false
		}
		c.s.removeItemsByID(c.player, headlessStoneStatueWorkItemID, c.s.countItems(c.player, headlessStoneStatueWorkItemID))
		c.send(playMovie(headlessStoneStatueMovieID))
		return true
	default:
		return false
	}
}

func (c *conn) headlessStoneStatueStart(script *data.QuestScript) {
	p := c.player
	template := c.s.data.Quests[headlessStoneStatueQuestID]
	if template == nil || !c.s.canStartQuest(p, script) || p.level < template.MinLevel {
		return
	}
	started := store.Quest{ID: headlessStoneStatueQuestID, Status: "START"}
	quest := p.quest(headlessStoneStatueQuestID)
	if quest != nil {
		started.CompleteCount = quest.CompleteCount
	}
	if err := c.s.quests.SaveQuest(p.ID, started); err != nil {
		c.s.log.Error("starting Headless Stone Statue", "character", p.Name, "err", err)
		return
	}
	if quest == nil {
		p.quests = append(p.quests, started)
	} else {
		*quest = started
	}
	c.send(questAccepted(1, started))
	c.send(c.s.nearbyQuests(p))
}

func (c *conn) headlessStoneStatueMovieEnd(movieID uint16) bool {
	if c == nil || c.player == nil || movieID != headlessStoneStatueMovieID {
		return false
	}
	p := c.player
	quest := p.quest(headlessStoneStatueQuestID)
	if quest == nil || quest.Status != "REWARD" {
		return false
	}
	c.s.giveExp(p, headlessStoneStatueRewardExp)
	complete := *quest
	complete.Status = "COMPLETE"
	complete.CompleteCount = 1
	if err := c.s.quests.SaveQuest(p.ID, complete); err != nil {
		c.s.log.Error("completing Headless Stone Statue", "character", p.Name, "err", err)
		return false
	}
	*quest = complete
	c.send(questAccepted(2, complete))
	c.send(c.s.nearbyQuests(p))
	finalUpdate := complete
	finalUpdate.Vars = 2
	c.send(questAccepted(2, finalUpdate))
	return true
}
