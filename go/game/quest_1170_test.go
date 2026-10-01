package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestHeadlessStoneStatueObjectItemAndMovieReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[headlessStoneStatueQuestID], d.Quests[headlessStoneStatueQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || script.StartNPC != headlessStoneStatueBodyNPCID || script.EndNPC != headlessStoneStatueBodyNPCID ||
		template.Race != "ELYOS" || template.MinLevel != 15 || template.NameID != 2204681 || len(template.Rewards) != 1 || template.Rewards[0].Experience != headlessStoneStatueRewardExp ||
		len(template.QuestWorkItems) != 1 || template.QuestWorkItems[0] != (data.QuestItem{ID: headlessStoneStatueWorkItemID, Count: 1}) {
		t.Fatalf("unexpected Headless Stone Statue metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{headlessStoneStatueBodyNPCID, headlessStoneStatueHeadNPCID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC/object %d", npcID)
		}
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.level = "ELYOS", 15
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p

	makeNPC := func(id, objectID int32) *object {
		o := questCatalogNPC(s, p, id, objectID)
		o.npc = d.Npcs[id]
		if o.npc == nil {
			o.npc = &data.NpcTemplate{ID: id}
		}
		s.initNpc(o)
		return o
	}
	body := makeNPC(headlessStoneStatueBodyNPCID, 0x31170)
	stoneHead := makeNPC(headlessStoneStatueHeadNPCID, 0x31171)

	if c.headlessStoneStatueMovieEnd(headlessStoneStatueMovieID) {
		t.Fatal("movie end completed a quest that had not started")
	}
	c.showDialog(dialogRequest(cmShowDialog, body.id, 0, 0))
	pageFound := false
	for _, packet := range packets.frames {
		pageFound = pageFound || bytes.Equal(packet, dialogWindow(0, 1011, headlessStoneStatueQuestID).Data)
	}
	if p.quest(headlessStoneStatueQuestID) == nil || p.quest(headlessStoneStatueQuestID).Status != "START" || !pageFound {
		t.Fatalf("plain click failed to start quest and show Java's page: quest=%+v pageFound=%t", p.quest(headlessStoneStatueQuestID), pageFound)
	}
	if c.headlessStoneStatueDialog(stoneHead, script, 25) {
		t.Fatal("stone head answered a non-click dialog")
	}

	p.targetID = stoneHead.id
	if c.headlessStoneStatueDialog(stoneHead, script, -1) || stoneHead.useTask == nil ||
		!bytes.Equal(packets.last(smEmotion), s.playerEmotionTo(p, emoteSit, 0, stoneHead.id, 0, 0, 0, 0).Data) {
		t.Fatalf("stone head click did not begin its sit interaction: task=%v emotion=%x", stoneHead.useTask != nil, packets.last(smEmotion))
	}
	if c.headlessStoneStatueDialog(stoneHead, script, -1) {
		t.Fatal("duplicate interaction was accepted while the first animation was pending")
	}
	time.Sleep(3100 * time.Millisecond)
	if stoneHead.useTask != nil || p.quest(headlessStoneStatueQuestID).Status != "REWARD" || s.countItems(p, headlessStoneStatueWorkItemID) != 1 || p.seen[stoneHead.id] != nil {
		t.Fatalf("stone head interaction did not despawn the object and grant the item: task=%v quest=%+v item=%d seen=%v", stoneHead.useTask != nil, p.quest(headlessStoneStatueQuestID), s.countItems(p, headlessStoneStatueWorkItemID), p.seen[stoneHead.id])
	}

	if !c.headlessStoneStatueDialog(body, script, -1) || s.countItems(p, headlessStoneStatueWorkItemID) != 0 ||
		!bytes.Equal(packets.last(smPlayMovie), playMovie(headlessStoneStatueMovieID).Data) {
		t.Fatalf("reward NPC did not consume the work item and start movie 16: item=%d movie=%x", s.countItems(p, headlessStoneStatueWorkItemID), packets.last(smPlayMovie))
	}
	if c.headlessStoneStatueMovieEnd(headlessStoneStatueMovieID+1) || p.quest(headlessStoneStatueQuestID).Status != "REWARD" {
		t.Fatal("wrong movie completed the quest")
	}
	beforeExp := p.Exp
	if !c.headlessStoneStatueMovieEnd(headlessStoneStatueMovieID) || p.quest(headlessStoneStatueQuestID).Status != "COMPLETE" || p.quest(headlessStoneStatueQuestID).CompleteCount != 1 || p.Exp-beforeExp != headlessStoneStatueRewardExp {
		t.Fatalf("movie end failed to complete the quest and award experience: quest=%+v exp=%d", p.quest(headlessStoneStatueQuestID), p.Exp-beforeExp)
	}
	statusPackets := 0
	for _, packet := range packets.frames {
		if len(packet) > 0 && packet[0] == smQuestAccepted {
			statusPackets++
		}
	}
	if statusPackets != 4 {
		t.Fatalf("quest status packet count=%d, expected start, reward, completion, and Java's duplicate completion update", statusPackets)
	}
}
