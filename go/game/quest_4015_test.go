package game

import (
	"testing"
	"time"
)

func TestMissingLaborers(t *testing.T) {
	f := newChainFixture(t, 4015)
	f.start(1011)
	f.p.spawned = true
	chain := talkChains[4015]
	cage := questCatalogNPC(f.s, f.p, 730107, 0x41000)
	f.p.targetID = cage.id
	f.talk(205130, 1009) // report before searching
	f.expectVars(0)
	f.s.visMu.Lock() // the search timer runs under this lock, as the handlers do
	started := f.c.talkChainClick(cage, f.script, chain) && cage.useTask != nil
	f.s.visMu.Unlock()
	if !started {
		t.Fatal("search did not begin")
	}
	time.Sleep(3200 * time.Millisecond)
	f.s.visMu.Lock()
	defer f.s.visMu.Unlock()
	f.expectVars(1)
	if f.c.talkChainClick(cage, f.script, chain) {
		t.Fatal("searched twice")
	}
	f.expectWindow(f.talk(205130, 25), 2375)
	f.expectWindow(f.talk(205130, 1009), 5)
	if q := f.p.quest(4015); q.Status != "REWARD" {
		t.Fatalf("state %+v", q)
	}
	f.finish(205130)
}
