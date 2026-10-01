package game

import (
	"slices"
	"testing"

	"aionlightning/game/store"
)

// auditRun sends a dialog to the npc through the real framework (click = -1) and returns the windows sent.
func auditRun(f *conformanceFixture, npcID int32, dialog int32, quest int32) [][2]int32 {
	f.packets.frames = nil
	for _, npc := range f.npcs {
		if npc.npc.ID != npcID {
			continue
		}
		if dialog == -1 {
			f.c.showDialog(dialogRequest(cmShowDialog, npc.id, 0, 0))
		} else {
			f.c.dialogSelect(dialogRequest(cmDialogSelect, npc.id, uint16(dialog), quest))
		}
	}
	return f.packets.windows(0)
}

func auditFixture(t *testing.T, id int32, status string, vars int32) *conformanceFixture {
	t.Helper()
	d := staticDataOrSkip(t)
	f, _ := newConformanceFixture(d, d.QuestScripts[id], status, vars)
	return f
}

func auditWant(t *testing.T, what string, got [][2]int32, want ...[2]int32) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s: windows %v, want %v", what, got, want)
	}
}

// Java _1014 has no case -1 (the click is the main menu) and its cases 1013/10000/10001 fall through.
func TestDukakiOdiumClickIsMenuAndCasesFallThrough(t *testing.T) {
	f := auditFixture(t, 1014, "START", 0)
	auditWant(t, "Pernos click", auditRun(f, 203129, -1, 0), [2]int32{10, 0})
	auditWant(t, "Pernos 25", auditRun(f, 203129, 25, 1014), [2]int32{1011, 1014})
	f = auditFixture(t, 1014, "START", 1)
	auditWant(t, "Hyan click", auditRun(f, 730020, -1, 0), [2]int32{10, 0})
	auditWant(t, "Hyan 25", auditRun(f, 730020, 25, 1014), [2]int32{1352, 1014})
	// var 10 + dialog 10000 falls into case 10001 (var == 10 -> 11).
	f = auditFixture(t, 1014, "START", 10)
	auditWant(t, "10000 at var 10", auditRun(f, 203129, 10000, 1014), [2]int32{10, 0})
	if q := f.p.quest(1014); q.Vars != 11 {
		t.Errorf("10000 at var 10 left vars %d, want 11", q.Vars)
	}
}

// Java _1018: START click has no case; case 25 with var != 0 falls into case 33 (missing items -> page 1097).
func TestMarkOfVengeanceClickIsMenu(t *testing.T) {
	f := auditFixture(t, 1018, "START", 0)
	auditWant(t, "click", auditRun(f, 203098, -1, 0), [2]int32{10, 0})
	auditWant(t, "25", auditRun(f, 203098, 25, 1018), [2]int32{1011, 1018})
}

// Java _1097: REWARD click is defaultQuestEndDialog (window 5); 25 gives page 10002.
func TestSwordTranscendenceRewardClickIsWindow5(t *testing.T) {
	f := auditFixture(t, 1097, "REWARD", 3)
	auditWant(t, "click", auditRun(f, 790001, -1, 0), [2]int32{5, 1097})
	auditWant(t, "25", auditRun(f, 790001, 25, 1097), [2]int32{10002, 1097})
	auditWant(t, "1009", auditRun(f, 790001, 1009, 1097), [2]int32{5, 1097})
}

// Java _1114 REWARD: Namus needs var 4 (click -> 2375, 1009 -> 6), Asteros var 3 (click -> 5).
func TestNymphsGownRewardVarsAndWindows(t *testing.T) {
	f := auditFixture(t, 1114, "REWARD", 4)
	auditWant(t, "Namus click", auditRun(f, 203075, -1, 0), [2]int32{2375, 1114})
	auditWant(t, "Namus 1009", auditRun(f, 203075, 1009, 1114), [2]int32{6, 1114})
	f = auditFixture(t, 1114, "REWARD", 3)
	auditWant(t, "Asteros click", auditRun(f, 203058, -1, 0), [2]int32{5, 1114})
}

// Java _1141 barrel: the brace-less else sends window 10 before defaultQuestEndDialog for everything but the
// var-0 click.
func TestBelbuasTreasureBarrelWindows(t *testing.T) {
	f := auditFixture(t, 1141, "START", 0)
	auditWant(t, "click", auditRun(f, 700122, -1, 0), [2]int32{5, 1141})
	if q := f.p.quest(1141); q.Status != "REWARD" || q.Vars != 2 {
		t.Errorf("click left %+v", q)
	}
	auditWant(t, "click again", auditRun(f, 700122, -1, 0), [2]int32{10, 0}, [2]int32{5, 1141})
	auditWant(t, "1009", auditRun(f, 700122, 1009, 1141), [2]int32{10, 0}, [2]int32{5, 1141})
	f = auditFixture(t, 1141, "START", 1)
	auditWant(t, "1999", auditRun(f, 700122, 1999, 1141), [2]int32{10, 0}, [2]int32{1999, 1141})
	_ = store.Quest{}
}
