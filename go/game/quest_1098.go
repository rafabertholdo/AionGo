package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

const (
	pearlOfProtectionQuestID int32 = 1098
	pearlOfProtectionPernos  int32 = 790001
	pearlOfProtectionDaminu  int32 = 730008
	pearlOfProtectionLodas   int32 = 730019
	pearlOfProtectionArbolu  int32 = 730133
	pearlOfProtectionKhidia  int32 = 203183
	pearlOfProtectionTumblusen int32 = 203989
	pearlOfProtectionAtropos    int32 = 798155
	pearlOfProtectionAphesius   int32 = 204549
	pearlOfProtectionJucleas    int32 = 203752
	pearlOfProtectionMorai      int32 = 203164
	pearlOfProtectionGaia       int32 = 203917
	pearlOfProtectionKimeia     int32 = 203996
	pearlOfProtectionJamanok    int32 = 798176
	pearlOfProtectionSerimnir   int32 = 798212
	pearlOfProtectionMaximus    int32 = 204535
	pearlOfProtectionFirstItem  int32 = 182206062
	pearlOfProtectionSecondItem int32 = 182206063
	pearlOfProtectionThirdItem  int32 = 182206064
	pearlOfProtectionFourthItem int32 = 182206065
)

type pearlOfProtectionStep struct {
	npcID        int32
	variable     int32
	page         int32
	acceptDialog int32
	removeItem   int32
	grantItem    int32
}

