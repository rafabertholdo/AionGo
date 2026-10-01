package game

import (
	"fmt"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestInsectProblemBranches(t *testing.T) {
	for _, branch := range []struct {
		choice uint16
		first  int32
		kills  []int32
		exp    int64
		kinah  int64
		items  int64
	}{
		{10000, 1, []int32{210734}, 1950, 200, 3},
		{10001, 11, []int32{210380, 210381}, 2010, 100, 1},
	} {
		t.Run(fmt.Sprint(branch.choice), func(t *testing.T) {
			d := staticDataOrSkip(t)
			script := &data.QuestScript{ID: 2114, Kind: data.QuestCustom, StartNPC: 203533, EndNPC: 203533}
			s := testServer(d)
			p := wrathchild(s)
			p.Race = "ASMODIANS"
			p.level = 4
			p.Exp = d.ExpStart(4)
			p.cube = nil
			p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
			p.quests = []store.Quest{{ID: 2110, Status: "COMPLETE", CompleteCount: 1}}
			p.seen = map[int32]*object{}
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			s.spawned[p.ID] = p
			npc := questCatalogNPC(s, p, 203533, 0x30001)
			c.insectProblemDialog(npc, script, 25)
			if got := packets.last(smDialogWindow); got == nil {
				t.Fatal("missing quest offer")
			}
			c.insectProblemDialog(npc, script, branch.choice)
			q := p.quest(2114)
			if q == nil || q.Status != "START" || q.Vars != branch.first {
				t.Fatalf("start = %+v", q)
			}
			for index := 0; index < 9; index++ {
				if !c.insectProblemKill(branch.kills[index%len(branch.kills)]) {
					t.Fatalf("kill %d rejected", index)
				}
			}
			if q.Status != "START" || q.Vars != branch.first+9 {
				t.Fatalf("nine kills = %+v", q)
			}
			if c.insectProblemKill(210999) {
				t.Fatal("unregistered kill accepted")
			}
			if !c.insectProblemKill(branch.kills[0]) || q.Status != "REWARD" {
				t.Fatalf("reward transition = %+v", q)
			}
			if c.insectProblemKill(branch.kills[0]) {
				t.Fatal("repeat kill accepted")
			}
			if !c.insectProblemShowDialog(npc, script) {
				t.Fatal("reward preview missing")
			}
			beforeExp := p.Exp
			c.insectProblemDialog(npc, script, 17)
			if q.Status != "COMPLETE" || p.Exp-beforeExp != branch.exp || p.kinah.Count != branch.kinah || s.countItems(p, 160003504) != branch.items {
				t.Fatalf("completion = %+v, exp=%d, kinah=%d, item=%d", q, p.Exp-beforeExp, p.kinah.Count, s.countItems(p, 160003504))
			}
			c.insectProblemDialog(npc, script, 17)
			if q.CompleteCount != 1 {
				t.Fatal("repeat finish granted reward")
			}
		})
	}
}
