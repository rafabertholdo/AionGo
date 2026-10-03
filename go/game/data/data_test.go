package data

import (
	"os"
	"testing"
)

// The real static data, when the tests run with it mounted.
var staticData = os.Getenv("AION_DATA")

func TestLoadsTheStaticData(t *testing.T) {
	if staticData == "" {
		t.Skip("AION_DATA is not set")
	}
	if _, err := os.Stat(staticData); err != nil {
		t.Skipf("static data not available at %s: %v", staticData, err)
	}
	d, err := Load(staticData)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Items) < 40000 {
		t.Fatalf("%d item templates", len(d.Items))
	}
	if sword := d.Items[100000001]; sword == nil || sword.Slot != 3 || sword.EquipmentType != "WEAPON" {
		t.Fatalf("sword: %+v", sword)
	}
	if d.Level(0) != 1 || d.Level(650) != 2 || d.Level(649) != 1 {
		t.Fatalf("levels: %d %d %d", d.Level(0), d.Level(650), d.Level(649))
	}
	if d.Initial.Elyos.MapID != 210010000 || len(d.Initial.Classes) != 4 {
		t.Fatalf("initial data: %+v", d.Initial.Elyos)
	}
	if skills := d.SkillsAt("WARRIOR", "ELYOS", 1); len(skills) == 0 {
		t.Fatal("no starting skills for a warrior")
	}
	book := d.Items[100600034]
	if book == nil || book.NameID != 1401641 || book.Mask != 562 || len(book.Modifiers) != 8 {
		t.Fatalf("book: %+v", book)
	}
	if m := book.Modifiers[5]; m.Kind != ModMean || m.Stat != Power || m.Min != 20 || m.Max != 23 {
		t.Fatalf("book's mean modifier: %+v", m)
	}
	if mage := d.PlayerStatsFor("MAGE", 1); mage.MaxHP != 132 || mage.MaxMP != 405 || mage.Knowledge != 115 {
		t.Fatalf("mage at 1: %+v", mage)
	}
	// Past the stats_templates: AL-Game's calculated template, with level 10's HP.
	if mage := d.PlayerStatsFor("MAGE", 50); mage.MaxHP != 458 || mage.MaxMP != 1000 || mage.AttackSpeed != 1.5 {
		t.Fatalf("mage at 50: %+v", mage)
	}
	if len(d.Titles) == 0 || d.Titles[1].Modifiers[0].Stat != MaxHP || !d.Titles[1].Modifiers[0].Bonus {
		t.Fatalf("title 1: %+v", d.Titles[1])
	}
	if d.BindPoints[1].MapID != 110010000 || d.WorldMaps[210010000].TwinCount != 2 {
		t.Fatalf("bind point 1: %+v, Poeta: %+v", d.BindPoints[1], d.WorldMaps[210010000])
	}
	robe := d.Skills[67]
	if robe == nil || !robe.Passive() || len(robe.Effects) != 1 || robe.Effects[0].Kind != "armormastery" ||
		robe.Effects[0].Armor != "ROBE" || robe.Effects[0].Changes[0].Stat != PhysicalDefense {
		t.Fatalf("robe mastery: %+v", robe)
	}
	if len(d.Sieges) != 54 || d.Sieges[0].ID != 1011 || d.Sieges[0].Type != "FORTRESS" {
		t.Fatalf("%d siege locations, first %+v", len(d.Sieges), d.Sieges[0])
	}
	if set := d.ItemSets[110100919]; set == nil || set.ID != 1 || len(set.PartBonuses) != 2 {
		t.Fatalf("item set: %+v", set)
	}
	for _, questID := range []int32{1000, 2000} {
		quest := d.Quests[questID]
		if quest == nil || quest.MinLevel != 1 || !quest.CannotGiveUp || len(quest.Rewards) != 1 || quest.Rewards[0].Experience != 1 {
			t.Fatalf("prologue %d: %+v", questID, quest)
		}
	}
	if quest := d.Quests[1103]; quest == nil || len(quest.FinishedQuestConditions) != 1 || quest.FinishedQuestConditions[0] != 1102 ||
		len(quest.CollectItems) != 1 || quest.CollectItems[0].ID != 182200201 || quest.CollectItems[0].Count != 4 ||
		len(quest.QuestDrops) != 1 || quest.QuestDrops[0].NPCID != 700105 || quest.QuestDrops[0].Chance != 100 {
		t.Fatalf("Grain Thieves data: %+v", quest)
	}
	if quest := d.Quests[2102]; quest == nil || len(quest.Rewards) != 1 || len(quest.Rewards[0].Items) != 1 ||
		quest.Rewards[0].Items[0].ID != 169300002 || quest.Rewards[0].Items[0].Count != 10 {
		t.Fatalf("A Bloody Task rewards: %+v", quest)
	}
}

