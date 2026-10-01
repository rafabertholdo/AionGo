package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

var newSkillClasses = [...]string{"WARRIOR", "SCOUT", "MAGE", "PRIEST"}

func newSkillNPCs(id int32) [4]int32 {
	if id == 1205 {
		return [4]int32{203087, 203088, 203089, 203090}
	}
	return [4]int32{203527, 203528, 203529, 203530}
}

// newSkillLevelUp ports the paired Elyos and Asmodian level-three handlers.
func (c *conn) newSkillLevelUp(id int32) bool {
	if c.player == nil || c.player.quest(id) != nil {
		return false
	}
	script := c.s.data.QuestScripts[id]
	template := c.s.data.Quests[id]
	if script == nil || template == nil || c.player.level < template.MinLevel || !c.s.canStartQuest(c.player, script) {
		return false
	}
	index := -1
	for at, class := range newSkillClasses {
		if c.player.Class == class {
			index = at
			break
		}
	}
	if index < 0 {
		return false
	}
	started := store.Quest{ID: id, Status: "START"}
	if err := c.s.quests.SaveQuest(c.player.ID, started); err != nil {
		c.s.log.Error("starting A New Skill", "quest", id, "err", err)
		return false
	}
	c.player.quests = append(c.player.quests, started)
	c.send(questAccepted(1, started))
	reward := started
	reward.Status = "REWARD"
	reward.Vars = int32(index + 1)
	if err := c.s.quests.SaveQuest(c.player.ID, reward); err != nil {
		c.s.log.Error("rewarding A New Skill", "quest", id, "err", err)
		return false
	}
	*c.player.quest(id) = reward
	c.send(questAccepted(2, reward))
	return true
}

func (c *conn) newSkillDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c.player == nil || o == nil || o.npc == nil || script == nil || (script.ID != 1205 && script.ID != 2132) {
		return false
	}
	q := c.player.quest(script.ID)
	if q == nil || q.Status != "REWARD" {
		return false
	}
	npcs := newSkillNPCs(script.ID)
	for index, npcID := range npcs {
		if o.npc.ID != npcID || c.player.Class != newSkillClasses[index] || q.Vars != int32(index+1) {
			continue
		}
		switch {
		case dialogID == -1:
			c.send(dialogWindow(o.id, uint16(1011+341*index), script.ID))
		case dialogID == 1009:
			c.send(dialogWindow(o.id, uint16(5+index), script.ID))
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuestReward(script, o.id, 17, 0)
		default:
			return false
		}
		return true
	}
	return false
}
