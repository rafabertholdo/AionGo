package game

import (
	"aionlightning/game/store"
	"testing"
)

func TestDarkPoetaVineSkillAndDuplicateHarvest(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	p.WorldID, p.instance, p.spawned = darkPoetaWorld, 1, true
	s.visMu.Lock()
	defer s.visMu.Unlock()
	o := &object{id: 0x40000, worldID: darkPoetaWorld, instance: 1, gatherable: d.Gatherables[401111], x: 1002, y: 1000, z: 130, interval: 21600}
	s.spawned[p.ID] = p
	s.addPlayerCell(p)
	s.byID[o.id] = o
	s.addObject(o)
	if p.seen[o.id] != o {
		t.Fatal("vine fixture is not visible")
	}
	p.skills = append(p.skills, store.Skill{ID: o.gatherable.HarvestSkill, Level: 299})
	s.startGathering(p, o)
	if p.interaction != nil || o.gathering != nil {
		t.Fatal("skill 299 harvested Huge Vine")
	}
	p.skills[len(p.skills)-1].Level = 300
	s.startGathering(p, o)
	if p.interaction == nil {
		t.Fatal("skill 300 cannot harvest Huge Vine")
	}
	in := p.interaction
	s.startGathering(p, o)
	if p.interaction != in {
		t.Fatal("duplicate harvest replaced pending use")
	}
	in.onSuccess(in)
	in.stop()
	o.respawn.cancel()
	if o.gatherCount != 1 || p.seen[o.id] != nil {
		t.Fatal("single harvest did not remove vine")
	}
	s.startGathering(p, o)
	if p.interaction != nil {
		t.Fatal("stale depleted vine accepted another harvest")
	}
}