func TestLoadsEntireQuestScriptCatalog(t *testing.T) {
	if staticData == "" {
		t.Skip("AION_DATA is not set")
	}
	d, err := Load(staticData)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{QuestReportTo: 170, QuestMonsterHunt: 316, QuestItemCollecting: 613, QuestWorkOrder: 492, QuestXML: 5}
	counts := map[string]int{}
	for _, script := range d.QuestScripts {
		counts[script.Kind]++
		if d.Quests[script.ID] == nil {
			t.Errorf("script %d has no quest template", script.ID)
		}
	}
	for kind, count := range want {
		if counts[kind] != count {
			t.Errorf("%s: loaded %d scripts, want %d", kind, counts[kind], count)
		}
	}
	for _, script := range d.QuestScripts {
		if !supportedQuestScript(script.Kind) {
			continue
		}
		wantStart := 1
		if script.Kind == QuestCustom && !script.NPCStart {
			wantStart = 0 // Java starts these by item, zone, level, or quest unlock.
		}
		if countScript(d.QuestStarts[script.StartNPC], script.ID) != wantStart || countScript(d.QuestEnds[script.EndNPC], script.ID) != 1 {
			t.Errorf("script %d has incorrect start or end NPC indexes", script.ID)
		}
		for _, drop := range d.Quests[script.ID].QuestDrops {
			if countScript(d.QuestDropsByNPC[drop.NPCID], script.ID) != 1 {
				t.Errorf("script %d has an absent or duplicate drop index for NPC %d", script.ID, drop.NPCID)
			}
		}
	}
	for _, tc := range []struct {
		id, start, end, action int32
		kind                   string
	}{
		{1101, 203049, 203057, 0, QuestReportTo},
		{1102, 203057, 203057, 0, QuestMonsterHunt},
		{1103, 203057, 203057, 700105, QuestItemCollecting},
		{2104, 203502, 203502, 700124, QuestItemCollecting},
	} {
		script := d.QuestScripts[tc.id]
		if script == nil || script.StartNPC != tc.start || script.EndNPC != tc.end || script.ActionNPC != tc.action || script.Kind != tc.kind {
			t.Errorf("script %d = %+v", tc.id, script)
		}
	}
	if hunt := d.QuestScripts[1112]; hunt == nil || len(hunt.MonsterInfos) != 5 || hunt.MonsterInfos[3].VarID != 1 {
		t.Errorf("multi-counter monster hunt: %+v", hunt)
	}
	if hunt := d.QuestScripts[2434]; hunt == nil || len(hunt.MonsterInfos) != 4 || hunt.MonsterInfos[0].VarID != 3 {
		t.Errorf("Java NPC-keyed monster list: %+v", hunt)
	}
	if q := d.Quests[1572]; q == nil || len(q.ClassPermitted) != 1 || q.ClassPermitted[0] != "SPIRIT_MASTER" {
		t.Errorf("class restriction: %+v", q)
	}
	if q := d.Quests[1971]; q == nil || len(q.Rewards) != 1 || len(q.Rewards[0].SelectableItems) != 4 || q.Rewards[0].SelectableItems[0].ID != 169200002 {
		t.Errorf("selectable rewards: %+v", q)
	}
	if q := d.Quests[4902]; q == nil || len(q.QuestWorkItems) != 1 || q.QuestWorkItems[0].ID != 182207068 {
		t.Errorf("quest work item: %+v", q)
	}
}

func countScript(scripts []*QuestScript, id int32) int {
	count := 0
	for _, script := range scripts {
		if script.ID == id {
			count++
		}
	}
	return count
}

func TestModifiers(t *testing.T) {
	for _, c := range []struct {
		m                  Modifier
		base, current, out int32
	}{
		{Modifier{Kind: ModSet, Stat: AttackRange, Value: 15000}, 1500, 1500, 15000},
		{Modifier{Kind: ModAdd, Stat: MaxMP, Value: 47}, 405, 405, 452},
		{Modifier{Kind: ModAdd, Stat: MaxHP, Value: 20, Bonus: true}, 132, 132, 20},
		{Modifier{Kind: ModAdd, Stat: MaxHP, Value: -200, Bonus: true}, 132, 132, -132},
		{Modifier{Kind: ModMean, Stat: MainHandPower, Min: 20, Max: 23}, 0, 0, 22},
		{Modifier{Kind: ModRate, Stat: MaxHP, Value: 10, Bonus: true}, 135, 135, 14},
		{Modifier{Kind: ModRate, Stat: MaxHP, Value: 10}, 135, 135, 149},
		{Modifier{Kind: ModSub, Stat: MaxHP, Value: 5}, 100, 100, 95},
		// Speed stays within 600…12000 of the current value.
		{Modifier{Kind: ModAdd, Stat: Speed, Value: 20000, Bonus: true}, 6000, 6000, 6000},
	} {
		if got := c.m.Apply(c.base, c.current); got != c.out {
			t.Errorf("%+v on %d/%d = %d, want %d", c.m, c.base, c.current, got, c.out)
		}
	}
}

func TestFirstSlot(t *testing.T) {
	if FirstSlot(3) != 1 || FirstSlot(192) != 64 || FirstSlot(8) != 8 {
		t.Fatal(FirstSlot(3), FirstSlot(192), FirstSlot(8))
	}
}

// TestZones checks that the zones load, link and hold points, and that flying is a zone's business.
func TestZones(t *testing.T) {
	if staticData == "" {
		t.Skip("AION_DATA is not set")
	}
	d, err := Load(staticData)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Zones[210010000]) == 0 || d.WorldMaps[110010000].DeathLevel != 400 || d.WorldMaps[210010000].WaterLevel != 100 {
		t.Fatalf("zones of Poeta %d, maps %+v", len(d.Zones[210010000]), d.WorldMaps[110010000])
	}
	z := d.Zones[210010000][0]
	if len(z.X) < 3 || len(z.Neighbors) == 0 {
		t.Fatalf("zone %+v", z)
	}
	square := &Zone{X: []float32{0, 10, 10, 0}, Y: []float32{0, 0, 10, 10}, Top: 50, Bottom: 5}
	if !square.Contains(5, 5, 20) || square.Contains(15, 5, 20) || square.Contains(5, 5, 60) || square.Contains(5, 5, 1) {
		t.Error("point in polygon is wrong")
	}
}
