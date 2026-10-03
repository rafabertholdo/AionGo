package data

import (
	"fmt"
	"strconv"
	"strings"
)

// QuestTemplate is the common quest metadata in quest_data.xml. Scripted quest
// steps live separately; this data provides start conditions and rewards.
type QuestTemplate struct {
	ID                         int32         `xml:"id,attr"`
	NameID                     int32         `xml:"nameId,attr"`
	Name                       string        `xml:"name,attr"`
	Race                       string        `xml:"race_permitted,attr"`
	MinLevel                   int           `xml:"minlevel_permitted,attr"`
	CombineSkill               int32         `xml:"combineskill,attr"`
	CombineSkillPoint          int32         `xml:"combine_skillpoint,attr"`
	MaxRepeatCount             int           `xml:"max_repeat_count,attr"`
	CannotGiveUp               bool          `xml:"cannot_giveup,attr"`
	UseClassReward             bool          `xml:"use_class_reward,attr"`
	ClassPermittedText         []string      `xml:"class_permitted"`
	ClassPermitted             []string      `xml:"-"`
	GenderPermitted            string        `xml:"gender_permitted"`
	FinishedQuestConditionText []string      `xml:"finished_quest_conds"`
	FinishedQuestConditions    []int32       `xml:"-"`
	CollectItems               []QuestItem   `xml:"collect_items>collect_item"`
	QuestDrops                 []QuestDrop   `xml:"quest_drop"`
	QuestWorkItems             []QuestItem   `xml:"quest_work_items>quest_work_item"`
	Rewards                    []QuestReward `xml:"rewards"`
	FighterRewards             []QuestItem   `xml:"fighter_selectable_reward"`
	KnightRewards              []QuestItem   `xml:"knight_selectable_reward"`
	RangerRewards              []QuestItem   `xml:"ranger_selectable_reward"`
	AssassinRewards            []QuestItem   `xml:"assassin_selectable_reward"`
	WizardRewards              []QuestItem   `xml:"wizard_selectable_reward"`
	ElementalistRewards        []QuestItem   `xml:"elementalist_selectable_reward"`
	PriestRewards              []QuestItem   `xml:"priest_selectable_reward"`
	ChanterRewards             []QuestItem   `xml:"chanter_selectable_reward"`
}

// QuestReward is one reward option in quest_data.xml.
type QuestReward struct {
	Experience      int64       `xml:"exp,attr"`
	Kinah           int64       `xml:"gold,attr"`
	AbyssPoints     int32       `xml:"reward_abyss_point,attr"`
	TitleID         int32       `xml:"title,attr"`
	ExtendInventory int32       `xml:"extend_inventory,attr"`
	Items           []QuestItem `xml:"reward_item"`
	SelectableItems []QuestItem `xml:"selectable_reward_item"`
}

type QuestItem struct {
	ID    int32 `xml:"item_id,attr"`
	Count int64 `xml:"count,attr"`
}

type QuestDrop struct {
	NPCID  int32   `xml:"npc_id,attr"`
	ItemID int32   `xml:"item_id,attr"`
	Chance float32 `xml:"chance,attr"`
}

// ClassRewards follows QuestService's selectable-reward class mapping.
func (q *QuestTemplate) ClassRewards(class string) []QuestItem {
	switch class {
	case "GLADIATOR":
		return q.FighterRewards
	case "TEMPLAR":
		return q.KnightRewards
	case "RANGER":
		return q.RangerRewards
	case "ASSASSIN":
		return q.AssassinRewards
	case "SORCERER":
		return q.WizardRewards
	case "SPIRIT_MASTER":
		return q.ElementalistRewards
	case "CLERIC":
		return q.PriestRewards
	case "CHANTER":
		return q.ChanterRewards
	default:
		return nil
	}
}

func loadQuests(path string) (map[int32]*QuestTemplate, error) {
	var file struct {
		Quests []*QuestTemplate `xml:"quest"`
	}
	if err := loadXML(path, &file); err != nil {
		return nil, err
	}
	quests := make(map[int32]*QuestTemplate, len(file.Quests))
	for _, quest := range file.Quests {
		for _, classes := range quest.ClassPermittedText {
			quest.ClassPermitted = append(quest.ClassPermitted, strings.Fields(classes)...)
		}
		quest.GenderPermitted = strings.TrimSpace(quest.GenderPermitted)
		for _, conditions := range quest.FinishedQuestConditionText {
			for _, field := range strings.Fields(conditions) {
				id, err := strconv.ParseInt(field, 10, 32)
				if err != nil {
					return nil, fmt.Errorf("quest %d prerequisite %q: %w", quest.ID, field, err)
				}
				quest.FinishedQuestConditions = append(quest.FinishedQuestConditions, int32(id))
			}
		}
		quests[quest.ID] = quest
	}
	return quests, nil
}
