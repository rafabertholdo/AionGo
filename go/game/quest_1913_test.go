package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestDispatchToVerteronStartsForClassBranches(t *testing.T) {
	d := staticDataOrSkip(t)
	branches := []struct {
		id    int32
		class string
	}{
		{1913, "GLADIATOR"}, {1913, "TEMPLAR"},
		{1914, "ASSASSIN"}, {1914, "RANGER"},
		{1915, "SORCERER"}, {1915, "SPIRIT_MASTER"},
		{1916, "CHANTER"}, {1916, "CLERIC"},
	}
	for _, branch := range branches {
		t.Run(branch.class, func(t *testing.T) {
			s, p, c, _, saver := starterQuestPlayer(t, "ELYOS")
			p.Class = branch.class
			p.level = 10
			p.Exp = d.ExpStart(p.level)
			p.quests = append(p.quests, store.Quest{ID: 1007, Status: "COMPLETE", CompleteCount: 1})
			s.quests = saver

			c.levelUpStartQuests()
			if q := p.quest(branch.id); q == nil || q.Status != "START" {
				t.Fatalf("class %s did not start quest %d: %+v", branch.class, branch.id, q)
			}
			for _, id := range []int32{1913, 1914, 1915, 1916} {
				if id != branch.id && p.quest(id) != nil {
					t.Errorf("class %s also started quest %d", branch.class, id)
				}
			}
		})
	}

	for _, tc := range []struct {
		name   string
		class  string
		level  int
		prereq bool
	}{
		{"wrong class", "MAGE", 10, true},
		{"below level", "GLADIATOR", 9, true},
		{"missing prerequisite", "GLADIATOR", 10, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, p, c, _, _ := starterQuestPlayer(t, "ELYOS")
			p.Class, p.level = tc.class, tc.level
			if tc.prereq {
				p.quests = append(p.quests, store.Quest{ID: 1007, Status: "COMPLETE", CompleteCount: 1})
			}
			c.levelUpStartQuests()
			for _, id := range []int32{1913, 1914, 1915, 1916} {
				if p.quest(id) != nil {
					t.Errorf("ineligible player started quest %d", id)
				}
			}
		})
	}
}

func TestDispatchToVerteronDialogAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	s, p, c, packets, saver := starterQuestPlayer(t, "ELYOS")
	p.Class = "GLADIATOR"
	p.level = 10
	p.Exp = d.ExpStart(p.level)
	p.spawned = true
	p.quests = append(p.quests, store.Quest{ID: 1007, Status: "COMPLETE", CompleteCount: 1})
	s.quests = saver
	c.levelUpStartQuests()
	script := d.QuestScripts[1913]
	if script == nil || script.Kind != data.QuestCustom || script.LevelUpNPC != 203726 || script.EndNPC != 203097 || script.LevelUpWorld != 210030000 || script.LevelUpX != 1643 || script.LevelUpY != 1500 || script.LevelUpZ != 120 || script.LevelUpDelayMS != 1000 {
		t.Fatalf("Dispatch to Verteron metadata is incomplete: %+v", script)
	}
	start := questNPC(s, p, script.LevelUpNPC, 0x30191)
	end := questNPC(s, p, script.EndNPC, 0x30192)
	start.ai = newNpcAI(s, start)
	end.ai = newNpcAI(s, end)

	selectQuestDialog(c, end, 25, script.ID)
	if q := p.quest(script.ID); q.Vars != 0 || q.Status != "START" {
		t.Fatalf("wrong NPC changed quest state: %+v", q)
	}
	selectQuestDialog(c, start, 25, script.ID)
	if got := packets.last(smDialogWindow); got == nil || !bytes.Equal(got, dialogWindow(start.id, 1352, script.ID).Data) {
		t.Fatalf("dispatch page = %x", got)
	}
	selectQuestDialog(c, start, 10000, script.ID)
	if q := p.quest(script.ID); q.Status != "START" || q.Vars != 1 {
		t.Fatalf("dispatch did not advance quest: %+v", q)
	}
	if got := packets.last(smItemUsageAnimation); got == nil || !bytes.Equal(got, itemUsageAnimation(p.ID, 0, 0, 1000, 0, 0).Data) {
		t.Fatalf("dispatch teleport animation = %x", got)
	}
	// Prevent the delayed callback from despawning these unit-test NPC stubs.
	s.visMu.Lock()
	p.spawned = false
	s.visMu.Unlock()
	selectQuestDialog(c, end, 25, script.ID)
	if q := p.quest(script.ID); q.Status != "REWARD" || q.Vars != 1 {
		t.Fatalf("destination NPC did not enable reward: %+v", q)
	}
	if got := packets.last(smDialogWindow); got == nil || !bytes.Equal(got, dialogWindow(end.id, 2375, script.ID).Data) {
		t.Fatalf("turn-in page = %x", got)
	}
	selectQuestDialog(c, end, 1009, script.ID)
	if got := packets.last(smDialogWindow); got == nil || !bytes.Equal(got, dialogWindow(end.id, 5, script.ID).Data) {
		t.Fatalf("reward page = %x", got)
	}
	selectQuestDialog(c, end, 8, script.ID)
	if q := p.quest(script.ID); q.Status != "COMPLETE" || q.CompleteCount != 1 {
		t.Fatalf("dispatch quest not completed: %+v", q)
	}
	wantReward := int32(169500042)
	if count := s.countItems(p, wantReward); count != 1 {
		t.Errorf("GLADIATOR reward count = %d, want 1", count)
	}
	if len(saver.rewards) != 1 {
		t.Fatalf("completion saves = %d, want 1", len(saver.rewards))
	}

}
