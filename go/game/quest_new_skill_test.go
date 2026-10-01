package game

import (
	"fmt"
	"testing"
)

func TestNewSkillClassBranchAndReward(t *testing.T) {
	for _, branch := range []struct {
		id   int32
		race string
	}{
		{1205, "ELYOS"},
		{2132, "ASMODIANS"},
	} {
		for index, class := range newSkillClasses {
			t.Run(fmt.Sprintf("%d/%s", branch.id, class), func(t *testing.T) {
				d := staticDataOrSkip(t)
				s := testServer(d)
				p := wrathchild(s)
				p.Race, p.Class, p.level, p.Exp = branch.race, class, 3, d.ExpStart(3)
				p.cube = nil
				p.seen = map[int32]*object{}
				packets := &questPackets{}
				c := &conn{s: s, player: p, tap: packets.tap}
				p.conn = c
				s.spawned[p.ID] = p
				if !c.newSkillLevelUp(branch.id) || c.newSkillLevelUp(branch.id) {
					t.Fatal("level-up quest was missing or repeated")
				}
				q := p.quest(branch.id)
				if q == nil || q.Status != "REWARD" || q.Vars != int32(index+1) {
					t.Fatalf("quest state = %+v", q)
				}
				script := d.QuestScripts[branch.id]
				npcs := newSkillNPCs(branch.id)
				wrong := questCatalogNPC(s, p, npcs[(index+1)%4], 0x30001)
				if c.newSkillDialog(wrong, script, -1) || c.newSkillDialog(wrong, script, 17) {
					t.Fatal("wrong class trainer accepted quest")
				}
				trainer := questCatalogNPC(s, p, npcs[index], 0x30002)
				if !c.newSkillDialog(trainer, script, -1) || !c.newSkillDialog(trainer, script, 1009) {
					t.Fatal("class trainer dialog missing")
				}
				before := p.Exp
				if !c.newSkillDialog(trainer, script, 17) || p.Exp-before != 250 || q.Status != "COMPLETE" || q.CompleteCount != 1 {
					t.Fatalf("quest reward = %+v, exp=%d", q, p.Exp-before)
				}
				c.newSkillDialog(trainer, script, 17)
				if p.Exp-before != 250 || q.CompleteCount != 1 {
					t.Fatal("repeat trainer reward")
				}
			})
		}
	}
}

func TestNewSkillRequiresLevelAndRace(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "WARRIOR", 2
	c := &conn{s: s, player: p}
	p.conn = c
	if c.newSkillLevelUp(1205) || p.quest(1205) != nil {
		t.Fatal("started before level three")
	}
	p.level = 3
	if c.newSkillLevelUp(2132) || p.quest(2132) != nil {
		t.Fatal("started wrong-race quest")
	}
}
