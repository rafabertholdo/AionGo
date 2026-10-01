package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestAltenosWeddingRingObjectTurnInAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[altenosWeddingRingQuestID], d.Quests[altenosWeddingRingQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || script.StartNPC != altenosWeddingRingStartNPCID || script.EndNPC != altenosWeddingRingStartNPCID ||
		template.Race != "ELYOS" || template.MinLevel != 15 || template.NameID != 2204665 || len(template.CollectItems) != 1 || template.CollectItems[0] != (data.QuestItem{ID: altenosWeddingRingItemID, Count: 1}) || len(template.Rewards) != 2 || template.Rewards[0].Experience != 20400 {
		t.Fatalf("unexpected Alteno's Wedding Ring metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{altenosWeddingRingStartNPCID, altenosWeddingRingReportNPCID, altenosWeddingRingObjectID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC/object %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.level, p.Exp = "ELYOS", 15, d.ExpStart(15)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p

	npc := func(id, objectID int32) *object {
		o := questCatalogNPC(s, p, id, objectID)
		o.npc = d.Npcs[id]
		if o.npc == nil {
			o.npc = &data.NpcTemplate{ID: id}
		}
		s.initNpc(o)
		return o
	}
	start := npc(altenosWeddingRingStartNPCID, 0x31162)
	reporter := npc(altenosWeddingRingReportNPCID, 0x31163)
	workObject := npc(altenosWeddingRingObjectID, 0x31164)

	if !c.altenosWeddingRingDialog(start, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1011, altenosWeddingRingQuestID).Data) {
		t.Fatalf("quest offer page = %x", packets.last(smDialogWindow))
	}
	if !c.altenosWeddingRingDialog(start, script, 1002) || p.quest(altenosWeddingRingQuestID) == nil || p.quest(altenosWeddingRingQuestID).Status != "START" {
		t.Fatalf("quest did not start: %+v", p.quest(altenosWeddingRingQuestID))
	}
	if c.altenosWeddingRingDialog(reporter, script, -1) {
		t.Fatal("report NPC accepted the quest before the ring interaction")
	}

	p.targetID = workObject.id
	if !c.altenosWeddingRingDialog(workObject, script, -1) || s.countItems(p, altenosWeddingRingItemID) != 1 || workObject.useTask == nil ||
		!bytes.Equal(packets.last(smUseObject), useObject(p.ID, workObject.id, 1).Data) {
		t.Fatalf("first object interaction did not grant the ring and start: item=%d task=%v use=%x", s.countItems(p, altenosWeddingRingItemID), workObject.useTask != nil, packets.last(smUseObject))
	}
	if !c.altenosWeddingRingDialog(workObject, script, -1) || s.countItems(p, altenosWeddingRingItemID) != 1 {
		t.Fatal("repeated object interaction duplicated the ring or was not safely handled")
	}
	p.targetID = 0
	time.Sleep(3100 * time.Millisecond)
	if workObject.useTask != nil || questVar(p.quest(altenosWeddingRingQuestID).Vars, 0) != 0 ||
		!bytes.Equal(packets.last(smUseObject), useObject(p.ID, workObject.id, 1).Data) {
		t.Fatalf("interaction progressed after the player changed target: task=%v quest=%+v use=%x", workObject.useTask != nil, p.quest(altenosWeddingRingQuestID), packets.last(smUseObject))
	}

	p.targetID = workObject.id
	if !c.altenosWeddingRingDialog(workObject, script, -1) || workObject.useTask == nil {
		t.Fatal("ring object interaction did not restart after the interrupted use")
	}
	time.Sleep(3100 * time.Millisecond)
	if workObject.useTask != nil || questVar(p.quest(altenosWeddingRingQuestID).Vars, 0) != 1 || s.countItems(p, altenosWeddingRingItemID) != 1 ||
		!bytes.Equal(packets.last(smUseObject), useObject(p.ID, workObject.id, 0).Data) ||
		!bytes.Equal(packets.last(smEmotion), s.emote(workObject, emoteDie, 0).Data) {
		t.Fatalf("object use did not complete with the quest item and var update: task=%v quest=%+v item=%d use=%x emotion=%x", workObject.useTask != nil, p.quest(altenosWeddingRingQuestID), s.countItems(p, altenosWeddingRingItemID), packets.last(smUseObject), packets.last(smEmotion))
	}

	if !c.altenosWeddingRingDialog(workObject, script, 25) || p.quest(altenosWeddingRingQuestID).Status != "REWARD" || s.countItems(p, altenosWeddingRingItemID) != 0 ||
		!bytes.Equal(packets.last(smDialogWindow), dialogWindow(workObject.id, 10, 0).Data) {
		t.Fatalf("object switch fallthrough did not consume the ring and ready reward: quest=%+v item=%d dialog=%x", p.quest(altenosWeddingRingQuestID), s.countItems(p, altenosWeddingRingItemID), packets.last(smDialogWindow))
	}
	if !c.altenosWeddingRingDialog(start, script, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 5, altenosWeddingRingQuestID).Data) {
		t.Fatalf("reward click page = %x", packets.last(smDialogWindow))
	}
	beforeExperience := p.Exp
	if !c.altenosWeddingRingDialog(start, script, 8) || p.quest(altenosWeddingRingQuestID).Status != "COMPLETE" || p.quest(altenosWeddingRingQuestID).CompleteCount != 1 || p.Exp-beforeExperience != template.Rewards[0].Experience {
		t.Fatalf("reward selection did not complete the quest: quest=%+v experience=%d", p.quest(altenosWeddingRingQuestID), p.Exp-beforeExperience)
	}
}
