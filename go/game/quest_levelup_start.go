package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// levelUpStartQuests ports Java's QuestEngine.onLvlUp registrations for
// handlers that create a quest as soon as its minimum level is reached.
func (c *conn) levelUpStartQuests() {
	if c == nil || c.player == nil {
		return
	}
	for _, script := range c.s.data.QuestScripts {
		if script.ID == josnackDilemmaQuestID || script.ID == pearlOfProtectionQuestID {
			continue
		}
		if (script.ID == 1006 || script.ID == 1007) && c.s.config.SimpleSecondClass {
			continue
		}
		if script.ID == sanctumCeremonyQuestID {
			ascension := c.player.quest(ascensionQuestID)
			if ascension == nil || ascension.Status != "COMPLETE" {
				continue
			}
		}
		if !script.LevelUpStart || c.player.quest(script.ID) != nil {
			continue
		}
		template := c.s.data.Quests[script.ID]
		if template == nil || c.player.level < template.MinLevel || !c.s.canStartQuest(c.player, script) {
			continue
		}
		started := store.Quest{ID: script.ID, Status: "START"}
		if err := c.s.quests.SaveQuest(c.player.ID, started); err != nil {
			c.s.log.Error("starting level-up quest", "quest", script.ID, "err", err)
			continue
		}
		c.player.quests = append(c.player.quests, started)
		c.send(questAccepted(1, started))
		c.send(c.s.nearbyQuests(c.player))
	}
}

// levelUpQuestDialog ports the common dispatch quest dialog used by 2901-2904
// and 1913-1916. Each source handler differs only in its XML constraints and
// the destination metadata in QuestScript.
func (c *conn) levelUpQuestDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || script == nil || !script.LevelUpStart {
		return false
	}
	q := c.player.quest(script.ID)
	if q == nil {
		return false
	}
	if q.Status == "REWARD" {
		if o.npc.ID != script.EndNPC {
			return false
		}
		switch {
		case dialogID == -1 || dialogID == 1009:
			c.send(dialogWindow(o.id, 5, script.ID))
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuest(script, o.id, uint16(dialogID))
		default:
			return false
		}
		return true
	}
	if q.Status != "START" {
		return false
	}
	variable := questVar(q.Vars, 0)
	if o.npc.ID == script.LevelUpNPC {
		if variable != 0 {
			return false
		}
		switch dialogID {
		case -1, 25:
			c.send(dialogWindow(o.id, 1352, script.ID))
			return true
		case 10000:
			if !c.customQuestProgress(script.ID, setQuestVar(q.Vars, 0, 1), "") {
				return false
			}
			c.s.teleportToInstance(c.player, script.LevelUpWorld, c.player.instance, script.LevelUpX, script.LevelUpY, script.LevelUpZ, byte(c.player.Heading), time.Duration(script.LevelUpDelayMS)*time.Millisecond)
			c.send(dialogWindow(o.id, 0, 0))
			return true
		}
		return false
	}
	if o.npc.ID == script.EndNPC && dialogID == 25 && variable == 1 {
		if c.customQuestProgress(script.ID, q.Vars, "REWARD") {
			c.send(dialogWindow(o.id, 2375, script.ID))
			return true
		}
	}
	return false
}
