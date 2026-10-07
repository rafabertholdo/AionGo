package game

import (
	"testing"
	"testing/synctest"
	"time"

	"aionlightning/game/store"
)

// TestDarkPoeta runs Dark Poeta: the score shows on entering, a door starts the run, the generators bring out
// Anuhart, and his death ranks the run and brings out the rank's boss.
func TestDarkPoeta(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	s.spawn(p)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	in := s.newInstance(darkPoetaWorld)
	defer s.destroyInstance(in)
	find := func(npcID int32) *object {
		for _, o := range s.byID {
			if o.worldID == darkPoetaWorld && o.instance == in.id && o.npc != nil && o.npc.ID == npcID && !o.dead {
				return o
			}
		}
		return nil
	}
	if find(npcAnuhart) != nil || find(215280) != nil {
		t.Fatal("Anuhart or a rank boss stands before the run")
	}
	s.changePosition(p, darkPoetaWorld, in.id, 1224, 418, 140, 0)
	s.spawnLocked(p)
	s.darkPoetaEnter(p)
	if tap.count(smInstanceScore) != 1 || in.dp.state != scorePreparing {
		t.Fatalf("no preparing score on entering: %v", tap.counts)
	}
	s.darkPoetaDoor(p)
	if in.dp.state != scoreStarted || in.dp.timeLeft() <= darkPoetaRun-time.Minute {
		t.Fatalf("the door didn't start the run: state %#x, %v left", in.dp.state, in.dp.timeLeft())
	}
	for _, id := range []int32{214895, 214896, 214897} {
		g := find(id)
		if g == nil {
			t.Fatalf("no generator %d", id)
		}
		s.npcDied(g, p)
		if g.respawn != nil {
			t.Fatalf("generator %d will respawn", id)
		}
	}
	anuhart := find(npcAnuhart)
	if anuhart == nil || in.dp.points != 377+377+330 || in.dp.kills != 3 {
		t.Fatalf("after the generators: Anuhart %v, %d points, %d kills", anuhart != nil, in.dp.points, in.dp.kills)
	}
	in.dp.points = 17000 // enough with Anuhart's 954 for S
	s.npcDied(anuhart, p)
	if decayTime(anuhart) != 4*time.Minute {
		t.Errorf("Anuhart's corpse decays in %v, taking his loot", decayTime(anuhart))
	}
	if in.dp.state != scoreEnded || in.dp.rank != 1 || find(215280) == nil {
		t.Fatalf("Anuhart's death: state %#x, rank %d, Tahabata %v", in.dp.state, in.dp.rank, find(215280) != nil)
	}
	gates := 0
	for _, o := range s.byID {
		if o.worldID == darkPoetaWorld && o.instance == in.id && o.npc != nil && o.npc.ID == 730211 && o.x > 1000 {
			gates++
			if !s.darkPoetaExit(p, o) || p.WorldID == darkPoetaWorld {
				t.Errorf("the gate in Tahabata's room didn't take the player out: map %d", p.WorldID)
			}
			s.changePosition(p, darkPoetaWorld, in.id, 1224, 418, 140, 0)
		}
	}
	if gates == 0 {
		t.Error("no abyss gate in Tahabata's room")
	}
	left := in.dp.timeLeft()
	if s.darkPoetaKill(find(215280)); in.dp.points != 17954 || in.dp.timeLeft() != left {
		t.Errorf("the run kept scoring or counting after it ended")
	}
}

func TestDarkPoetaRank(t *testing.T) {
	for _, c := range []struct {
		left   time.Duration
		points int32
		rank   int32
	}{
		{3 * time.Hour, 17817, 1}, {3 * time.Hour, 17816, 2}, {100 * time.Minute, 20000, 2},
		{61 * time.Minute, 10914, 3}, {61 * time.Minute, 10913, 4}, {31 * time.Minute, 0, 5}, {0, 99999, rankFailed},
	} {
		if rank, _ := darkPoetaRank(c.left, c.points); rank != c.rank {
			t.Errorf("%v left, %d points: rank %d, want %d", c.left, c.points, rank, c.rank)
		}
	}
}

