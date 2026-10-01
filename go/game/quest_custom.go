package game

import (
	"aionlightning/game/data"
)

// customQuestDialog dispatches individually ported Java quest handlers. Only
// handlers listed here are registered in data.QuestScripts and NPC indexes.
func (c *conn) customQuestDialog(o *object, script *data.QuestScript, dialogID uint16) {
	c.customQuestDialogID(o, script, int32(dialogID))
}

// customQuestDialogID is customQuestDialog for a dialog id that may be -1, the id Java gives CM_SHOW_DIALOG. A handler
// that takes a uint16 sees 0xffff for it.
func (c *conn) customQuestDialogID(o *object, script *data.QuestScript, d int32) {
	u := uint16(d)
	switch script.ID {
	case 1006:
		c.dialogResult(c.ascensionDialog(o, script, d))
		return
	case 1007:
		c.dialogResult(c.sanctumCeremonyDialog(o, script, d))
		return
	case 1031:
		c.dialogResult(c.mandurisSecretDialog(o, script, d))
		return
	case 1032:
		c.dialogResult(c.rulersDutyDialog(o, script, d))
		return
	case 1033:
		c.dialogResult(c.satalocasHeartDialog(o, script, d))
		return
	case 1034:
		c.dialogResult(c.disappearingAetherDialog(o, script, d))
		return
	case 1035:
		c.dialogResult(c.refreshingSpringsDialog(o, script, d))
		return
	case 1036:
		c.dialogResult(c.kaidanPrisonerDialog(o, script, d))
		return
	case 1037:
		c.dialogResult(c.secretsOfTempleDialog(o, script, d))
		return
	case 1038:
		c.dialogResult(c.shadowsCommandDialog(o, script, d))
		return
	case 1039:
		c.dialogResult(c.somethingInTheWaterDialog(o, script, d))
		return
	case 1040:
		c.dialogResult(c.scoutingTheScoutsDialog(o, script, d))
		return
	case 1041:
		c.dialogResult(c.dangerousArtifactDialog(o, script, d))
		return
	case 1042:
		c.dialogResult(c.keeperKaidanKeyDialog(o, script, d))
		return
	case 1043:
		c.dialogResult(c.balaurConspiracyDialog(o, script, d))
		return
	case 1051:
		c.dialogResult(c.ruinsOfRoahDialog(o, script, d))
		return
	case 1052:
		c.dialogResult(c.rootOfRotDialog(o, script, d))
		return
	case 1053:
		c.dialogResult(c.klawThreatDialog(o, script, d))
		return
	case 1054:
		c.dialogResult(c.powerOfElimDialog(o, script, d))
		return
	case 1055:
		c.dialogResult(c.eternalRestDialog(o, script, d))
		return
	case 1056:
		c.dialogResult(c.lepharistPoisonResearchDialog(o, script, d))
		return
	case 1057:
		c.dialogResult(c.creatingMonsterDialog(o, script, d))
		return
	case 1058:
		c.dialogResult(c.aetherInsanityDialog(o, script, d))
		return
	case 1059:
		c.dialogResult(c.archonOfStormsDialog(o, script, d))
		return
	case 1062:
		c.dialogResult(c.indratuLegionDialog(o, script, d))
		return
	case 1072:
		c.dialogResult(c.abyssTrainingDialog(o, script, d))
		return
	case 1071:
		c.dialogResult(c.speakingBalaurDialog(o, script, d))
		return
	case 1075:
		c.dialogResult(c.newWingsDialog(o, script, d))
		return
	case 1076:
		c.dialogResult(c.fragmentOfMemory2Dialog(o, script, d))
		return
	case 1091:
		c.dialogResult(c.atroposRequestDialog(o, script, d))
		return
	case 1092:
		c.dialogResult(c.josnackDilemmaDialog(o, script, d))
		return
	case 1098:
		c.dialogResult(c.pearlOfProtectionDialog(o, script, d))
		return
	case 1162:
		c.dialogResult(c.altenosWeddingRingDialog(o, script, d))
		return
	case 1163:
		c.dialogResult(c.arachnaAntidoteDialog(o, script, d))
		return
	case 1170:
		c.dialogResult(c.headlessStoneStatueDialog(o, script, d))
		return
	case 1183:
		c.dialogResult(c.spiritOfNatureDialog(o, script, d))
		return
	case 1192:
		c.dialogResult(c.verteronReinforcementsDialog(o, script, d))
		return
	case 3060:
		c.dialogResult(c.redJournalDialog(o, script, d))
		return
	case 1011:
		c.dangerFromAboveDialog(o, script, d)
		return
	case 1012:
		c.maskedLoiterersDialog(o, script, u)
		return
	case 1013:
		c.huntingLepharistRevolutionariesDialog(o, script, d)
		return
	case 1014:
		c.dukakiOdiumDialog(o, script, d)
		return
	case 1015:
		c.frillneckHuntDialog(o, script, d)
		return
	case 1016:
		c.sourcePollutionDialog(o, script, d)
		return
	case 1017:
		c.heldSacredDialog(o, script, d)
		return
	case 1018:
		c.markOfVengeanceDialog(o, script, d)
		return
	case 1019:
		c.flyingReconnaissanceDialog(o, script, d)
		return
	case 1020:
		c.sealingAbyssGateDialog(o, script, d)
		return
	case 1021:
		c.trandilasEggsDialog(o, script, d)
		return
	case 1022:
		c.krallDesecrationDialog(o, script, d)
		return
	case 1023:
		c.aNestOfLepharistsDialog(o, script, d)
		return
	case 1097:
		c.swordTranscendenceDialog(o, script, u)
		return
	case 1149:
		c.missingPoppyDialog(o, script, d)
		return
	case 1130:
		c.summonsToCitadelDialog(o, script, u)
		return
	case 1156:
		c.stolenVillageSealDialog(o, script, d)
		return
	case 1157:
		c.gaphyrksLoveDialog(o, script, d)
		return
	case 1158:
		c.villageSealFoundDialog(o, script, d)
		return
	case forestOutlawQuestID:
		c.forestOutlawDialog(o, script, d)
		return
	case belbuasTreasureQuestID:
		c.belbuasTreasureDialog(o, script, d)
		return
	case delicateMandrakeQuestID:
		c.delicateMandrakeDialog(o, script, d)
		return
	}
	if script.LevelUpStart {
		c.levelUpQuestDialog(o, script, d)
		return
	}
	if chain := talkChains[script.ID]; chain != nil {
		c.talkChainDialog(o, script, u, chain)
		return
	}
	switch script.ID {
	case 1528, 1527, 1609, 1628, 1909, 1452, 1324, 2693, 2651, 1314, 2222, 1218, 1131:
		c.simpleThreeNPCQuestDialog(o, script, u)
		return
	case 1553, 1620, 1578, 1605, 1483:
		c.twoReportsQuestDialog(o, script, u)
		return
	case 2578, 2846, 2847, 2848, 4053:
		c.itemTwoReportQuestDialog(o, script, u)
		return
	case 1309, 1323, 2274:
		c.delayedItemQuestNPCDialog(o, script, u)
		return
	}
	if script.ItemUseDelay > 0 {
		c.delayedItemQuestDialog(o, script, u)
		return
	}
	if script.MiddleNPC != 0 {
		c.threeNPCQuestDialog(o, script, u)
		return
	}
	switch script.ID {
	case 1111:
		c.insomniaMedicineDialog(o, script, u)
	case 1114:
		c.nymphsGownDialog(o, script, d)
	case 1122:
		c.pernosRobeDialog(o, script, u)
	case 2125:
		c.robberyPlotDialog(o, script, u)
	case 2135:
		c.negiDialog(o, script, u)
	case 2114:
		c.insectProblemDialog(o, script, u)
	case 1123:
		c.wheresTuttyDialog(o, script, u)
	case 1100:
		c.kaliosCallDialog(o, script, u)
	case 2100:
		c.orderOfTheCaptainDialog(o, script, u)
	case 1001:
		c.kerubThreatDialog(o, script, u)
	case 2001:
		c.dialogResult(c.thinkingAheadEvent(o, script, d, false))
	case 1002:
		c.requestOfTheElimDialog(o, script, d)
	case 1003:
		c.illegalLoggingDialog(o, script, d)
	case 2002:
		c.dialogResult(c.wheresRaeEvent(o, script, d, false))
	case 2003:
		c.dialogResult(c.treasureOfTheDeceasedEvent(o, script, d))
	case 1004:
		c.neutralizingOdiumDialog(o, script, d)
	case 1005:
		c.barringTheGateDialog(o, script, d)
	case 2004:
		c.dialogResult(c.aCharmedCubeEvent(o, script, d, false))
	case 2005:
		c.dialogResult(c.teachingALessonEvent(o, script, d))
	case 2006:
		c.dialogResult(c.hitThemWhereItHurtsDialog(o, script, d))
	case 2007:
		c.dialogResult(c.wheresRaeThisTimeDialog(o, script, d))
	case 1107:
		c.lostAxeDialog(o, script, u)
	case 2107:
		c.returnToSenderDialog(o, script, u)
	case 1205, 2132:
		c.newSkillDialog(o, script, d)
	}
}

