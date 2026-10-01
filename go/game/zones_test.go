package game

import (
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// inside is a point in the zone, found on a grid over its bounds.
func inside(t *testing.T, z *data.Zone) (x, y, height float32) {
	t.Helper()
	minX, maxX, minY, maxY := z.X[0], z.X[0], z.Y[0], z.Y[0]
	for i := range z.X {
		minX, maxX, minY, maxY = min(minX, z.X[i]), max(maxX, z.X[i]), min(minY, z.Y[i]), max(maxY, z.Y[i])
	}
	height = (z.Top + z.Bottom) / 2
	for i := range 50 {
		for j := range 50 {
			x, y = minX+(maxX-minX)*float32(i)/49, minY+(maxY-minY)*float32(j)/49
			if z.Contains(x, y, height) {
				return x, y, height
			}
		}
	}
	t.Fatalf("no point in %s", z.Name)
	return
}

// TestZonesForbidFlying puts a player in a zone that doesn't allow flying, and in one that does.
func TestZonesForbidFlying(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	fly := func() {
		p.conn.emotion(packet(cmEmotion, func(w *wire.Writer) { w.C(emoteFly) }))
	}
	var no, yes *data.Zone
	for _, list := range d.Zones {
		for _, z := range list {
			if z.Fly && yes == nil {
				yes = z
			} else if !z.Fly && no == nil {
				no = z
			}
		}
	}
	if no == nil || yes == nil {
		t.Skip("no zone of each kind")
	}
	s.visMu.Lock()
	p.WorldID = no.MapID
	p.X, p.Y, p.Z = inside(t, no)
	s.refreshZone(p)
	s.visMu.Unlock()
	fly()
	s.visMu.Lock()
	if p.zone == nil || p.zone.Fly || p.inState(stateFlying) || tap.count(smSystemMessage) != 1 {
		t.Errorf("zone %v, flying %v, messages %d", p.zone.Name, p.inState(stateFlying), tap.count(smSystemMessage))
	}
	p.WorldID = yes.MapID
	p.X, p.Y, p.Z = inside(t, yes)
	s.refreshZone(p)
	flying := p.zone.Fly
	s.visMu.Unlock()
	if flying {
		fly()
		if !p.inState(stateFlying) {
			t.Error("didn't fly where it may")
		}
	}
}

// TestZoneFollowsMoves has the player leave its zone for a linked one, and drown below the water level.
func TestZoneFollowsMoves(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	var from *data.Zone
	for _, z := range d.Zones[p.WorldID] {
		if len(z.Neighbors) > 0 && !z.Neighbors[0].Breath {
			from = z
			break
		}
	}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	p.spawned = true
	p.zone = from
	to := from.Neighbors[0]
	p.X, p.Y, p.Z = inside(t, to)
	s.data.WorldMaps[p.WorldID].WaterLevel = -1e6
	s.updateZone(p)
	if p.zone != to {
		t.Errorf("in %v, not %s", p.zone, to.Name)
	}
	// Under the water in a zone one can't breathe in, life goes.
	s.data.WorldMaps[p.WorldID].WaterLevel = 1e6
	p.zone = &data.Zone{}
	hp := p.life.HP
	s.checkWaterLevel(p)
	s.visMu.Unlock()
	time.Sleep(100 * time.Millisecond)
	s.visMu.Lock()
	if p.drowning == nil || p.life.HP >= hp {
		t.Errorf("hp %d, was %d, task %v", p.life.HP, hp, p.drowning)
	}
	p.drowning.cancel()
	// Below the death level it dies.
	s.data.WorldMaps[p.WorldID].DeathLevel = 1e6
	s.checkWaterLevel(p)
	if !p.dead {
		t.Error("alive below the death level")
	}
}

// TestWeatherChanges has an expired weather redrawn and its players told, and the clock told to all.
func TestWeatherChanges(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p, tap := fighter(t, s, 1000)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.spawned[p.ID] = p
	s.weathers = map[int32]weatherState{p.WorldID: {code: 3, since: time.Now().Add(-3 * time.Hour)}}
	s.checkWeathers()
	if tap.count(smWeather) != 1 || time.Since(s.weathers[p.WorldID].since) > time.Minute {
		t.Errorf("%d weather packets, %+v", tap.count(smWeather), s.weathers[p.WorldID])
	}
	s.checkWeathers()
	s.setWeather(p.WorldID+1, 2)
	if tap.count(smWeather) != 1 {
		t.Errorf("%d weather packets after a fresh one and another map's", tap.count(smWeather))
	}
}
