package game

import (
	"time"

	"aionlightning/game/data"
)

const (
	mandurisSecretQuestID int32 = 1031
	mandurisAureliusNPC   int32 = 203902
	mandurisArchelaosNPC  int32 = 203936
	mandurisGliderNPC     int32 = 700179
	mandurisMelginieNPC   int32 = 204043
	mandurisCelestineNPC  int32 = 204030
)

func (c *conn) mandurisSecretLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(mandurisSecretQuestID)
	template := c.s.data.Quests[mandurisSecretQuestID]
	if quest == nil || template == nil || quest.Status != "LOCKED" || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking The Manduri's Secret", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) mandurisSecretKill(npcID int32) bool {
	if c == nil || c.player == nil {
		return false
	}
	switch npcID {
	case 210770, 210771, 210759, 210758:
	default:
		return false
	}
	quest := c.player.quest(mandurisSecretQuestID)
	if quest == nil || quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if variable < 1 || variable > 6 {
		return false
	}
	return c.customQuestProgress(mandurisSecretQuestID, setQuestVar(quest.Vars, 0, variable+1), "")
}

func (c *conn) mandurisSecretDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != mandurisSecretQuestID {
		return false
	}
	quest := c.player.quest(mandurisSecretQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "REWARD" {
		if o.npc.ID != mandurisAureliusNPC {
			return false
		}
		switch {
		case dialogID == -1:
			c.send(dialogWindow(o.id, 3398, mandurisSecretQuestID))
			return true
		case dialogID == 1009:
			c.send(dialogWindow(o.id, 5, mandurisSecretQuestID))
			return true
		case dialogID >= 8 && dialogID <= 11:
			c.finishQuestReward(script, o.id, uint16(dialogID), 0)
			return true
		default:
			return false
		}
	}
	if quest.Status != "START" {
		return false
	}
	switch o.npc.ID {
	case mandurisAureliusNPC:
		switch dialogID {
		case -1, 25:
			switch variable {
			case 0:
				c.send(dialogWindow(o.id, 1011, mandurisSecretQuestID))
				return true
			case 7:
				c.send(dialogWindow(o.id, 1352, mandurisSecretQuestID))
				return true
			}
		case 10000:
			if variable == 0 && c.customQuestProgress(mandurisSecretQuestID, setQuestVar(quest.Vars, 0, 1), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		case 10001:
			if variable == 7 && c.customQuestProgress(mandurisSecretQuestID, setQuestVar(quest.Vars, 0, 8), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
	case mandurisArchelaosNPC:
		if dialogID == 25 && variable == 8 {
			c.send(dialogWindow(o.id, 1693, mandurisSecretQuestID))
			return true
		}
		if dialogID == 10002 && variable == 8 && c.customQuestProgress(mandurisSecretQuestID, setQuestVar(quest.Vars, 0, 9), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case mandurisGliderNPC:
		if dialogID == -1 && variable == 9 {
			return c.mandurisSecretUseGlider(o)
		}
	case mandurisMelginieNPC:
		if dialogID == 25 && variable == 10 {
			c.send(dialogWindow(o.id, 2375, mandurisSecretQuestID))
			return true
		}
		if dialogID == 10004 && variable == 10 && c.customQuestProgress(mandurisSecretQuestID, setQuestVar(quest.Vars, 0, 12), "") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	case mandurisCelestineNPC:
		if dialogID == 25 && variable == 12 {
			c.send(dialogWindow(o.id, 3057, mandurisSecretQuestID))
			return true
		}
		if dialogID == 10006 && variable == 12 && c.customQuestProgress(mandurisSecretQuestID, quest.Vars, "REWARD") {
			c.send(dialogWindow(o.id, 10, 0))
			return true
		}
	}
	return false
}

func (c *conn) mandurisSecretUseGlider(o *object) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || o.useTask != nil || c.player.targetID != o.id {
		return false
	}
	p := c.player
	quest := p.quest(mandurisSecretQuestID)
	if quest == nil || quest.Status != "START" || questVar(quest.Vars, 0) != 9 {
		return false
	}
	if !c.customQuestProgress(mandurisSecretQuestID, setQuestVar(quest.Vars, 0, 10), "") {
		return false
	}
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(c.s.playerEmotionTo(p, emoteNeutralMode2, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = c.s.later(3*time.Second, func() {
		o.useTask = nil
		if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead || p.targetID != o.id {
			return
		}
		current := p.quest(mandurisSecretQuestID)
		if current == nil || current.Status != "START" || questVar(current.Vars, 0) != 10 {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(c.s.playerEmotionTo(p, emoteStartLoot, 0, o.id, 0, 0, 0, 0), true)
	})
	c.dialogNotHandled()
	return false
}