func (c *conn) customQuestShowDialog(o *object, script *data.QuestScript) bool {
	switch script.ID {
	case 1006:
		return c.ascensionDialog(o, script, -1)
	case 1007:
		return c.sanctumCeremonyDialog(o, script, -1)
	case 1031:
		return c.mandurisSecretDialog(o, script, -1)
	case 1032:
		return c.rulersDutyDialog(o, script, -1)
	case 1033:
		return c.satalocasHeartDialog(o, script, -1)
	case 1034:
		return c.disappearingAetherDialog(o, script, -1)
	case 1035:
		return c.refreshingSpringsDialog(o, script, -1)
	case 1036:
		return c.kaidanPrisonerDialog(o, script, -1)
	case 1037:
		return c.secretsOfTempleDialog(o, script, -1)
	case 1038:
		return c.shadowsCommandDialog(o, script, -1)
	case 1039:
		return c.somethingInTheWaterDialog(o, script, -1)
	case 1040:
		return c.scoutingTheScoutsDialog(o, script, -1)
	case 1041:
		return c.dangerousArtifactDialog(o, script, -1)
	case 1042:
		return c.keeperKaidanKeyDialog(o, script, -1)
	case 1043:
		return c.balaurConspiracyDialog(o, script, -1)
	case 1051:
		return c.ruinsOfRoahDialog(o, script, -1)
	case 1052:
		return c.rootOfRotDialog(o, script, -1)
	case 1053:
		return c.klawThreatDialog(o, script, -1)
	case 1054:
		return c.powerOfElimDialog(o, script, -1)
	case 1055:
		return c.eternalRestDialog(o, script, -1)
	case 1056:
		return c.lepharistPoisonResearchDialog(o, script, -1)
	case 1057:
		return c.creatingMonsterDialog(o, script, -1)
	case 1058:
		return c.aetherInsanityDialog(o, script, -1)
	case 1059:
		return c.archonOfStormsDialog(o, script, -1)
	case 1062:
		return c.indratuLegionDialog(o, script, -1)
	case 1072:
		return c.abyssTrainingDialog(o, script, -1)
	case 1071:
		return c.speakingBalaurDialog(o, script, -1)
	case 1075:
		return c.newWingsDialog(o, script, -1)
	case 1076:
		return c.fragmentOfMemory2Dialog(o, script, -1)
	case 1091:
		return c.atroposRequestDialog(o, script, -1)
	case 1092:
		return c.josnackDilemmaDialog(o, script, -1)
	case 1098:
		return c.pearlOfProtectionDialog(o, script, -1)
	case 1162:
		return c.altenosWeddingRingDialog(o, script, -1)
	case 1163:
		return c.arachnaAntidoteDialog(o, script, -1)
	case 1170:
		return c.headlessStoneStatueDialog(o, script, -1)
	case 1183:
		return c.spiritOfNatureDialog(o, script, -1)
	case 1192:
		return c.verteronReinforcementsDialog(o, script, -1)
	case 3060:
		return c.redJournalDialog(o, script, -1)
	case 1001:
		return c.kerubThreatShowDialog(o, script)
	case 1011:
		return c.dangerFromAboveDialog(o, script, -1)
	case 1012:
		return false
	case 1013:
		return c.huntingLepharistRevolutionariesDialog(o, script, -1)
	case 1014:
		return c.dukakiOdiumDialog(o, script, -1)
	case 1015:
		return c.frillneckHuntShowDialog(o, script)
	case 1016:
		return c.sourcePollutionShowDialog(o, script)
	case 1017:
		return c.heldSacredDialog(o, script, -1)
	case 1018:
		return c.markOfVengeanceDialog(o, script, -1)
	case 1019:
		return c.flyingReconnaissanceDialog(o, script, -1)
	case 1020:
		return c.sealingAbyssGateShowDialog(o, script)
	case 1021:
		return c.trandilasEggsShowDialog(o, script)
	case 1022:
		return c.krallDesecrationShowDialog(o, script)
	case 1023:
		return c.aNestOfLepharistsShowDialog(o, script)
	case 1097:
		return c.swordTranscendenceDialog(o, script, ^uint16(0))
	case 1149:
		return c.missingPoppyShowDialog(o, script)
	case 1130:
		return c.summonsToCitadelDialog(o, script, ^uint16(0))
	case 1156:
		if o != nil && o.npc != nil && o.npc.ID == 798003 {
			return c.stolenVillageSealRewardShowDialog(o, script)
		}
		return c.stolenVillageSealShowDialog(o, script)
	case 1157:
		return c.gaphyrksLoveShowDialog(o, script)
	case 1158:
		return c.villageSealFoundDialog(o, script, -1)
	case forestOutlawQuestID:
		return c.forestOutlawShowDialog(o, script)
	case belbuasTreasureQuestID:
		return c.belbuasTreasureDialog(o, script, -1)
	case delicateMandrakeQuestID:
		return c.delicateMandrakeDialog(o, script, -1)
	}
	if script.ID == 1309 || script.ID == 1323 || script.ID == 2274 {
		return c.delayedItemQuestNPCDialog(o, script, ^uint16(0))
	}
	if script.ID == 1114 {
		return c.nymphsGownDialog(o, script, -1)
	}
	if script.LevelUpStart {
		return c.levelUpQuestDialog(o, script, -1)
	}
	if chain := talkChains[script.ID]; chain != nil {
		return c.talkChainShowDialog(o, script, chain)
	}
	if script.ID == 1205 || script.ID == 2132 {
		return c.newSkillDialog(o, script, -1)
	}
	if script.ID == 1100 {
		return c.kaliosCallShowDialog(o, script)
	}
	if script.ID == 1001 {
		return c.kerubThreatShowDialog(o, script)
	}
	if script.ID == 2001 {
		return c.thinkingAheadEvent(o, script, -1, false)
	}
	if script.ID == 1002 {
		if q := c.player.quest(1002); q != nil && q.Status == "REWARD" {
			return c.answered(func() { c.requestOfTheElimDialog(o, script, -1) })
		}
	}
	if script.ID == 2002 {
		return c.wheresRaeEvent(o, script, -1, false)
	}
	if script.ID == 2003 {
		return c.treasureOfTheDeceasedEvent(o, script, -1)
	}
	if script.ID == 1004 {
		if q := c.player.quest(1004); q != nil && q.Status == "REWARD" {
			return c.answered(func() { c.neutralizingOdiumDialog(o, script, -1) })
		}
	}
	if script.ID == 1005 {
		if q := c.player.quest(1005); q != nil && q.Status == "REWARD" {
			return c.answered(func() { c.barringTheGateDialog(o, script, -1) })
		}
	}
	if script.ID == 2004 {
		return c.aCharmedCubeEvent(o, script, -1, false)
	}
	if script.ID == 2005 {
		return c.teachingALessonEvent(o, script, -1)
	}
	if script.ID == 2006 {
		return c.hitThemWhereItHurtsDialog(o, script, -1)
	}
	if script.ID == 2007 {
		return c.wheresRaeThisTimeDialog(o, script, -1)
	}
	if script.ID == 2114 {
		return c.insectProblemShowDialog(o, script)
	}
	if script.ID == 1122 {
		q := c.player.quest(script.ID)
		if q != nil && q.Status == "REWARD" && q.Vars >= 1 && q.Vars <= 3 {
			c.send(dialogWindow(o.id, uint16(4+q.Vars), script.ID))
			return true
		}
	}
	if script.ID != 1111 {
		return false
	}
	q := c.player.quest(script.ID)
	if q == nil || q.Status != "REWARD" {
		return false
	}
	switch q.Vars {
	case 2:
		c.s.removeItemsByID(c.player, 182200222, c.s.countItems(c.player, 182200222))
		c.send(dialogWindow(o.id, 2375, script.ID))
	case 3:
		c.s.removeItemsByID(c.player, 182200221, c.s.countItems(c.player, 182200221))
		c.send(dialogWindow(o.id, 2716, script.ID))
	default:
		return false
	}
	return true
}

