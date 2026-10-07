package game

import (
	"aionlightning/wire"
	"testing"
	"testing/synctest"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestPortalDialogRejectsInvalidRunObjects(t *testing.T) {
	for _, name := range []string{"other map", "other instance", "dead portal", "remote portal"} {
		t.Run(name, func(t *testing.T) {
			s := testServer(&data.Data{Portals: map[int32]*data.Portal{730069: {NPC: 730069}}})
			p := &player{character: &character{Character: &store.Character{ID: 1, WorldID: 210020000}}, instance: 2, spawned: true}
			o := &object{id: 10, worldID: p.WorldID, instance: p.instance, npc: &data.NpcTemplate{ID: 730069, Type: "PORTAL"}}
			switch name {
			case "other map":
				o.worldID = 220020000
			case "other instance":
				o.instance = 3
			case "dead portal":
				o.dead = true
			case "remote portal":
				o.x = 11
			}
			p.seen = map[int32]*object{o.id: o}
			c := &conn{s: s, player: p}
			if c.portalDialog(o.id) {
				t.Fatal("invalid portal request claimed or scheduled a transfer")
			}
		})
	}
}

func TestPortalTransferRevalidatesAfterDelay(t *testing.T) {
	for _, name := range []string{"moved", "dead", "disconnected", "unseen", "replaced", "other instance"} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := testServer(&data.Data{Portals: map[int32]*data.Portal{730069: {NPC: 730069}}})
				p := &player{character: &character{Character: &store.Character{ID: 1, WorldID: 210020000}}, instance: 2, spawned: true, known: map[int32]*player{}}
				o := &object{id: 10, worldID: p.WorldID, instance: p.instance, npc: &data.NpcTemplate{ID: 730069, Type: "PORTAL"}}
				p.seen = map[int32]*object{o.id: o}
				s.byID[o.id] = o
				count := 0
				c := &conn{s: s, player: p, tap: func(w *wire.Writer) {
					if w.Data[0] == smUseObject {
						count++
					}
				}}
				p.conn = c
				p.stats = &gameStats{}
				if !c.portalDialog(o.id) {
					t.Fatal("valid portal not accepted")
				}
				s.visMu.Lock()
				switch name {
				case "moved":
					p.X = 11
				case "dead":
					p.dead = true
				case "disconnected":
					p.conn = nil
				case "unseen":
					delete(p.seen, o.id)
				case "replaced":
					s.byID[o.id] = &object{id: o.id}
				case "other instance":
					p.instance = 3
				}
				s.visMu.Unlock()
				time.Sleep(3 * time.Second)
				synctest.Wait()
				if count != 1 || p.WorldID != 210020000 {
					t.Fatal("invalid delayed transfer completed")
				}
			})
		})
	}
}

func TestNochsanaExitReturnsEachRaceOutsideInstance(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, race := range []string{"ELYOS", "ASMODIANS"} {
		t.Run(race, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := testServer(d)
				p, _ := fighter(t, s, 1000)
				p.Race = race
				s.spawn(p)
				s.visMu.Lock()
				in := s.newInstance(300030000)
				defer func() { s.visMu.Lock(); s.destroyInstance(in); s.visMu.Unlock() }()
				var exit *object
				for _, o := range s.byID {
					if o.worldID == in.world && o.instance == in.id && o.npc != nil && o.npc.ID == 700438 {
						exit = o
						break
					}
				}
				if exit == nil {
					t.Fatal("missing Nochsana exit")
				}
				s.changePosition(p, in.world, in.id, exit.x, exit.y, exit.z, 0)
				s.spawnLocked(p)
				s.visMu.Unlock()
				if !p.conn.portalDialog(exit.id) {
					t.Fatal("Nochsana gate did not accept dialog")
				}
				time.Sleep(3 * time.Second)
				synctest.Wait()
				if p.WorldID != 400010000 || p.instance != 0 {
					t.Fatalf("exit map=%d instance=%d", p.WorldID, p.instance)
				}
				portal := d.InstancePortal(in.world, race)
				point := portal.EntryFor(race)
				if p.X != point.X || p.Y != point.Y || p.Z != point.Z {
					t.Fatal("exit did not choose faction return")
				}
			})
		})
	}
}
