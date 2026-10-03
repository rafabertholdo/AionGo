package game

import (
	"bytes"
	"fmt"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestAsmodianCampaignZoneStartAndTurnIn(t *testing.T) {
	for _, tc := range []struct {
		id, mapID, endNPC, firstLock, lastLock int32
		zone                                   string
	}{
		{altgardDutiesQuestID, altgardDutiesMapID, altgardDutiesEndNPC, 2011, 2022, altgardDutiesZone},
		{morheimCommandersCallQuestID, morheimCommandersCallMapID, morheimCommandersCallEndNPC, 2031, 2042, morheimCommandersCallZone},
	} {
		t.Run(fmt.Sprint(tc.id), func(t *testing.T) {
			_, s, p, c, script, packets := customQuestPortFixture(t, tc.id, nil)
			p.Race = "ASMODIANS"
			p.WorldID = tc.mapID
			s.enterQuestZone(p, nil, &data.Zone{Name: tc.zone, MapID: tc.mapID})
			quest := p.quest(tc.id)
			if quest == nil || quest.Status != "START" {
				t.Fatalf("fortress entry did not start quest %d: %+v", tc.id, quest)
			}
			s.enterQuestZone(p, nil, &data.Zone{Name: tc.zone, MapID: tc.mapID})
			if len(p.quests) != 1 {
				t.Fatalf("repeated zone entry created duplicate quests: %+v", p.quests)
			}
			npc := questCatalogNPC(s, p, tc.endNPC, tc.id+0x30000)
			var handled bool
			if tc.id == altgardDutiesQuestID {
				handled = c.altgardDutiesDialog(npc, script, 25)
			} else {
				handled = c.morheimCommandersCallDialog(npc, script, 25)
			}
			if !handled || quest.Status != "REWARD" || questVar(quest.Vars, 0) != 1 || string(packets.last(smDialogWindow)) != string(dialogWindow(npc.id, 1011, tc.id).Data) {
				t.Fatalf("campaign report did not reach reward state: quest=%+v page=%x", quest, packets.last(smDialogWindow))
			}
			if !c.customQuestShowDialog(npc, script) || string(packets.last(smDialogWindow)) != string(dialogWindow(npc.id, 5, tc.id).Data) {
				t.Fatal("reward click did not show the default end page")
			}
			if tc.id == altgardDutiesQuestID {
				c.altgardDutiesDialog(npc, script, 17)
			} else {
				c.morheimCommandersCallDialog(npc, script, 17)
			}
			if completed := p.quest(tc.id); completed == nil || completed.Status != "COMPLETE" {
				t.Fatalf("campaign quest did not complete: %+v", completed)
			}
			for id := tc.firstLock; id <= tc.lastLock; id++ {
				// LOCKED, or already START where Java's onLvlUp after the finish unlocks it at this level.
				if followUp := p.quest(id); followUp == nil || followUp.Status != "LOCKED" && followUp.Status != "START" {
					t.Fatalf("follow-up %d = %+v", id, followUp)
				}
			}
		})
	}
}

func TestMorheimCommandersCallUnlocksExistingLockedQuestAtLevel(t *testing.T) {
	d, s, p, c, _, packets := customQuestPortFixture(t, morheimCommandersCallQuestID, nil)
	p.Race = "ASMODIANS"
	locked := store.Quest{ID: morheimCommandersCallQuestID, Status: "LOCKED"}
	if err := s.quests.SaveQuest(p.ID, locked); err != nil {
		t.Fatal(err)
	}
	p.quests = append(p.quests, locked)
	p.level = d.Quests[morheimCommandersCallQuestID].MinLevel - 1
	if c.morheimCommandersCallLevelUp() || p.quest(morheimCommandersCallQuestID).Status != "LOCKED" {
		t.Fatal("quest unlocked below its required level")
	}
	p.level = d.Quests[morheimCommandersCallQuestID].MinLevel
	c.levelUpStartQuests()
	if p.quest(morheimCommandersCallQuestID).Status != "LOCKED" {
		t.Fatal("quest-finish check unlocked a quest reserved for the level-up event")
	}
	s.levelUp(p)
	if p.quest(morheimCommandersCallQuestID).Status != "START" {
		t.Fatal("level-up event did not unlock the existing locked quest")
	}
	want := questAccepted(2, *p.quest(morheimCommandersCallQuestID)).Data
	found := false
	for _, frame := range packets.frames {
		if bytes.Equal(frame, want) {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("level-up status packet")
	}
	if c.morheimCommandersCallLevelUp() {
		t.Fatal("repeated level-up transitioned the quest again")
	}
}

func TestCampaignZonesRequireRaceMapZoneAndLevel(t *testing.T) {
	_, _, p, c, _, _ := customQuestPortFixture(t, altgardDutiesQuestID, nil)
	if c.altgardDutiesEnterZone(altgardDutiesZone) {
		t.Fatal("wrong-race player started Altgard Duties")
	}
	p.Race = "ASMODIANS"
	p.WorldID = morheimCommandersCallMapID
	if c.altgardDutiesEnterZone(altgardDutiesZone) {
		t.Fatal("wrong map started Altgard Duties")
	}
	p.WorldID = altgardDutiesMapID
	p.level = 1
	if c.altgardDutiesEnterZone(altgardDutiesZone) {
		t.Fatal("under-level player started Altgard Duties")
	}
	if c.altgardDutiesEnterZone("OTHER") {
		t.Fatal("unrelated zone started Altgard Duties")
	}
}
