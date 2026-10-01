package game

import (
	"aionlightning/game/data"
	"aionlightning/wire"
)

const sanctumCeremonyQuestID int32 = 1007

func (c *conn) sanctumCeremonyDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || script == nil || script.ID != sanctumCeremonyQuestID {
		return false
	}
	quest := c.player.quest(sanctumCeremonyQuestID)
	if quest == nil {
		return false
	}
	variable := questVar(quest.Vars, 0)
	if quest.Status == "START" {
		switch o.npc.ID {
		case 790001:
			if dialogID == 25 && variable == 0 {
				c.send(dialogWindow(o.id, 1011, sanctumCeremonyQuestID))
				return true
			}
			if dialogID == 10000 && variable == 0 {
				if !c.customQuestProgress(sanctumCeremonyQuestID, setQuestVar(quest.Vars, 0, 1), "") {
					return false
				}
				c.send(dialogWindow(o.id, 0, 0))
				c.send(sanctumCeremonyTeleportLocation())
				c.s.teleportTo(c.player, 110010000, 1313, 1512, 568, byte(c.player.Heading), teleportDefaultDelay)
				return true
			}
		case 203725:
			if dialogID == 25 && variable == 1 {
				c.send(dialogWindow(o.id, 1352, sanctumCeremonyQuestID))
				return true
			}
			if dialogID == 1353 && variable == 1 {
				c.send(ascensionMovie(92))
				return false
			}
			if dialogID == 10001 && variable == 1 && c.customQuestProgress(sanctumCeremonyQuestID, setQuestVar(quest.Vars, 0, 2), "") {
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		case 203752:
			if dialogID == 25 && variable == 2 {
				c.send(dialogWindow(o.id, 1693, sanctumCeremonyQuestID))
				return true
			}
			if dialogID == 1694 && variable == 2 {
				c.send(ascensionMovie(91))
				return false
			}
			if dialogID == 10002 && variable == 2 {
				classVariable := sanctumCeremonyClassVariable(c.player.Class)
				if classVariable == 0 || !c.customQuestProgress(sanctumCeremonyQuestID, setQuestVar(quest.Vars, 0, classVariable), "REWARD") {
					return false
				}
				c.send(dialogWindow(o.id, 10, 0))
				return true
			}
		}
		return false
	}
	if quest.Status != "REWARD" {
		return false
	}
	rewardNPC, rewardPage, rewardWindow, rewardIndex := sanctumCeremonyReward(variable)
	if o.npc.ID != rewardNPC {
		return false
	}
	switch dialogID {
	case -1:
		c.send(dialogWindow(o.id, rewardPage, sanctumCeremonyQuestID))
		return true
	case 1009:
		c.send(dialogWindow(o.id, rewardWindow, sanctumCeremonyQuestID))
		return true
	default:
		if dialogID >= 8 && dialogID <= 17 {
			c.finishQuestReward(script, o.id, uint16(dialogID), rewardIndex)
			return true
		}
	}
	return false
}

func sanctumCeremonyClassVariable(class string) int32 {
	switch class {
	case "WARRIOR", "GLADIATOR", "TEMPLAR":
		return 10
	case "SCOUT", "ASSASSIN", "RANGER":
		return 20
	case "MAGE", "SORCERER", "SPIRIT_MASTER":
		return 30
	case "PRIEST", "CLERIC", "CHANTER":
		return 40
	default:
		return 0
	}
}

func sanctumCeremonyReward(variable int32) (npcID int32, page, window uint16, rewardIndex int) {
	switch variable {
	case 10:
		return 203758, 2034, 5, 0
	case 20:
		return 203759, 2375, 6, 1
	case 30:
		return 203760, 2716, 7, 2
	case 40:
		return 203761, 3057, 8, 3
	default:
		return 0, 0, 0, -1
	}
}

func sanctumCeremonyTeleportLocation() *wire.Writer {
	w := wire.Packet(smTeleportLoc)
	w.C(3)
	w.C(0x90)
	w.C(0x9e)
	w.D(110010000)
	w.F(1313)
	w.F(1512)
	w.F(568)
	w.C(0)
	return w
}
