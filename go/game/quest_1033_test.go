package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestSatalocasHeartLevelUpKillsAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[satalocasHeartQuestID], d.Quests[satalocasHeartQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || template.Race != "ELYOS" || template.MinLevel != 20 || template.NameID != 2204105 ||
		len(script.MonsterInfos) != 1 || script.MonsterInfos[0] != (data.QuestMonster{NPCID: archonDrakeNPCID, VarID: 0, MaxKill: 11}) || len(template.Rewards) != 2 ||
		template.Rewards[0].Experience != 168000 || len(template.Rewards[0].SelectableItems) != 4 || len(template.QuestDrops) != 2 {
		t.Fatalf("unexpected Sataloca's Heart metadata: script=%+v template=%+v", script, template)
	}
	if len(d.QuestKills[archonDrakeNPCID]) == 0 || len(d.QuestCustomTalks[satalocasHeartNPCID]) == 0 || len(d.QuestCustomTalks[satalocaKimeiaNPCID]) == 0 {
		t.Fatal("Sataloca's Heart kill or NPC dialogue indexes are missing")
	}
	s := testServer(d)
	saver := &recordedQuests{}
	s.quests = saver
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "SORCERER", 19
	p.Exp = d.ExpStart(p.level)
	p.cube = []*store.Item{}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: satalocasHeartQuestID, Status: "LOCKED"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p

	if c.satalocasHeartLevelUp() {
		t.Fatal("quest unlocked below level 20")
	}
	p.level = 20
	if !c.satalocasHeartLevelUp() || p.quest(satalocasHeartQuestID).Status != "START" || c.satalocasHeartLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(satalocasHeartQuestID))
	}
	diomedes := questCatalogNPC(s, p, satalocasHeartNPCID, 0x31040)
	kimeia := questCatalogNPC(s, p, satalocaKimeiaNPCID, 0x31041)
	script = d.QuestScripts[satalocasHeartQuestID]
	if c.satalocasHeartDialog(diomedes, script, -1) {
		t.Fatal("initial click bypassed the default quest-start dialog")
	}
	if !c.satalocasHeartDialog(diomedes, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(diomedes.id, 1011, satalocasHeartQuestID).Data) {
		t.Fatalf("Diomedes opening page = %x", packets.last(smDialogWindow))
	}
	if !c.satalocasHeartDialog(diomedes, script, 10000) || questVar(p.quest(satalocasHeartQuestID).Vars, 0) != 1 {
		t.Fatalf("Diomedes did not advance to Kimeia: %+v", p.quest(satalocasHeartQuestID))
	}
	if !c.satalocasHeartDialog(kimeia, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(kimeia.id, 1693, satalocasHeartQuestID).Data) {
		t.Fatalf("Kimeia page = %x", packets.last(smDialogWindow))
	}
	if !c.satalocasHeartDialog(kimeia, script, 10002) || questVar(p.quest(satalocasHeartQuestID).Vars, 0) != 10 || !bytes.Equal(packets.last(smPlayMovie), ascensionMovie(42).Data) {
		t.Fatalf("Kimeia did not start the Drake hunt: quest=%+v movie=%x", p.quest(satalocasHeartQuestID), packets.last(smPlayMovie))
	}
	if c.satalocasHeartKill(210808) || questVar(p.quest(satalocasHeartQuestID).Vars, 0) != 10 {
		t.Fatal("wrong monster advanced the hunt")
	}
	if !c.satalocasHeartKill(archonDrakeNPCID) || questVar(p.quest(satalocasHeartQuestID).Vars, 0) != 11 {
		t.Fatal("first Archon Drake did not advance the hunt")
	}
	for kill := 2; kill <= 10; kill++ {
		if !c.satalocasHeartKill(archonDrakeNPCID) {
			t.Fatalf("Archon Drake kill %d was not answered", kill)
		}
	}
	if questVar(p.quest(satalocasHeartQuestID).Vars, 0) != 11 || p.quest(satalocasHeartQuestID).Status != "START" {
		t.Fatalf("ten Drakes did not complete the hunt stage: %+v", p.quest(satalocasHeartQuestID))
	}
	if !c.satalocasHeartKill(archonDrakeNPCID) || questVar(p.quest(satalocasHeartQuestID).Vars, 0) != 11 {
		t.Fatal("extra Drake did not preserve and echo the completed hunt state")
	}
	if !c.satalocasHeartDialog(kimeia, script, 25) || p.quest(satalocasHeartQuestID).Status != "REWARD" || questVar(p.quest(satalocasHeartQuestID).Vars, 0) != 12 ||
		!bytes.Equal(packets.last(smDialogWindow), dialogWindow(kimeia.id, 2205, satalocasHeartQuestID).Data) {
		t.Fatalf("Kimeia did not open the reward stage: quest=%+v page=%x", p.quest(satalocasHeartQuestID), packets.last(smDialogWindow))
	}
	if !c.satalocasHeartDialog(diomedes, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(diomedes.id, 5, satalocasHeartQuestID).Data) {
		t.Fatalf("Diomedes reward preview = %x", packets.last(smDialogWindow))
	}
	if !c.satalocasHeartDialog(diomedes, script, 8) || p.quest(satalocasHeartQuestID).Status != "COMPLETE" {
		t.Fatalf("selectable reward did not complete the quest: %+v", p.quest(satalocasHeartQuestID))
	}
	if got := s.countItems(p, template.Rewards[0].SelectableItems[0].ID); got != 1 {
		t.Fatalf("selected reward count = %d, want 1", got)
	}
}

func TestSatalocasHeartRewardDialog17CompletesWithoutChoice(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "SORCERER", 20
	p.Exp = d.ExpStart(p.level)
	p.cube = []*store.Item{}
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: satalocasHeartQuestID, Status: "REWARD", Vars: 12}}
	c := &conn{s: s, player: p}
	p.conn = c
	s.spawned[p.ID] = p
	diomedes := questCatalogNPC(s, p, satalocasHeartNPCID, 0x31042)
	if !c.satalocasHeartDialog(diomedes, d.QuestScripts[satalocasHeartQuestID], 17) || p.quest(satalocasHeartQuestID).Status != "COMPLETE" {
		t.Fatalf("dialog 17 did not finish the quest without a selected reward: %+v", p.quest(satalocasHeartQuestID))
	}
	if got := s.countItems(p, d.Quests[satalocasHeartQuestID].Rewards[0].SelectableItems[0].ID); got != 0 {
		t.Fatalf("dialog 17 unexpectedly awarded a selectable item: %d", got)
	}
}
