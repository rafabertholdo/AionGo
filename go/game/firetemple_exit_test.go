package game

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestFireTempleExit(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, tc := range []struct {
		name, race string
		world      int32
		moveAway   bool
	}{
		{"Elyos", "ELYOS", 210020000, false},
		{"Asmodians", "ASMODIANS", 220020000, false},
		{"moved away", "ELYOS", 210020000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := testServer(d)
				p, _ := fighter(t, s, 1000)
				p.Race = tc.race
				s.spawn(p)
				s.visMu.Lock()
				in := s.newInstance(fireTempleWorld)
				defer func() { s.visMu.Lock(); s.destroyInstance(in); s.visMu.Unlock() }()
				var exit *object
				count := 0
				for _, o := range s.byID {
					if o.worldID == in.world && o.instance == in.id && o.npc != nil && o.npc.ID == 730048 {
						exit = o
						count++
					}
				}
				if count != 1 {
					s.visMu.Unlock()
					t.Fatalf("Fire Temple exit count = %d, want 1", count)
				}
				portal := d.InstancePortal(in.world, tc.race)
				arrival := portal.Exit
				if distance3D(arrival.X, arrival.Y, arrival.Z, exit.x, exit.y, exit.z) > 10 {
					s.visMu.Unlock()
					t.Fatal("exit is not reachable from the arrival point")
				}
				s.changePosition(p, in.world, in.id, arrival.X, arrival.Y, arrival.Z, 0)
				s.spawnLocked(p)
				s.visMu.Unlock()
				if !p.conn.portalDialog(exit.id) {
					t.Fatal("Fire Temple exit did not accept interaction")
				}
				if tc.moveAway {
					s.visMu.Lock()
					p.X = exit.x + 11
					s.visMu.Unlock()
				}
				time.Sleep(3 * time.Second)
				synctest.Wait()
				if tc.moveAway {
					if p.WorldID != in.world || p.instance != in.id {
						t.Fatal("moving away did not cancel the exit")
					}
					return
				}
				if p.WorldID != tc.world || p.instance != 0 {
					t.Fatalf("exit map=%d instance=%d, want map=%d instance=0", p.WorldID, p.instance, tc.world)
				}
				point := portal.EntryFor(tc.race)
				if p.X != point.X || p.Y != point.Y || p.Z != point.Z {
					t.Fatal("exit did not return to the faction entrance")
				}
			})
		})
	}
}
