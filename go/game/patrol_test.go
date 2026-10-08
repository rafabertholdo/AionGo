package game

import (
	"testing"
	"testing/synctest"
	"time"
)

// Exercise the actual dungeon spawn and the full out-and-back route while a player watches
// from above the aggro range. Visibility must not freeze or restart the route.
func TestFireTempleRotanPatrolWithObserver(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		s.visMu.Lock()
		in := s.newInstance(fireTempleWorld)
		defer func() {
			s.visMu.Lock()
			s.destroyInstance(in)
			s.visMu.Unlock()
		}()
		var rotan *object
		patrollers := 0
		for _, o := range s.byID {
			if o.worldID == fireTempleWorld && o.instance == in.id && o.walker > 0 {
				patrollers++
				if len(d.Walkers[o.walker]) == 0 || !o.ai.has(&walkDesire{}) {
					s.visMu.Unlock()
					t.Fatalf("npc %d has no usable patrol", o.npc.ID)
				}
				if o.npc.ID == 212844 {
					rotan = o
				}
			}
		}
		if rotan == nil || patrollers < 2 || len(d.Walkers[rotan.walker]) < 12 {
			s.visMu.Unlock()
			t.Fatal("Fire Temple patrol data is missing")
		}
		p, _ := fighter(t, s, 240)
		p.adminInvulnerable = true
		defer func() {
			s.visMu.Lock()
			p.fx.removeAll()
			s.visMu.Unlock()
		}()
		p.WorldID, p.instance, p.Y, p.Z = fireTempleWorld, in.id, 235, 155
		s.spawnLocked(p)
		if p.seen[rotan.id] == nil {
			s.visMu.Unlock()
			t.Fatal("observer cannot see Rotan")
		}
		routeSteps := len(d.Walkers[rotan.walker])
		startX, startY := rotan.x, rotan.y
		s.visMu.Unlock()

		visited := map[int]bool{}
		for range 1200 {
			time.Sleep(time.Second)
			synctest.Wait()
			s.visMu.Lock()
			for _, desire := range rotan.ai.desires {
				if walk, ok := desire.(*walkDesire); ok && walk.walkingToNext {
					visited[walk.target] = true
				}
			}
			s.visMu.Unlock()
			if len(visited) == routeSteps {
				break
			}
		}
		s.visMu.Lock()
		if len(visited) != routeSteps || rotan.x == startX && rotan.y == startY {
			t.Errorf("Rotan visited %d/%d patrol legs while observed", len(visited), routeSteps)
		}
		if rotan.ai.state != aiActive || len(rotan.aggro) != 0 {
			t.Error("Rotan aggroed an observer outside aggro range")
		}
		var walk desire
		for _, desire := range rotan.ai.desires {
			if _, ok := desire.(*walkDesire); ok {
				walk = desire
			}
		}
		s.forgetObject(p, rotan, deleteLeaving)
		rotan.ai.run()
		if rotan.ai.state != aiActive || !rotan.ai.has(&walkDesire{}) {
			t.Error("losing the observer interrupted the patrol")
		}
		for _, desire := range rotan.ai.desires {
			if _, ok := desire.(*walkDesire); ok && desire != walk {
				t.Error("losing the observer restarted the route")
			}
		}
		// Entering aggro range must still interrupt the patrol and start combat.
		s.updatePosition(p, rotan.x+1, rotan.y, rotan.z, 0)
		s.visMu.Unlock()
		time.Sleep(5 * time.Second)
		synctest.Wait()
		s.visMu.Lock()
		if rotan.ai.state != aiAttacking || rotan.ai.has(&walkDesire{}) {
			t.Errorf("patrol did not yield to combat: state %d", rotan.ai.state)
		}
		s.visMu.Unlock()
	})
}

// The two ends of Rotan's corridor must reverse the patrol. Joining the ends
// directly would send him across the dungeon's walls.
func TestFireTempleRotanRouteRetracesCorridor(t *testing.T) {
	route := staticDataOrSkip(t).Walkers[13]
	if len(route) < 4 || len(route)%2 != 0 {
		t.Fatal("Rotan has no symmetric out-and-back route")
	}
	near, far := -1, -1
	for i, step := range route {
		if distance3D(step.X, step.Y, step.Z, 285.26807, 231.48059, 119.11023) < 0.001 {
			near = i
		}
		if distance3D(step.X, step.Y, step.Z, 221.43166, 277.81894, 125.087) < 0.001 {
			far = i
		}
		next := route[(i+1)%len(route)]
		if distance3D(step.X, step.Y, step.Z, next.X, next.Y, next.Z) > 6.5 {
			t.Errorf("Rotan's leg %d skips floor samples or cuts a corridor corner", i+1)
		}
	}
	if near < 0 || far != (near+len(route)/2)%len(route) {
		t.Fatal("Rotan's corridor endpoints do not turn back along the same path")
	}
	for offset := 1; offset < len(route)/2; offset++ {
		out := route[(near+offset)%len(route)]
		back := route[(near-offset+len(route))%len(route)]
		if distance3D(out.X, out.Y, out.Z, back.X, back.Y, back.Z) > 0.001 {
			t.Fatalf("Rotan does not retrace the corridor at offset %d", offset)
		}
	}
}
