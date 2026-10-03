package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// javaLevelUpCreates are the handlers whose Java onLvlUpEvent creates the quest once the player reaches its minimum
// level (qs == null, checkLevelRequirement, QuestService.startQuest). The campaign chains are not here: their Java
// onLvlUpEvent only unlocks a LOCKED row that an orders quest (1100, 1130, 1300, ...) created.
var javaLevelUpCreates = []int32{1006, 1007, 1205, 1913, 1914, 1915, 1916, 1929, 2008, 2009, 2098, 2132, 2900, 2901, 2902, 2903, 2904}

// javaSkillQuestVars are _1205ANewSkill / _2132ANewSkill: started straight into REWARD with the starting class's var.
var javaSkillQuestVars = map[string]int32{"WARRIOR": 1, "SCOUT": 2, "MAGE": 3, "PRIEST": 4}

// levelUpStartQuests is QuestEngine.onLvlUp for javaLevelUpCreates.
func (c *conn) levelUpStartQuests() {
	if c == nil || c.player == nil {
		return
	}
	for _, id := range javaLevelUpCreates {
		script := c.s.data.QuestScripts[id]
		if script == nil {
			continue
		}
		if (id == 1006 || id == 1007 || id == 2008 || id == 2009) && c.s.currentConfig().SimpleSecondClass {
			continue
		}
		// _1007ACeremonyinSanctum / _2009ACeremonyinPandaemonium: only after the ascension quest.
		if prev := map[int32]int32{sanctumCeremonyQuestID: ascensionQuestID, 2009: 2008}[id]; prev != 0 {
			ascension := c.player.quest(prev)
			if ascension == nil || ascension.Status != "COMPLETE" {
				continue
			}
		}
		template := c.s.data.Quests[id]
		if c.player.quest(id) != nil || template == nil || c.player.level < template.MinLevel || !c.beginQuest(script) {
			continue
		}
		if id == 1205 || id == 2132 {
			c.updateQuest(id, func(q *store.Quest) { q.Status, q.Vars = "REWARD", javaSkillQuestVars[c.player.Class] })
		}
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
