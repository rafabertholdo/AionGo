package game

import (
	"aionlightning/game/data"
)

const (
	fungusAmongUsQuestID      int32  = 2011
	fungusAmongUsCaptainID    int32  = 203558
	fungusAmongUsScoutID      int32  = 203572
	fungusAmongUsMushroomID   int32  = 700092
	encroachersQuestID        int32  = 2012
	encroachersReportNPCID    int32  = 203559
	encroachersBruteNPCID     int32  = 210715
	dangerousCropQuestID      int32  = 2013
	dangerousCropReportNPCID  int32  = 203605
	dangerousCropFieldNPCID   int32  = 700096
	dangerousCropZone         string = "MUMU_FARMLAND_220030000"
	dangerousCropFieldItemID  int32  = 182203012
	scoutItOutQuestID         int32  = 2014
	scoutItOutStartNPCID      int32  = 203606
	scoutItOutGraveNPCID      int32  = 700009
	scoutItOutScoutNPCID      int32  = 203633
	scoutItOutReportNPCID     int32  = 203631
	scoutItOutNamedNPCID      int32  = 700135
	scoutItOutEvidenceItemID  int32  = 182203015
	takeTheInitiativeQuestID  int32  = 2015
	takeInitiativeReportNPCID int32  = 203631
	fearThisQuestID           int32  = 2016
	fearThisReportNPCID       int32  = 203631
	fearThisSupplyNPCID       int32  = 203621
	fearThisItemID            int32  = 182203019
	fearThisZone              string = "Q2016"
	observatoryQuestID        int32  = 2017
	observatoryTalkNPCID      int32  = 203654
	observatoryReportNPCID    int32  = 203558
	impetusiumQuestID         int32  = 2018
	impetusiumReportNPCID     int32  = 203649
	impetusiumJewelBoxNPCID   int32  = 700097
	impetusiumGraveNPCID      int32  = 700098
	impetusiumNamedNPCID      int32  = 210752
	altgardDutiesPrerequisite int32  = 2200
)

// altgardStartupLevelUp ports the Asmodian startup and early campaign
// onLvlUpEvent handlers. The quests are created as LOCKED by 2200; absent
// quests must stay absent, as Java's handlers ignore a nil QuestState.
func (c *conn) altgardStartupLevelUp() {
	if c == nil || c.player == nil || c.player.Race != "ASMODIANS" {
		return
	}
	if previous := c.player.quest(altgardDutiesPrerequisite); previous != nil && previous.Status == "COMPLETE" {
		c.startLockedAltgardQuest(fungusAmongUsQuestID)
	}
	if template := c.s.data.Quests[encroachersQuestID]; template != nil && c.player.level >= template.MinLevel {
		c.startLockedAltgardQuest(encroachersQuestID)
	}
	c.startLockedAltgardQuest(dangerousCropQuestID)
	c.startLockedAltgardQuest(scoutItOutQuestID)
	if previous := c.player.quest(scoutItOutQuestID); previous != nil && previous.Status == "COMPLETE" {
		c.startLockedAltgardQuest(takeTheInitiativeQuestID)
	}
	c.startLockedAltgardQuest(fearThisQuestID)
	if template := c.s.data.Quests[observatoryQuestID]; template != nil && c.player.level >= template.MinLevel {
		if previous := c.player.quest(takeTheInitiativeQuestID); previous != nil && previous.Status == "COMPLETE" {
			c.startLockedAltgardQuest(observatoryQuestID)
		}
	}
	if template := c.s.data.Quests[impetusiumQuestID]; template != nil && c.player.level >= template.MinLevel {
		c.startLockedAltgardQuest(impetusiumQuestID)
	}
}

func (c *conn) startLockedAltgardQuest(questID int32) bool {
	if c == nil || c.player == nil {
		return false
	}
	quest := c.player.quest(questID)
	if quest == nil || quest.Status != "LOCKED" {
		return false
	}
	next := *quest
	next.Status = "START"
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("starting locked Altgard quest", "quest", questID, "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}

func (c *conn) altgardStartupQuestDialog(o *object, script *data.QuestScript, dialogID int32) bool {
	if c == nil || c.player == nil || o == nil || o.npc == nil || script == nil {
		return false
	}
	switch script.ID {
	case fungusAmongUsQuestID:
		return c.fungusAmongUsDialog(o, script, dialogID)
	case encroachersQuestID:
		return c.encroachersDialog(o, script, dialogID)
	case dangerousCropQuestID:
		return c.dangerousCropDialog(o, script, dialogID)
	case scoutItOutQuestID:
		return c.scoutItOutDialog(o, script, dialogID)
	case takeTheInitiativeQuestID:
		return c.takeTheInitiativeDialog(o, script, dialogID)
	case fearThisQuestID:
		return c.fearThisDialog(o, script, dialogID)
	case observatoryQuestID:
		return c.observatoryDialog(o, script, dialogID)
	case impetusiumQuestID:
		return c.impetusiumDialog(o, script, dialogID)
	default:
		return false
	}
}

func (c *conn) finishAltgardStartupQuest(o *object, script *data.QuestScript, questID int32, dialogID int32) bool {
	if script == nil || script.ID != questID || o == nil || o.npc == nil {
		return false
	}
	quest := c.player.quest(questID)
	if quest == nil || quest.Status != "REWARD" {
		return false
	}
	switch {
	case dialogID == -1 || dialogID == 1009:
		c.send(dialogWindow(o.id, 5, questID))
	case dialogID >= 8 && dialogID <= 17:
		c.finishQuest(script, o.id, uint16(dialogID))
	default:
		return false
	}
	return true
}

func (c *conn) beginAltgardStartupQuest(questID int32, vars int32, status string) bool {
	if c == nil || c.player == nil || c.player.quest(questID) == nil {
		return false
	}
	quest := c.player.quest(questID)
	if quest.Status != "START" {
		return false
	}
	next := *quest
	next.Vars = vars
	if status != "" {
		next.Status = status
	}
	if err := c.s.quests.SaveQuest(c.player.ID, next); err != nil {
		c.s.log.Error("updating Altgard quest", "quest", questID, "err", err)
		return false
	}
	*quest = next
	c.send(questAccepted(2, next))
	return true
}
