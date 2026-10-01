package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

type chainFixture struct {
	t       *testing.T
	s       *Server
	p       *player
	c       *conn
	script  *data.QuestScript
	packets *questPackets
	npcs    map[int32]*object
}

func newChainFixture(t *testing.T, id int32) *chainFixture {
	d := staticDataOrSkip(t)
	script := d.QuestScripts[id]
	template := d.Quests[id]
	if script == nil || template == nil {
		t.Fatalf("quest %d is not registered", id)
	}
	s := testServer(d)
	p := wrathchild(s)
	p.Race = template.Race
	p.Class = "GLADIATOR"
	p.level = max(template.MinLevel, 11)
	p.Exp = d.ExpStart(p.level)
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	for _, required := range template.FinishedQuestConditions {
		p.quests = append(p.quests, store.Quest{ID: required, Status: "COMPLETE", CompleteCount: 1})
	}
	return &chainFixture{t: t, s: s, p: p, c: c, script: script, packets: packets, npcs: map[int32]*object{}}
}

// talk sends a dialog select to the NPC and returns the object it went to.
func (f *chainFixture) talk(npc int32, dialog uint16) *object {
	o := f.npcs[npc]
	if o == nil {
		o = questCatalogNPC(f.s, f.p, npc, 0x32000+int32(len(f.npcs)))
		f.npcs[npc] = o
	}
	f.c.customQuestDialog(o, f.script, dialog)
	return o
}

func (f *chainFixture) expectWindow(o *object, page uint16) {
	f.t.Helper()
	if got := f.packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(o.id, page, f.script.ID).Data) {
		f.t.Fatalf("dialog window = %x, want page %d", got, page)
	}
}

func (f *chainFixture) expectVars(want int32) {
	f.t.Helper()
	if q := f.p.quest(f.script.ID); q == nil || q.Status != "START" || questVar(q.Vars, 0) != want {
		f.t.Fatalf("quest state = %+v, want START var %d", q, want)
	}
}

func (f *chainFixture) give(item int32, count int64) {
	f.t.Helper()
	if !f.s.addItem(f.p, item, count) {
		f.t.Fatalf("could not give item %d", item)
	}
}

// start offers and accepts the quest at its start NPC.
func (f *chainFixture) start(offerPage uint16) {
	f.t.Helper()
	o := f.talk(f.script.StartNPC, 25)
	f.expectWindow(o, offerPage)
	f.talk(f.script.StartNPC, 1002)
	f.expectVars(0)
}

// finish completes the reward at npc and checks experience and repeat safety.
func (f *chainFixture) finish(npc int32) {
	f.t.Helper()
	template := f.s.data.Quests[f.script.ID]
	choice := uint16(17)
	if len(template.Rewards[0].SelectableItems) > 0 {
		choice = 8
	}
	before := f.p.Exp
	f.talk(npc, choice)
	q := f.p.quest(f.script.ID)
	if q.Status != "COMPLETE" || q.CompleteCount != 1 || f.p.Exp-before != template.Rewards[0].Experience {
		f.t.Fatalf("completion = %+v exp=%d", q, f.p.Exp-before)
	}
	f.talk(npc, choice)
	if f.p.quest(f.script.ID).CompleteCount != 1 {
		f.t.Fatal("duplicate completion")
	}
}

// purify runs a Balder-style step: page without and with the item, then reward.
func (f *chainFixture) purify(npc int32, item int32, pageMissing, page uint16) {
	f.t.Helper()
	f.expectVarsBeforeReward()
	f.expectWindow(f.talk(npc, 25), pageMissing)
	f.give(item, 1)
	f.expectWindow(f.talk(npc, 25), page)
	f.expectWindow(f.talk(npc, 10255), 10002)
	f.talk(npc, 1009)
	if q := f.p.quest(f.script.ID); q.Status != "REWARD" || f.s.countItems(f.p, item) != 0 {
		f.t.Fatalf("purification = %+v holy=%d", q, f.s.countItems(f.p, item))
	}
}

func (f *chainFixture) expectVarsBeforeReward() {
	f.t.Helper()
	if q := f.p.quest(f.script.ID); q == nil || q.Status != "START" {
		f.t.Fatalf("quest not in START: %+v", q)
	}
}
