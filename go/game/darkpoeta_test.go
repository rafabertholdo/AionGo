package game

import (
	"testing"
	"time"
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
