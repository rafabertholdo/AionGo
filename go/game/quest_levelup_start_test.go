package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestLevelUpStartQuestCatalogAndEligibility(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, tc := range []struct {
		id    int32
		class string
	}{
		{2901, "GLADIATOR"},
		{2902, "ASSASSIN"},
		{2903, "SORCERER"},
		{2904, "CHANTER"},
	} {
		script := d.QuestScripts[tc.id]
		if script == nil || !script.LevelUpStart || script.NPCStart || script.LevelUpNPC != 204191 || script.EndNPC != 203559 || script.LevelUpWorld != 220030000 || script.LevelUpX != 1748 || script.LevelUpY != 1807 || script.LevelUpZ != 255 || script.LevelUpDelayMS != 1000 {
			t.Fatalf("quest %d level-up metadata mismatch: %+v", tc.id, script)
		}
		if got := d.QuestCustomTalks[204191]; !hasLevelUpScript(got, tc.id) {
			t.Fatalf("quest %d is not indexed at its level-up NPC", tc.id)
		}
		if got := d.QuestEnds[203559]; !hasLevelUpScript(got, tc.id) {
			t.Fatalf("quest %d is not indexed at its final NPC", tc.id)
		}
		s := testServer(d)
		p := wrathchild(s)
		p.Race, p.Class, p.level = "ASMODIANS", tc.class, 9
		p.quests = []store.Quest{{ID: 2009, Status: "COMPLETE"}}
		c := &conn{s: s, player: p}
		c.levelUpStartQuests()
		if q := p.quest(tc.id); q == nil || q.Status != "START" {
			t.Fatalf("eligible quest %d did not start: %+v", tc.id, q)
		}
		c.levelUpStartQuests()
		count := 0
		for _, q := range p.quests {
			if q.ID == tc.id {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("quest %d started %d times", tc.id, count)
		}
	}

	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ASMODIANS", "GLADIATOR", 9
	p.quests = []store.Quest{{ID: 2009, Status: "COMPLETE"}}
	c := &conn{s: s, player: p}
	c.levelUpStartQuests()
	for _, id := range []int32{2902, 2903, 2904} {
		if p.quest(id) != nil {
			t.Fatalf("class-ineligible quest %d started", id)
		}
	}
	p = wrathchild(s)
	p.Race, p.Class, p.level = "ASMODIANS", "GLADIATOR", 9
	c.player = p
	c.levelUpStartQuests()
	if p.quest(2901) != nil {
		t.Fatal("quest started without its finished-quest prerequisite")
	}
	p.Race, p.quests = "ELYOS", []store.Quest{{ID: 2009, Status: "COMPLETE"}}
	c.levelUpStartQuests()
	if p.quest(2901) != nil {
		t.Fatal("Asmodian quest started for Elyos")
	}
}

func TestLevelUpStartQuestDialogAndTeleport(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ASMODIANS", "GLADIATOR", 9
	p.quests = []store.Quest{{ID: 2009, Status: "COMPLETE"}}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	c.levelUpStartQuests()
	if q := p.quest(2901); q == nil || q.Status != "START" {
		t.Fatal("level-up did not start Dispatch to Altgard")
	}
	script := d.QuestScripts[2901]
	start := questCatalogNPC(s, p, script.LevelUpNPC, 0x40001)
	end := questCatalogNPC(s, p, script.EndNPC, 0x40002)
	if !c.levelUpQuestDialog(start, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1352, 2901).Data) {
		t.Fatal("initial dialog page did not match 1352")
	}
	instant := *script
	instant.LevelUpDelayMS = 0
	if !c.levelUpQuestDialog(start, &instant, 10000) || questVar(p.quest(2901).Vars, 0) != 1 {
		t.Fatal("teleport transition did not advance quest variable")
	}
	if !c.levelUpQuestDialog(end, script, 25) || p.quest(2901).Status != "REWARD" {
		t.Fatal("final NPC did not unlock reward")
	}
	if !c.levelUpQuestDialog(end, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, 2901).Data) {
		t.Fatal("reward dialog did not show default page")
	}
	if c.levelUpQuestDialog(start, script, 10000) || p.quest(2901).Status != "REWARD" {
		t.Fatal("wrong NPC advanced completed stage")
	}
}

func hasLevelUpScript(scripts []*data.QuestScript, id int32) bool {
	for _, script := range scripts {
		if script.ID == id {
			return true
		}
	}
	return false
}
