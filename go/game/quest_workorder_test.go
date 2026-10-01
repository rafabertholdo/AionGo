package game

import (
	"fmt"
	"slices"
	"sort"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

func TestAllWorkOrdersStartAndComplete(t *testing.T) {
	d := staticDataOrSkip(t)
	var ids []int32
	for id, script := range d.QuestScripts {
		if script.Kind == data.QuestWorkOrder {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) != 492 {
		t.Fatalf("work order count = %d", len(ids))
	}
	for _, id := range ids {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			script := d.QuestScripts[id]
			template := d.Quests[id]
			recipe := d.Recipes[script.RecipeID]
			if template == nil || recipe == nil || len(script.Components) == 0 || len(template.CollectItems) == 0 {
				t.Fatalf("incomplete work order data: script=%+v quest=%+v recipe=%+v", script, template, recipe)
			}
			s := testServer(d)
			p := wrathchild(s)
			p.Race = template.Race
			p.level = max(1, template.MinLevel)
			p.Exp = d.ExpStart(p.level)
			p.cube = nil
			p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
			p.seen = map[int32]*object{}
			p.skills = append(p.skills, store.Skill{ID: template.CombineSkill, Level: template.CombineSkillPoint})
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			s.spawned[p.ID] = p
			npc := questCatalogNPC(s, p, script.StartNPC, 0x30001)
			if !s.canStartQuest(p, script) {
				t.Fatal("eligible work order was not offered")
			}
			selectQuestDialog(c, npc, 1002, id)
			if q := p.quest(id); q == nil || q.Status != "START" || !slices.Contains(p.recipes, recipe.ID) {
				t.Fatalf("work order did not start: quest=%+v recipes=%v", q, p.recipes)
			}
			for _, component := range script.Components {
				if got := s.countItems(p, component.ID); got != component.Count {
					t.Fatalf("component %d: %d, want %d", component.ID, got, component.Count)
				}
			}
			selectQuestDialog(c, npc, 17, id)
			if p.quest(id).Status != "START" {
				t.Fatal("turned in without crafted goods")
			}
			for _, item := range template.CollectItems {
				if !s.addItem(p, item.ID, item.Count) {
					t.Fatalf("could not add crafted item %d", item.ID)
				}
			}
			selectQuestDialog(c, npc, 17, id)
			if q := p.quest(id); q.Status != "COMPLETE" || q.CompleteCount != 1 {
				t.Fatalf("work order did not complete: %+v", q)
			}
			if slices.Contains(p.recipes, recipe.ID) || packets.last(smRecipeDelete) == nil {
				t.Fatal("temporary recipe was not removed")
			}
			for _, item := range append(slices.Clone(script.Components), template.CollectItems...) {
				if count := s.countItems(p, item.ID); count != 0 {
					t.Fatalf("quest item %d was not cleaned: %d", item.ID, count)
				}
			}
		})
	}
}

func TestWorkOrderSkillBoundsAndAbandonment(t *testing.T) {
	d := staticDataOrSkip(t)
	script := d.QuestScripts[5000]
	template := d.Quests[5000]
	s := testServer(d)
	p := wrathchild(s)
	p.level = template.MinLevel
	p.cube = nil
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	npc := questCatalogNPC(s, p, script.StartNPC, 0x30001)
	if s.canStartQuest(p, script) {
		t.Fatal("work order offered without crafting skill")
	}
	p.skills = append(p.skills, store.Skill{ID: template.CombineSkill, Level: template.CombineSkillPoint + 41})
	if s.canStartQuest(p, script) {
		t.Fatal("work order offered after skill range")
	}
	p.skills[len(p.skills)-1].Level = template.CombineSkillPoint
	selectQuestDialog(c, npc, 1002, script.ID)
	if p.quest(script.ID) == nil || p.quest(script.ID).Status != "START" {
		t.Fatal("work order did not start at the required skill level")
	}
	w := wire.Packet(cmDeleteQuest)
	w.H(uint16(script.ID))
	c.deleteQuest(wire.NewReader(w.Data[1:]))
	if p.quest(script.ID).Status != "NONE" || slices.Contains(p.recipes, script.RecipeID) || packets.last(smRecipeDelete) == nil {
		t.Fatal("abandonment did not clear the quest and recipe")
	}
	for _, component := range script.Components {
		if count := s.countItems(p, component.ID); count != 0 {
			t.Fatalf("abandoned component %d remained: %d", component.ID, count)
		}
	}
}