// Delivering Pernos's Robe chooses one of three turn-in items and matching
// reward records. The alternate drops are registered from quest_data.xml.
func (c *conn) pernosRobeDialog(o *object, script *data.QuestScript, dialogID uint16) {
	p := c.player
	q := p.quest(script.ID)
	if o.npc.ID == script.StartNPC && (q == nil || q.Status == "NONE" || q.Status == "COMPLETE") {
		c.customQuestStart(o, script, dialogID, script.ItemID)
		return
	}
	if q == nil || o.npc.ID != script.EndNPC {
		return
	}
	if q.Status == "REWARD" {
		if q.Vars < 1 || q.Vars > 3 {
			return
		}
		switch dialogID {
		case 1009:
			c.send(dialogWindow(o.id, uint16(4+q.Vars), script.ID))
		case 17:
			c.finishQuestReward(script, o.id, dialogID, int(q.Vars-1))
		}
		return
	}
	if q.Status != "START" {
		return
	}
	if dialogID == 25 {
		c.send(dialogWindow(o.id, 1352, script.ID))
		return
	}
	if dialogID < 10000 || dialogID > 10002 {
		return
	}
	index := int(dialogID - 10000)
	turnIns := [...]int32{182200218, 182200219, 182200220}
	windows := [...]uint16{1523, 1438, 1353}
	if c.s.countItems(p, turnIns[index]) == 0 {
		c.send(dialogWindow(o.id, 1608, script.ID))
		return
	}
	if c.customQuestProgress(script.ID, int32(index+1), "REWARD") {
		c.s.removeItemsByID(p, turnIns[index], c.s.countItems(p, turnIns[index]))
		c.s.removeItemsByID(p, script.ItemID, c.s.countItems(p, script.ItemID))
		c.send(dialogWindow(o.id, windows[index], script.ID))
	}
}