func TestDarkPoetaRankBossUsesAuthoredSpawn(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	in := s.newInstance(darkPoetaWorld)
	defer s.destroyInstance(in)
	for _, id := range []int32{215280, 215281, 215282, 215283} {
		var spotFound bool
		for _, group := range d.Spawns[darkPoetaWorld] {
			if group.NpcID != id || len(group.Spots) == 0 {
				continue
			}
			spot := group.Spots[0]
			s.darkPoetaSpawnRankBoss(in, id)
			var found bool
			for _, o := range s.byID {
				if o.instance == in.id && o.npc != nil && o.npc.ID == id {
					found = true
					if o.x != spot.X || o.y != spot.Y || o.z != spot.Z || o.heading != byte(spot.Heading) {
						t.Errorf("boss %d spawned at (%v,%v,%v,%d), want (%v,%v,%v,%d)", id, o.x, o.y, o.z, o.heading, spot.X, spot.Y, spot.Z, spot.Heading)
					}
				}
			}
			if !found {
				t.Errorf("boss %d was not spawned", id)
			}
			spotFound = true
			break
		}
		if !spotFound {
			t.Errorf("boss %d has no authored spawn", id)
		}
	}
}

func TestDarkPoetaRepeatedDeathDoesNotScoreOrAdvanceAgain(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	in := s.newInstance(darkPoetaWorld)
	defer s.destroyInstance(in)
	s.darkPoetaStart(in)
	find := func(id int32) *object {
		for _, o := range s.byID {
			if o.worldID == darkPoetaWorld && o.instance == in.id && o.npc != nil && o.npc.ID == id {
				return o
			}
		}
		return nil
	}
	generator := find(214895)
	if generator == nil {
		t.Fatal("missing main generator fixture")
	}
	s.darkPoetaKill(generator)
	s.darkPoetaKill(generator)
	if in.dp.points != 377 || in.dp.kills != 1 {
		t.Fatalf("duplicate death: points=%d, kills=%d; want 377, 1", in.dp.points, in.dp.kills)
	}
	s.darkPoetaKill(find(214896))
	if find(npcAnuhart) != nil {
		t.Fatal("two unique generators plus a repeated event spawned Anuhart")
	}
	s.darkPoetaKill(find(214897))
	if find(npcAnuhart) == nil {
		t.Fatal("three unique generators did not spawn Anuhart")
	}
}

func TestDarkPoetaGatherOnlyWhileRunning(t *testing.T) {
	s := testServer(nil)
	in := &instance{world: darkPoetaWorld, id: 7, dp: &darkPoeta{state: scorePreparing, since: time.Now()}}
	s.instances = map[[2]int32]*instance{{darkPoetaWorld, 7}: in}
	p := &player{character: &character{Character: &store.Character{WorldID: darkPoetaWorld}}, instance: 7}
	s.darkPoetaGather(p)
	if in.dp.gather != 0 {
		t.Fatal("gather credited before the run started")
	}
	in.dp.state = scoreStarted
	s.darkPoetaGather(p)
	if in.dp.gather != 1 {
		t.Fatal("running gather not credited")
	}
	in.dp.since = time.Now().Add(-darkPoetaRun)
	s.darkPoetaGather(p)
	if in.dp.gather != 1 || in.dp.state != scoreEnded || in.dp.rank != rankFailed {
		t.Fatalf("expired gathering: count=%d state=%#x rank=%d", in.dp.gather, in.dp.state, in.dp.rank)
	}
}

