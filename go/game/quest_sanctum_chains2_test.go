package game

import "testing"

func TestClassPreceptorConsent(t *testing.T) {
	f := newChainFixture(t, 3933)
	f.start(4762)
	f.talk(203705, 10001) // out of order
	f.expectVars(0)
	for i, npc := range []int32{203704, 203705, 203706, 203707} {
		f.talk(npc, uint16(10000+i))
		f.expectVars(int32(i + 1))
	}
	f.purify(203752, 186000080, 2461, 2375)
	f.finish(203701)
}

func TestStopTheShulacks(t *testing.T) {
	f := newChainFixture(t, 3932)
	f.start(1011)
	f.talk(203711, 33) // not yet var 1: ignored
	f.expectVars(0)
	f.expectWindow(f.talk(204656, 25), 1352)
	f.talk(204656, 10000)
	f.expectVars(1)
	f.expectWindow(f.talk(203711, 25), 2375)
	f.expectWindow(f.talk(203711, 33), 2716)
	f.give(182206082, 100)
	f.give(182206083, 1)
	f.expectWindow(f.talk(203711, 33), 5)
	if q := f.p.quest(3932); q.Status != "REWARD" || f.s.countItems(f.p, 182206082) != 0 {
		t.Fatalf("state %+v", q)
	}
	f.finish(203711)
}

func TestStigmaQuests(t *testing.T) {
	f := newChainFixture(t, 3931)
	f.start(4762)
	f.expectWindow(f.talk(798321, 25), 1011)
	f.talk(798321, 10000)
	f.expectVars(1)
	f.expectWindow(f.talk(798321, 33), 10001)
	f.give(182206077, 4)
	f.expectWindow(f.talk(798321, 33), 10000)
	f.expectVars(2)
	if f.s.countItems(f.p, 182206080) != 1 || f.s.countItems(f.p, 182206077) != 0 {
		t.Fatal("belt not swapped for drops")
	}
	f.expectWindow(f.talk(279005, 25), 1693)
	f.talk(279005, 10255)
	if q := f.p.quest(3931); q.Status != "REWARD" || f.s.countItems(f.p, 182206080) != 0 || f.s.countItems(f.p, 182206081) != 1 {
		t.Fatalf("state %+v", q)
	}
	f.expectWindow(f.talk(203711, 1009), 5)
	f.finish(203711)

	f = newChainFixture(t, 3930)
	f.start(4762)
	f.advance(203833, 1011, 10000, 1)
	f.advance(798321, 1352, 10001, 2)
	f.expectWindow(f.talk(798321, 25), 1693)
	f.expectWindow(f.talk(798321, 33), 10001)
	f.give(182206075, 1)
	f.expectWindow(f.talk(798321, 33), 10000)
	if q := f.p.quest(3930); q.Status != "REWARD" || f.s.countItems(f.p, 182206075) != 0 {
		t.Fatalf("state %+v", q)
	}
	f.finish(203711)
}

func TestOrderForGojirunerk(t *testing.T) {
	f := newChainFixture(t, 3319)
	f.start(1011)
	f.talk(798050, 1009) // too early
	f.expectVars(0)
	f.advance(798138, 1352, 10000, 1)
	f.expectWindow(f.talk(798050, 25), 2375)
	f.talk(798050, 1009)
	f.finish(798050)
}

func TestSecretDumplingRecipe(t *testing.T) {
	f := newChainFixture(t, 3093)
	f.expectWindow(f.talk(798185, 25), 1011)
	f.talk(798185, 1002)
	f.expectVars(1)
	if f.s.countItems(f.p, 182206062) != 1 {
		t.Fatal("recipe not handed out")
	}
	f.advance(798177, 1352, 10000, 2)
	f.advance(798179, 1693, 10001, 3)
	f.expectWindow(f.talk(203784, 25), 2034)
	f.talk(203784, 10002)
	if q := f.p.quest(3093); q.Status != "REWARD" || f.s.countItems(f.p, 182208052) != 1 {
		t.Fatalf("state %+v", q)
	}
	f.expectWindow(f.talk(798179, 1009), 5) // any of the NPCs pays out
	f.finish(798185)
}

func TestTheShugoMenace(t *testing.T) {
	f := newChainFixture(t, 3326)
	f.expectWindow(f.talk(798053, 25), 4)
	f.talk(798053, 1002)
	f.expectVars(0)
	f.expectWindow(f.talk(798053, 25), 10002)
	f.talk(798053, 1009) // too early
	f.expectVars(0)
	for i := 0; i < 25; i++ {
		f.s.visMu.Lock()
		f.s.recordQuestKill(questCatalogNPC(f.s, f.p, []int32{210897, 210939, 210873, 210919, 211754}[i%5], int32(0x33000+i)), f.p)
		f.s.visMu.Unlock()
	}
	f.expectVars(20)
	f.expectWindow(f.talk(798053, 1009), 5)
	if q := f.p.quest(3326); q.Status != "REWARD" {
		t.Fatalf("state %+v", q)
	}
	f.finish(798053)
}

func TestBalaurReport(t *testing.T) {
	f := newChainFixture(t, 3914)
	const report = 182206084
	f.give(report, 1)
	if !f.c.questStartItemUse(f.p.cube[len(f.p.cube)-1], f.script) {
		t.Fatal("report item unusable")
	}
	f.expectWindow(&object{id: 0}, 4)
	if !f.c.itemStartedQuestDialog(f.script, 1002) {
		t.Fatal("start refused")
	}
	f.expectVars(0)
	f.talk(203384, 17) // not in reward yet
	f.expectWindow(f.talk(203752, 25), 1352)
	f.talk(203752, 10000)
	if q := f.p.quest(3914); q.Status != "REWARD" {
		t.Fatalf("state %+v", q)
	}
	f.finish(203384)
}
