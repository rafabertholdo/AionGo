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