// Insomnia Medicine has two mutually exclusive reward records chosen at the
// middle NPC. The potion handed out there is removed before final turn-in.
func (c *conn) insomniaMedicineDialog(o *object, script *data.QuestScript, dialogID uint16) {
	p := c.player
	q := p.quest(script.ID)
	if o.npc.ID == script.StartNPC && (q == nil || q.Status == "NONE" || q.Status == "COMPLETE") {
		c.customQuestStart(o, script, dialogID, 0)
		return
	}
	if q == nil {
		return
	}
	if o.npc.ID == script.StartNPC && q.Status == "REWARD" {
		switch dialogID {
		case 1009:
			c.send(dialogWindow(o.id, uint16(q.Vars+3), script.ID))
		case 17:
			c.finishQuestReward(script, o.id, dialogID, int(q.Vars-2))
		}
		return
	}
	if o.npc.ID != 203061 || q.Status != "START" {
		return
	}
	switch dialogID {
	case 25:
		if q.Vars == 0 || q.Vars == 1 {
			c.send(dialogWindow(o.id, uint16(1352+q.Vars), script.ID))
		}
	case 33:
		if !c.s.hasQuestItems(p, c.s.data.Quests[script.ID]) {
			c.send(dialogWindow(o.id, 1693, script.ID))
			return
		}
		if c.customQuestProgress(script.ID, q.Vars+1, "") {
			for _, item := range c.s.data.Quests[script.ID].CollectItems {
				c.s.removeItemsByID(p, item.ID, item.Count)
			}
			c.send(dialogWindow(o.id, 1353, script.ID))
		}
	case 10000, 10001:
		itemID, nextVars := int32(182200222), int32(2)
		if dialogID == 10001 {
			itemID, nextVars = 182200221, 3
		}
		if !c.s.questRewardsFit(p, []data.QuestItem{{ID: itemID, Count: 1}}) {
			c.send(systemMessage(msgInventoryFull))
			return
		}
		if c.customQuestProgress(script.ID, nextVars, "REWARD") {
			c.s.addItem(p, itemID, 1)
			c.send(dialogWindow(o.id, 10, 0))
		}
	}
}