func TestDarkPoetaExpiresWithoutPlayerEvents(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := testServer(nil)
		in := &instance{world: darkPoetaWorld, id: 7, dp: &darkPoeta{state: scorePreparing}}
		s.instances = map[[2]int32]*instance{{darkPoetaWorld, 7}: in}
		s.visMu.Lock()
		s.darkPoetaStart(in)
		s.visMu.Unlock()
		time.Sleep(darkPoetaRun - time.Second)
		synctest.Wait()
		s.visMu.Lock()
		started := in.dp.state == scoreStarted
		s.visMu.Unlock()
		if !started {
			t.Fatal("run ended before the deadline")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		s.visMu.Lock()
		defer s.visMu.Unlock()
		if in.dp.state != scoreEnded || in.dp.rank != rankFailed || in.dp.timeLeft() != 0 {
			t.Fatalf("expiry: state=%#x rank=%d left=%v", in.dp.state, in.dp.rank, in.dp.timeLeft())
		}
	})
}

func TestDarkPoetaStaleExpiryCannotEndReplacementRun(t *testing.T) {
	s := testServer(nil)
	stale := &instance{world: darkPoetaWorld, id: 7, dp: &darkPoeta{state: scoreStarted}}
	current := &instance{world: darkPoetaWorld, id: 7, dp: &darkPoeta{state: scoreStarted, since: time.Now()}}
	s.instances = map[[2]int32]*instance{{darkPoetaWorld, 7}: current}
	s.darkPoetaExpire(stale)
	if current.dp.state != scoreStarted {
		t.Fatal("stale expiry ended replacement run")
	}
}

func TestDarkPoetaStaleStartCannotStartReplacementRun(t *testing.T) {
	s := testServer(nil)
	stale := &instance{world: darkPoetaWorld, id: 7, dp: &darkPoeta{state: scorePreparing}}
	current := &instance{world: darkPoetaWorld, id: 7, dp: &darkPoeta{state: scorePreparing, since: time.Now()}}
	s.instances = map[[2]int32]*instance{{darkPoetaWorld, 7}: current}
	s.darkPoetaStart(stale)
	if stale.dp.state != scorePreparing || current.dp.state != scorePreparing {
		t.Fatal("stale start changed a discarded or replacement run")
	}
}

func TestDarkPoetaRespawnCanScoreANewDeath(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	in := s.newInstance(darkPoetaWorld)
	defer s.destroyInstance(in)
	s.darkPoetaStart(in)
	var o *object
	for _, candidate := range s.byID {
		if candidate.worldID == darkPoetaWorld && candidate.instance == in.id && candidate.npc != nil && candidate.npc.ID == 214853 {
			o = candidate
			break
		}
	}
	if o == nil {
		t.Fatal("missing Anuhart Biteblade fixture")
	}
	s.darkPoetaKill(o)
	s.darkPoetaKill(o)
	s.respawnNpc(o)
	s.darkPoetaKill(o)
	if in.dp.kills != 2 {
		t.Fatalf("respawned NPC death count = %d; want 2", in.dp.kills)
	}
}

func TestDarkPoetaDestroyCancelsNpcWork(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		s.visMu.Lock()
		in := s.newInstance(darkPoetaWorld)
		var o *object
		for _, candidate := range s.byID {
			if candidate.npc != nil && candidate.npc.ID == 214853 && candidate.instance == in.id {
				o = candidate
				break
			}
		}
		if o == nil {
			t.Fatal("missing NPC fixture")
		}
		fired := false
		o.ai.schedule()
		o.ai.talkTask = s.later(time.Second, func() { fired = true })
		sk := &skill{s: s, tmpl: d.Skills[18532], effector: o, level: 1}
		sk.cast = s.later(time.Second, func() { fired = true })
		o.cast = sk
		// Dead NPCs must be cleaned up too; evDespawn must bypass the dead guard.
		o.dead = true
		s.destroyInstance(in)
		s.visMu.Unlock()
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if fired || o.cast != nil || o.ai.scheduled() {
			t.Fatal("destroyed instance retained NPC work")
		}
		if !in.dp.start.cancelled {
			t.Fatal("destroyed run retained preparation timer")
		}
	})
}
