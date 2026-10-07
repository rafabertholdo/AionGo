package game

import (
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// The non-dialog events of the translated Java handlers, through the server's own event paths (TestQuestJavaParity
// covers their dialogs).
func TestJavaQuestEventDispatch(t *testing.T) {
	t.Run("zone entry starts Orders from Nerita", func(t *testing.T) {
		d, s, p, _, _, _ := customQuestPortFixture(t, 2500, nil)
		p.Race = "ASMODIANS"
		zone := &data.Zone{Name: "BELUSLAN_FORTRESS_220040000", MapID: 220040000}
		p.WorldID = zone.MapID
		s.enterQuestZone(p, nil, zone)
		if q := p.quest(2500); q == nil || q.Status != "START" {
			t.Fatalf("zone entry did not start the quest: %+v (template %+v)", q, d.Quests[2500] != nil)
		}
	})
	t.Run("kill advances Securing the Supply Route", func(t *testing.T) {
		_, s, p, _, _, _ := customQuestPortFixture(t, 2019, []store.Quest{{ID: 2019, Status: "START", Vars: 1}})
		s.recordQuestKill(&object{npc: &data.NpcTemplate{ID: 210492}}, p)
		if questVar(p.quest(2019).Vars, 0) != 2 {
			t.Fatalf("kill did not advance the quest: %+v", p.quest(2019))
		}
		s.recordQuestKill(&object{npc: &data.NpcTemplate{ID: 1}}, p)
		if questVar(p.quest(2019).Vars, 0) != 2 {
			t.Fatalf("an unrelated kill advanced the quest: %+v", p.quest(2019))
		}
	})
	t.Run("level-up unlocks a campaign quest", func(t *testing.T) {
		_, _, p, c, _, _ := customQuestPortFixture(t, 2019, []store.Quest{{ID: 2019, Status: "LOCKED"}})
		c.questLevelUp()
		if q := p.quest(2019); q.Status != "START" {
			t.Fatalf("level-up did not unlock the quest: %+v", q)
		}
	})
	t.Run("item use outside its zone does nothing", func(t *testing.T) {
		_, _, p, c, _, packets := customQuestPortFixture(t, 2031, []store.Quest{{ID: 2031, Status: "START", Vars: 2}})
		item := &store.Item{UniqueID: 0x70001, ItemID: 182204001, Count: 1}
		p.cube = append(p.cube, item)
		if c.javaItemUse(item) || p.quest(2031).Status != "START" || packets.last(smItemUsageAnimation) != nil {
			t.Fatalf("item use outside the Hill of Belemu acted: %+v", p.quest(2031))
		}
	})
}

func TestFireTempleHardKromedeGrantsHannetsVengeanceCredit(t *testing.T) {
	_, s, p, c, _, _ := customQuestPortFixture(t, 1470, []store.Quest{{ID: 1470, Status: "START"}})
	o := &object{worldID: 320100000, npc: s.data.Npcs[214621]}
	if !c.javaKill(o) || p.quest(1470).Status != "REWARD" {
		t.Fatal("hard Kromede did not advance Hannet's Vengeance")
	}
	if o.npc.ID != 214621 {
		t.Fatal("quest credit changed the NPC identity used for loot and combat")
	}
	before := p.quest(1470).Vars
	if c.javaKill(o) || p.quest(1470).Vars != before {
		t.Fatal("repeated credit advanced an already rewarded quest")
	}
}

func TestFireTempleHardKromedeCreditRequiresDungeonAndActiveQuest(t *testing.T) {
	for _, name := range []string{"other map", "inactive quest"} {
		t.Run(name, func(t *testing.T) {
			_, s, p, c, _, _ := customQuestPortFixture(t, 1470, nil)
			world := int32(320100000)
			if name == "other map" {
				p.quests = append(p.quests, store.Quest{ID: 1470, Status: "START"})
				world = 210020000
			}
			if c.javaKill(&object{worldID: world, npc: s.data.Npcs[214621]}) {
				t.Fatal("invalid Kromede kill granted credit")
			}
		})
	}
}
