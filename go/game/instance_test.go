package game

import (
	"testing"
)

// TestInstance has a player go through a portal into a dungeon: it gets an instance of the map with its npcs, sees
// nobody else's, and is taken out when the instance is destroyed.
func TestInstance(t *testing.T) {
	d := staticDataOrSkip(t)
	var portalNPC int32
	for _, portal := range d.PortalList {
		if portal.Instance && !portal.Group && len(portal.Entry) > 0 && len(d.Spawns[portal.Exit.MapID]) > 0 {
			portalNPC = portal.NPC
			break
		}
	}
	if portalNPC == 0 {
		t.Skip("no simple instance portal in the data")
	}
	portal := d.Portals[portalNPC]
	s := testServer(d)
	p, tap := fighter(t, s, 1000)
	s.spawn(p)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	p.level = max(int(portal.MinLevel), 1)
	if portal.Race != "" {
		p.Race = portal.Race
	}
	s.portalUse(p, portal)
	world := portal.Exit.MapID
	in := s.registeredInstance(world, p.ID)
	if in == nil || p.WorldID != world || p.instance != in.id || tap.count(smChannelInfo) != 1 {
		t.Fatalf("the player isn't in an instance: world %d, instance %d, %v", p.WorldID, p.instance, tap.counts)
	}
	spawned := 0
	for _, o := range s.byID {
		if o.worldID == world && o.instance == in.id {
			spawned++
		}
	}
	if spawned == 0 {
		t.Errorf("the instance has no npcs")
	}
	s.spawnLocked(p) // the client has loaded the map
	s.destroyInstance(in)
	if p.WorldID == world || len(s.instances) != 0 {
		t.Errorf("the player is still in map %d, %d instances left", p.WorldID, len(s.instances))
	}
	for _, o := range s.byID {
		if o.worldID == world && o.instance == in.id {
			t.Fatalf("an npc of the instance is left")
		}
	}
}
