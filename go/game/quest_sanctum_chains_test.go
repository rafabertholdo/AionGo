package game

import "testing"

// walk drives a plain "advance" step: page on 25, out-of-order ignored, dialog advances.
func (f *chainFixture) advance(npc int32, page, dialog uint16, wantVars int32) {
	f.t.Helper()
	f.expectWindow(f.talk(npc, 25), page)
	f.talk(npc, dialog)
	f.expectVars(wantVars)
}

func TestSanctumTemplarsAndBurden(t *testing.T) {
	f := newChainFixture(t, 3934)
	f.start(4762)
	f.talk(798360, 10001) // out of order
	f.expectVars(0)
	for i, npc := range []int32{798359, 798360, 798361, 798362, 798363, 798364, 798365, 798366} {
		f.talk(npc, uint16(10000+i))
		f.expectVars(int32(i + 1))
	}
	f.purify(203752, 186000080, 3825, 3739)
	f.finish(203701)

	f = newChainFixture(t, 3935)
	f.start(4762)
	f.advance(203316, 1011, 10000, 1)
	f.advance(203702, 1352, 10001, 2)
	f.advance(203329, 1693, 10002, 3)
	f.expectWindow(f.talk(203329, 33), 10001)
	f.give(186000079, 30)
	f.expectWindow(f.talk(203329, 33), 10000)
	f.expectVars(4)
	f.purify(203752, 186000080, 2461, 2375)
	f.finish(203701)
}

func TestDecorationsOfSanctum(t *testing.T) {
	f := newChainFixture(t, 3936)
	f.start(4762)
	f.expectWindow(f.talk(203710, 25), 1011)
	f.expectWindow(f.talk(203710, 10000), 1352)
	f.expectWindow(f.talk(203710, 33), 10001)
	for _, id := range []int32{182206091, 182206092, 182206093, 182206094} {
		f.give(id, 10)
	}
	f.expectWindow(f.talk(203710, 33), 5)
	if q := f.p.quest(3936); q.Status != "REWARD" || f.s.countItems(f.p, 182206091) != 0 {
		t.Fatalf("state %+v", q)
	}
	f.finish(203710)
}

func TestWellRoundedAndPersistenceAndLuck(t *testing.T) {
	f := newChainFixture(t, 3938)
	f.start(4762)
	f.talk(203701, 10003)
	f.expectVars(1)
	for i, m := range []struct {
		npc  int32
		page uint16
		item int32
	}{{203788, 1352, 152201596}, {203792, 1693, 152201639}, {203790, 2034, 152201615},
		{203793, 2375, 152201632}, {203784, 2716, 152201644}, {203786, 3057, 152201643}} {
		f.expectWindow(f.talk(m.npc, 25), m.page)
		f.talk(m.npc, 10006)
		f.expectVars(int32(i + 2))
		if f.s.countItems(f.p, m.item) != 1 {
			t.Fatalf("npc %d gave %d", m.npc, f.s.countItems(f.p, m.item))
		}
	}
	f.give(186000077, 1)
	f.expectWindow(f.talk(798316, 33), 10000)
	f.purify(203752, 186000081, 3825, 3739)
	f.finish(203701)

	f = newChainFixture(t, 3939)
	f.start(4762)
	f.advance(203780, 1011, 10000, 1)
	f.expectWindow(f.talk(203781, 25), 1352)
	f.expectWindow(f.talk(203781, 10001), 1438) // no kinah
	f.expectVars(1)
	f.p.kinah.Count = 3400000
	f.talk(203781, 10001)
	f.expectVars(2)
	if f.p.kinah.Count != 0 {
		t.Fatalf("kinah %d", f.p.kinah.Count)
	}
	f.expectWindow(f.talk(203780, 33), 10001)
	f.give(182206098, 20)
	f.expectWindow(f.talk(203780, 33), 10000)
	f.expectVars(3)
	f.purify(203752, 186000080, 2120, 2034)
	f.finish(203701)
}

func TestGalleriaUniformAndDyeBox(t *testing.T) {
	f := newChainFixture(t, 3965)
	f.p.Gender = "FEMALE"
	f.start(1011)
	if n := f.s.countItems(f.p, 182206120); n != 2 {
		t.Fatalf("start gave %d vouchers", n)
	}
	f.talk(798390, 1009) // Palentine before Andu
	f.expectVars(0)
	f.expectWindow(f.talk(798391, 25), 1352)
	f.talk(798391, 10000)
	f.expectVars(1)
	if n := f.s.countItems(f.p, 182206120); n != 1 {
		t.Fatalf("Andu left %d vouchers", n)
	}
	f.expectWindow(f.talk(798390, 25), 2375)
	f.expectWindow(f.talk(798390, 1009), 5)
	if q := f.p.quest(3965); q.Status != "REWARD" || f.s.countItems(f.p, 182206120) != 0 {
		t.Fatalf("state %+v", q)
	}
	f.finish(798390)

	f = newChainFixture(t, 3966)
	f.p.Gender = "MALE"
	f.start(1011)
	f.advance(203994, 1352, 10000, 1)
	f.advance(204030, 1693, 10001, 2)
	f.talk(204568, 10002)
	if q := f.p.quest(3966); q.Status != "REWARD" {
		t.Fatalf("state %+v", q)
	}
	if o := f.npcs[798391]; !f.c.customQuestShowDialog(o, f.script) {
		t.Fatal("no reward window on click")
	}
	f.expectWindow(f.npcs[798391], 2375)
	f.finish(798391)

	// 3967 starts only after 3966.
	f2 := newChainFixture(t, 3967)
	f2.p.Gender = "MALE"
	f2.p.quests = f.p.quests
	f2.start(1011)
	f2.expectWindow(f2.talk(798309, 25), 1352)
	f2.talk(798309, 10000)
	if q := f2.p.quest(3967); q.Status != "REWARD" || f2.s.countItems(f2.p, 182206122) != 1 {
		t.Fatalf("state %+v", q)
	}
	f2.talk(798391, 1009)
	if f2.s.countItems(f2.p, 182206122) != 0 {
		t.Fatal("box not taken")
	}
	f2.finish(798391)

	f3 := newChainFixture(t, 3967)
	f3.p.Gender = "MALE"
	f3.p.quests = nil // fixture pre-completes 3966
	f3.talk(798391, 1002)
	if q := f3.p.quest(3967); q != nil {
		t.Fatalf("started without 3966: %+v", q)
	}
}
