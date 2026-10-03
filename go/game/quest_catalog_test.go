package game

import (
	"fmt"
	"sort"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// Exercise every data-driven Java template, so expansion of the quest catalog
// does not depend on manually visiting each NPC with the client.
func TestAllGenericQuestScriptsComplete(t *testing.T) {
	d := staticDataOrSkip(t)
	ids := make([]int32, 0, len(d.QuestScripts))
	for id, script := range d.QuestScripts {
		if script.Kind == data.QuestReportTo || script.Kind == data.QuestMonsterHunt || script.Kind == data.QuestItemCollecting {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) != 1099 {
		t.Fatalf("generic script count = %d", len(ids))
	}
	for _, id := range ids {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			script := d.QuestScripts[id]
			template := d.Quests[id]
			s := testServer(d)
			p := wrathchild(s)
			p.Race = template.Race
			if p.Race == "" {
				p.Race = "ELYOS"
			}
			p.Class = "GLADIATOR"
			if len(template.ClassPermitted) > 0 {
				p.Class = template.ClassPermitted[len(template.ClassPermitted)-1]
			}
			p.level = max(1, template.MinLevel)
			p.Exp = d.ExpStart(p.level)
			p.stats = s.playerStats(p)
			p.seen = map[int32]*object{}
			p.cube = nil
			p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
			for _, prerequisite := range template.FinishedQuestConditions {
				p.quests = append(p.quests, store.Quest{ID: prerequisite, Status: "COMPLETE", CompleteCount: 1})
			}
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			s.spawned[p.ID] = p
			start := questCatalogNPC(s, p, script.StartNPC, 0x30001)
			end := start
			if script.EndNPC != script.StartNPC {
				end = questCatalogNPC(s, p, script.EndNPC, 0x30002)
			}
			selectQuestDialog(c, start, 1002, id)
			if q := p.quest(id); q == nil || q.Status != "START" {
				t.Fatalf("quest did not start: %+v", q)
			}
			switch script.Kind {
			case data.QuestMonsterHunt:
				for _, monster := range script.MonsterInfos {
					for i := int32(0); i < monster.MaxKill; i++ {
						s.recordQuestKill(&object{npc: &data.NpcTemplate{ID: monster.NPCID}}, p)
					}
				}
				if !monsterHuntComplete(p.quest(id), script) {
					t.Fatalf("kills did not complete: %+v", p.quest(id))
				}
			case data.QuestItemCollecting:
				for _, item := range template.CollectItems {
					p.cube = append(p.cube, &store.Item{UniqueID: item.ID, ItemID: item.ID, Count: item.Count, Owner: p.ID})
				}
			}
			if script.Kind == data.QuestItemCollecting {
				selectQuestDialog(c, end, 33, id)
			} else {
				selectQuestDialog(c, end, 1009, id)
			}
			if q := p.quest(id); q.Status != "REWARD" {
				t.Fatalf("quest did not reach reward: %+v", q)
			}
			choice := uint16(17)
			if len(template.Rewards[0].SelectableItems) > 0 || len(template.ClassRewards(p.Class)) > 0 {
				choice = 8
			}
			selectQuestDialog(c, end, choice, id)
			if q := p.quest(id); q.Status != "COMPLETE" {
				t.Fatalf("quest did not complete: %+v", q)
			}
		})
	}
}

func questCatalogNPC(s *Server, p *player, npcID, objectID int32) *object {
	o := &object{id: objectID, worldID: p.WorldID, x: p.X + 2, y: p.Y, z: p.Z,
		npc: &data.NpcTemplate{ID: npcID}, stats: &gameStats{}, watchers: map[int32]*player{p.ID: p}}
	p.seen[objectID] = o
	s.byID[objectID] = o
	return o
}
