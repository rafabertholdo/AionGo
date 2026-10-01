package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestForestOutlawLevelUpNineKillRouteAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Quests[forestOutlawQuestID]
	if template == nil || template.Race != "ELYOS" || template.MinLevel != 11 || len(template.Rewards) != 1 ||
		template.Rewards[0].Experience != 18700 || template.Rewards[0].Kinah != 2100 {
		t.Fatalf("unexpected The Forest Outlaw metadata: %+v", template)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.level = "ELYOS", 10
	p.Exp = d.ExpStart(p.level)
	p.seen = map[int32]*object{}
	p.quests = []store.Quest{{ID: forestOutlawQuestID, Status: "LOCKED"}}
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: forestOutlawQuestID, Kind: data.QuestCustom, EndNPC: 203124}
	end := questCatalogNPC(s, p, 203124, 0x61139)
	wrongNPC := questCatalogNPC(s, p, 203125, 0x61140)

	if c.forestOutlawLevelUp() {
		t.Fatal("quest unlocked below level 11")
	}
	p.level = 11
	if !c.forestOutlawLevelUp() || p.quest(forestOutlawQuestID).Status != "START" || c.forestOutlawLevelUp() {
		t.Fatalf("level-up transition failed or repeated: %+v", p.quest(forestOutlawQuestID))
	}
	if c.forestOutlawDialog(wrongNPC, script, 25) || c.forestOutlawDialog(end, script, 10001) {
		t.Fatal("wrong NPC or pre-dialog event advanced the quest")
	}
	if !c.forestOutlawDialog(end, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 1011, forestOutlawQuestID).Data) {
		t.Fatalf("opening dialog = %x", packets.last(smDialogWindow))
	}
	if c.forestOutlawDialog(end, script, 10002) || questVar(p.quest(forestOutlawQuestID).Vars, 0) != 0 {
		t.Fatal("invalid starter dialog advanced the quest")
	}
	if !c.forestOutlawDialog(end, script, 10001) || questVar(p.quest(forestOutlawQuestID).Vars, 0) != 1 ||
		c.forestOutlawDialog(end, script, 10000) {
		t.Fatalf("starter dialog failed or repeated: %+v", p.quest(forestOutlawQuestID))
	}
	if c.forestOutlawKill(210139) || questVar(p.quest(forestOutlawQuestID).Vars, 0) != 1 {
		t.Fatal("wrong monster advanced the kill counter")
	}
	for kill := int32(1); kill <= 8; kill++ {
		if !c.forestOutlawKill(210138) || p.quest(forestOutlawQuestID).Status != "START" || questVar(p.quest(forestOutlawQuestID).Vars, 0) != kill+1 {
			t.Fatalf("210138 kill %d did not advance: %+v", kill, p.quest(forestOutlawQuestID))
		}
	}
	if c.forestOutlawKill(210140) || questVar(p.quest(forestOutlawQuestID).Vars, 0) != 9 {
		t.Fatal("210140 completed outside its fifth-kill boundary")
	}
	if !c.forestOutlawKill(210138) || p.quest(forestOutlawQuestID).Status != "REWARD" || questVar(p.quest(forestOutlawQuestID).Vars, 0) != 9 ||
		c.forestOutlawKill(210138) {
		t.Fatalf("ninth 210138 kill did not open reward once: %+v", p.quest(forestOutlawQuestID))
	}
	if !c.forestOutlawShowDialog(end, script) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, forestOutlawQuestID).Data) {
		t.Fatal("reward preview failed")
	}
	if !c.forestOutlawDialog(end, script, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, forestOutlawQuestID).Data) {
		t.Fatal("reward confirmation page failed")
	}
	beforeExp, beforeKinah := p.Exp, p.kinah.Count
	if !c.forestOutlawDialog(end, script, 8) || p.quest(forestOutlawQuestID).Status != "COMPLETE" ||
		p.Exp-beforeExp != template.Rewards[0].Experience || p.kinah.Count-beforeKinah != template.Rewards[0].Kinah {
		t.Fatalf("reward mismatch: %+v exp=%d kinah=%d", p.quest(forestOutlawQuestID), p.Exp-beforeExp, p.kinah.Count-beforeKinah)
	}
	if c.forestOutlawDialog(end, script, 8) || p.quest(forestOutlawQuestID).CompleteCount != 1 {
		t.Fatal("repeated reward dialog completed quest twice")
	}
}

func TestForestOutlawFiveKillRoute(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.quests = []store.Quest{{ID: forestOutlawQuestID, Status: "START", Vars: setQuestVar(0, 0, 1)}}
	c := &conn{s: s, player: p, tap: (&questPackets{}).tap}
	p.conn = c
	for kill := 1; kill <= 4; kill++ {
		if !c.forestOutlawKill(210140) || questVar(p.quest(forestOutlawQuestID).Vars, 0) != int32(kill+1) || p.quest(forestOutlawQuestID).Status != "START" {
			t.Fatalf("210140 kill %d did not advance: %+v", kill, p.quest(forestOutlawQuestID))
		}
	}
	if c.forestOutlawKill(210138) || p.quest(forestOutlawQuestID).Status != "START" || questVar(p.quest(forestOutlawQuestID).Vars, 0) != 5 {
		t.Fatal("210138 completed the 210140 route at variable 5")
	}
	if !c.forestOutlawKill(210140) || p.quest(forestOutlawQuestID).Status != "REWARD" || questVar(p.quest(forestOutlawQuestID).Vars, 0) != 5 {
		t.Fatalf("fifth 210140 kill did not open reward: %+v", p.quest(forestOutlawQuestID))
	}
}
