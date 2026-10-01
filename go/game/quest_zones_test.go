package game

import (
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestQuestZoneEntryFromRefreshAndMovement(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 7
	p.Exp = d.ExpStart(7)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10579, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	movieCount := func() int {
		count := 0
		for _, frame := range packets.frames {
			if frame[0] == smPlayMovie {
				count++
			}
		}
		return count
	}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	script := &data.QuestScript{ID: 1123, Kind: data.QuestCustom, StartNPC: 790001, EndNPC: 790001}
	pernos := questCatalogNPC(s, p, 790001, 0x30003)
	c.wheresTuttyDialog(pernos, script, 1002)
	if q := p.quest(1123); q == nil || q.Status != "START" {
		t.Fatalf("quest start = %+v", q)
	}

	var target *data.Zone
	for _, zone := range d.Zones[210010000] {
		if zone.Name == "Q1123" {
			target = zone
			break
		}
	}
	if target == nil {
		t.Fatal("Q1123 zone missing")
	}
	p.WorldID = 210010000
	p.X, p.Y, p.Z = inside(t, target)
	s.refreshZone(p)
	if p.zone == nil || p.zone.Name != "KABARAH_STRIP_MINE" {
		t.Fatalf("refresh selected zone %+v", p.zone)
	}
	s.updateZone(p)
	if p.zone != target || p.quest(1123).Status != "REWARD" || movieCount() != 1 {
		t.Fatalf("movement entry: zone %v, quest %+v, movies %d", p.zone, p.quest(1123), movieCount())
	}
	s.updateZone(p)
	if movieCount() != 1 {
		t.Fatal("same zone replayed movie")
	}
}

func TestQuestZoneEntryRequiresMatchingMapAndTransition(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p := wrathchild(s)
	p.conn = &conn{s: s, player: p}
	zone := &data.Zone{Name: "Q1123", MapID: 210010000}
	p.WorldID = 220010000
	s.enterQuestZone(p, nil, zone)
	if p.quest(1123) != nil {
		t.Fatal("zone on another map started quest")
	}
	p.WorldID = 210010000
	s.enterQuestZone(p, zone, zone)
	if p.quest(1123) != nil {
		t.Fatal("unchanged zone started quest")
	}
}
