package game

import "testing"

func TestProvingProficiency(t *testing.T) {
	f := newChainFixture(t, 4942)
	f.start(4762)
	f.talk(204104, 10006) // Logi before Kvasir's craft choice
	f.expectVars(0)
	f.talk(204053, 10003) // any of the six craft dialogs advances
	f.expectVars(1)
	masters := []struct {
		npc  int32
		page uint16
		item int32
	}{
		{204104, 1352, 152201596}, {204108, 1693, 152201639}, {204106, 2034, 152201615},
		{204110, 2375, 152201632}, {204100, 2716, 152201644}, {204102, 3057, 152201643},
	}
	for i, m := range masters {
		f.expectWindow(f.talk(m.npc, 25), m.page)
		f.talk(m.npc, 10006)
		f.talk(m.npc, 10006)
		f.expectVars(int32(i + 2))
		if f.s.countItems(f.p, m.item) != 1 {
			t.Fatalf("master %d handed out %d of item %d", m.npc, f.s.countItems(f.p, m.item), m.item)
		}
	}
	f.expectWindow(f.talk(798317, 33), 10001)
	f.give(186000077, 1)
	f.expectWindow(f.talk(798317, 33), 10000)
	f.expectVars(8)
	f.purify(204075, 186000084, 3825, 3739)
	f.finish(204053)
}