func (c *conn) pearlOfProtectionLevelUp() bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(pearlOfProtectionQuestID)
	prerequisite := c.player.quest(1097)
	template := c.s.data.Quests[pearlOfProtectionQuestID]
	if quest == nil || quest.Status != "LOCKED" || prerequisite == nil || prerequisite.Status != "COMPLETE" || template == nil || c.player.level < template.MinLevel {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("unlocking Pearl of Protection", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

var pearlOfProtectionSteps = [...]pearlOfProtectionStep{
	{npcID: pearlOfProtectionPernos, variable: 0, page: 1011, acceptDialog: 10000, grantItem: pearlOfProtectionFirstItem},
	{npcID: pearlOfProtectionDaminu, variable: 1, page: 1352, acceptDialog: 10001},
	{npcID: pearlOfProtectionLodas, variable: 2, page: 1693, acceptDialog: 10002},
	{npcID: pearlOfProtectionArbolu, variable: 3, page: 2034, acceptDialog: 10003, removeItem: pearlOfProtectionFirstItem, grantItem: pearlOfProtectionSecondItem},
	{npcID: pearlOfProtectionKhidia, variable: 4, page: 2375, acceptDialog: 10004},
	{npcID: pearlOfProtectionTumblusen, variable: 5, page: 2716, acceptDialog: 10005},
	{npcID: pearlOfProtectionAtropos, variable: 6, page: 3057, acceptDialog: 10006},
	{npcID: pearlOfProtectionAphesius, variable: 7, page: 3398, acceptDialog: 10007},
	{npcID: pearlOfProtectionJucleas, variable: 8, page: 3739, acceptDialog: 10008, removeItem: pearlOfProtectionSecondItem, grantItem: pearlOfProtectionThirdItem},
	{npcID: pearlOfProtectionMorai, variable: 9, page: 4080, acceptDialog: 10009},
	{npcID: pearlOfProtectionGaia, variable: 10, page: 1608, acceptDialog: 10010},
	{npcID: pearlOfProtectionKimeia, variable: 11, page: 1949, acceptDialog: 10011},
	{npcID: pearlOfProtectionJamanok, variable: 12, page: 2290, acceptDialog: 10012},
	{npcID: pearlOfProtectionSerimnir, variable: 13, page: 2631, acceptDialog: 10013},
	{npcID: pearlOfProtectionMaximus, variable: 14, page: 2972, acceptDialog: 10255, removeItem: pearlOfProtectionThirdItem, grantItem: pearlOfProtectionFourthItem},
}

// pearlOfProtectionDialog ports the Java fifteen-NPC dialogue chain. Each
// work item is removed at its corresponding handoff; Java does not gate the
// handoff on having the item or on inventory capacity for the next one.
func (c *conn) pearlOfProtectionDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || o.dead || script == nil || script.ID != pearlOfProtectionQuestID {
		return false
	}
	quest := c.player.quest(pearlOfProtectionQuestID)
	if quest == nil || quest.Status == "NONE" {
		if o.npc.ID != pearlOfProtectionPernos {
			return false
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 1011, pearlOfProtectionQuestID))
			return true
		case 1007:
			c.send(dialogWindow(o.id, 4, pearlOfProtectionQuestID))
			return true
		case 1003:
			c.send(dialogWindow(o.id, 1004, pearlOfProtectionQuestID))
			return true
		case 10000:
			if !c.s.canStartQuest(c.player, script) {
				return false
			}
			next := store.Quest{ID: pearlOfProtectionQuestID, Status: "START", Vars: setQuestVar(0, 0, 1)}
			if quest != nil {
				next.CompleteCount = quest.CompleteCount
			}
			if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
				c.s.log.Error("starting Pearl of Protection", "err", err)
				return false
			}
			if quest == nil {
				c.player.quests = append(c.player.quests, next)
			} else {
				*quest = next
			}
			c.send(questAccepted(1, next))
			c.send(c.s.nearbyQuests(c.player))
			c.send(dialogWindow(o.id, 10, 0))
			if c.s.countItems(c.player, pearlOfProtectionFirstItem) == 0 {
				c.s.addItem(c.player, pearlOfProtectionFirstItem, 1)
			}
			return true
		default:
			return false
		}
	}
	if quest.Status == "REWARD" {
		if o.npc.ID == pearlOfProtectionPernos {
			switch {
			case dialogID == 25:
				c.send(dialogWindow(o.id, 10002, pearlOfProtectionQuestID))
				return true
			case dialogID == -1 || dialogID == 1009:
				if dialogID == 1009 {
					if !c.pearlOfProtectionPersist(quest, setQuestVar(quest.Vars, 0, 14), "REWARD") {
						return false
					}
				}
				c.send(dialogWindow(o.id, 5, pearlOfProtectionQuestID))
				return true
		case dialogID >= 8 && dialogID <= 17:
			// Java delegates every remaining reward choice to defaultQuestEndDialog,
			// whose fixed reward path completes only for choice 17.
			c.finishQuestReward(script, o.id, 17, 0)
				return true
			default:
				return false
			}
		}
		return false
	}
	if quest.Status != "START" {
		return false
	}
	variable := questVar(quest.Vars, 0)
	for _, step := range pearlOfProtectionSteps {
		if o.npc.ID != step.npcID || variable != step.variable {
			continue
		}
		if dialogID == 25 {
			c.send(dialogWindow(o.id, uint16(step.page), pearlOfProtectionQuestID))
			return true
		}
		if dialogID != step.acceptDialog {
			return false
		}
		return c.pearlOfProtectionAdvance(o, quest, step)
	}
	return false
}

// pearlOfProtectionAdvance is kept separate so the raw status mutation stays
// behind the same quest persistence boundary as the other custom handlers.
func (c *conn) pearlOfProtectionAdvance(o *object, quest *store.Quest, step pearlOfProtectionStep) bool {
	nextVariable := step.variable + 1
	nextStatus := ""
	if step.npcID == pearlOfProtectionMaximus {
		nextVariable = step.variable
		nextStatus = "REWARD"
	}
	if step.removeItem != 0 {
		c.s.removeItemsByID(c.player, step.removeItem, c.s.countItems(c.player, step.removeItem))
	}
	if !c.customQuestProgress(pearlOfProtectionQuestID, setQuestVar(quest.Vars, 0, nextVariable), nextStatus) {
		return false
	}
	c.send(dialogWindow(o.id, 10, 0))
	if step.grantItem != 0 {
		c.s.addItem(c.player, step.grantItem, 1)
	}
	return true
}

func (c *conn) pearlOfProtectionPersist(quest *store.Quest, vars int32, status string) bool {
	next := *quest
	next.Vars = vars
	if status != "" {
		next.Status = status
	}
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("updating Pearl of Protection", "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}