func (c *conn) customQuestStart(o *object, script *data.QuestScript, dialogID uint16, giveItem int32) bool {
	return c.customQuestStartN(o, script, dialogID, giveItem, 1)
}

// customQuestStartN is customQuestStart handing out giveCount of giveItem on accept.
func (c *conn) customQuestStartN(o *object, script *data.QuestScript, dialogID uint16, giveItem int32, giveCount int64) bool {
	if o.npc.ID != script.StartNPC || !c.s.canStartQuest(c.player, script) {
		return false
	}
	switch dialogID {
	case 25:
		c.send(dialogWindow(o.id, 1011, script.ID))
	case 1007:
		c.send(dialogWindow(o.id, 4, script.ID))
	case 1002:
		if giveItem != 0 && !c.s.questRewardsFit(c.player, []data.QuestItem{{ID: giveItem, Count: giveCount}}) {
			c.send(systemMessage(msgInventoryFull))
			return true
		}
		c.startQuest(script, o.id)
		if giveItem != 0 {
			if q := c.player.quest(script.ID); q != nil && q.Status == "START" {
				c.s.addItem(c.player, giveItem, giveCount)
			}
		}
	case 1003:
		c.send(dialogWindow(o.id, 1004, script.ID))
	}
	return true
}

func (c *conn) customQuestEnd(o *object, script *data.QuestScript, dialogID uint16) {
	if o.npc.ID != script.EndNPC {
		return
	}
	q := c.player.quest(script.ID)
	if q == nil || q.Status != "REWARD" {
		return
	}
	if dialogID == 1009 {
		c.send(dialogWindow(o.id, 5, script.ID))
	} else if dialogID >= 8 && dialogID <= 17 {
		c.finishQuest(script, o.id, dialogID)
	}
}

func (c *conn) customQuestProgress(id, vars int32, status string) bool {
	q := c.player.quest(id)
	if q == nil || q.Status != "START" {
		return false
	}
	next := *q
	next.Vars = vars
	if status != "" {
		next.Status = status
	}
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("updating custom quest", "quest", id, "err", err)
		return false
	}
	*q = next
	c.send(questAccepted(2, next))
	return true
}

// The Robbery Plot: three NPC conversations, with quest variable 0->1->2.
func (c *conn) robberyPlotDialog(o *object, script *data.QuestScript, dialogID uint16) {
	p := c.player
	q := p.quest(script.ID)
	if o.npc.ID == script.StartNPC && (q == nil || q.Status == "NONE" || q.Status == "COMPLETE") {
		c.customQuestStart(o, script, dialogID, 0)
		return
	}
	if q == nil {
		return
	}
	if q.Status == "REWARD" {
		c.customQuestEnd(o, script, dialogID)
		return
	}
	if q.Status != "START" {
		return
	}
	switch o.npc.ID {
	case 203514:
		if q.Vars != 0 {
			return
		}
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1352, script.ID))
		} else if dialogID == 10000 && c.customQuestProgress(script.ID, 1, "") {
			c.send(dialogWindow(o.id, 10, 0))
		}
	case script.EndNPC:
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 2375, script.ID))
		} else if dialogID == 1009 && c.customQuestProgress(script.ID, 2, "REWARD") {
			c.customQuestEnd(o, script, dialogID)
		}
	}
}

// For Love of Negi: the starter lends an item, the second NPC takes it, and
// the starter accepts the final report.
func (c *conn) negiDialog(o *object, script *data.QuestScript, dialogID uint16) {
	p := c.player
	q := p.quest(script.ID)
	if o.npc.ID == script.StartNPC && (q == nil || q.Status == "NONE" || q.Status == "COMPLETE") {
		c.customQuestStart(o, script, dialogID, script.ItemID)
		return
	}
	if q == nil {
		return
	}
	if q.Status == "REWARD" {
		c.customQuestEnd(o, script, dialogID)
		return
	}
	if q.Status != "START" {
		return
	}
	switch o.npc.ID {
	case 203531:
		if q.Vars != 0 {
			return
		}
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 1352, script.ID))
		} else if dialogID == 10000 && c.customQuestProgress(script.ID, 1, "") {
			c.s.removeItemsByID(p, script.ItemID, c.s.countItems(p, script.ItemID))
			c.send(dialogWindow(o.id, 10, 0))
		}
	case script.StartNPC:
		if dialogID == 25 {
			c.send(dialogWindow(o.id, 2375, script.ID))
		} else if dialogID == 1009 && c.customQuestProgress(script.ID, 2, "REWARD") {
			c.customQuestEnd(o, script, dialogID)
		}
	}
}
