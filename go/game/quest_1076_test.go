package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestFragmentOfMemory2ProgressCollectionItemUseAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[fragmentOfMemory2QuestID], d.Quests[fragmentOfMemory2QuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || script.StartNPC != fragmentOfMemory2StartNPCID || script.EndNPC != fragmentOfMemory2RewardNPCID || script.ItemID != fragmentOfMemory2QuestItemID || template.Race != "ELYOS" || template.MinLevel != 35 || template.NameID != 2204251 ||
		len(template.FinishedQuestConditions) != 2 || template.FinishedQuestConditions[0] != 1074 || template.FinishedQuestConditions[1] != 1038 ||
		len(template.CollectItems) != 3 || template.CollectItems[0] != (data.QuestItem{ID: 167000323, Count: 1}) || template.CollectItems[1] != (data.QuestItem{ID: 152000309, Count: 1}) || template.CollectItems[2] != (data.QuestItem{ID: fragmentOfMemory2MonsterFragment, Count: 3}) ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 2108600 || template.Rewards[0].Kinah != 40000 || template.Rewards[0].AbyssPoints != 2500 || len(template.Rewards[0].Items) != 0 || len(template.Rewards[0].SelectableItems) != 0 {
		t.Fatalf("unexpected Fragment of Memory 2 metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{fragmentOfMemory2StartNPCID, fragmentOfMemory2ReportNPCID, fragmentOfMemory2CollectionNPCID, fragmentOfMemory2EndNPCID, fragmentOfMemory2RewardNPCID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC %d", npcID)
		}
	}
	foundItemUse := false
	for _, registered := range d.QuestItemUses[fragmentOfMemory2QuestItemID] {
		foundItemUse = foundItemUse || registered.ID == fragmentOfMemory2QuestID
	}
	if !foundItemUse {
		t.Fatal("item-use index is missing quest item 182202006")
	}
	foundDrop := false
	for _, registered := range d.QuestDropsByNPC[fragmentOfMemory2DropNPCID] {
		foundDrop = foundDrop || registered.ID == fragmentOfMemory2QuestID
	}
	if !foundDrop {
		t.Fatal("quest-drop index is missing NPC 255160")
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 34
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.kinah = &store.Item{UniqueID: 0x107601, ItemID: data.Kinah, Owner: p.ID, Count: 100000}
	p.spawned = true
	p.quests = []store.Quest{
		{ID: fragmentOfMemory2QuestID, Status: "LOCKED"},
		{ID: 1701, Status: "START"},
		{ID: 1074, Status: "COMPLETE"},
		{ID: 1038, Status: "COMPLETE"},
	}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.fragmentOfMemory2LevelUp() {
		t.Fatal("quest unlocked below level 35")
	}
	p.level = 35
	if c.fragmentOfMemory2LevelUp() {
		t.Fatal("quest unlocked before Java prerequisite 1701 completed")
	}
	p.quest(1701).Status = "COMPLETE"
	if !c.fragmentOfMemory2LevelUp() || p.quest(fragmentOfMemory2QuestID).Status != "START" || c.fragmentOfMemory2LevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(fragmentOfMemory2QuestID))
	}

	npc := func(id, objectID int32) *object {
		o := questCatalogNPC(s, p, id, objectID)
		o.npc = d.Npcs[id]
		if o.npc == nil {
			t.Fatalf("NPC template %d is missing", id)
		}
		s.initNpc(o)
		return o
	}
	startNPC := npc(fragmentOfMemory2StartNPCID, 0x31761)
	reportNPC := npc(fragmentOfMemory2ReportNPCID, 0x31762)
	collectionNPC := npc(fragmentOfMemory2CollectionNPCID, 0x31763)
	endNPC := npc(fragmentOfMemory2EndNPCID, 0x31764)
	rewardNPC := npc(fragmentOfMemory2RewardNPCID, 0x31765)
	dialog := func(o *object, dialogID int32) bool {
		return c.fragmentOfMemory2Dialog(o, script, dialogID)
	}
	if dialog(reportNPC, 25) || !bytes.Equal(packets.last(smPlayMovie), playMovie(102).Data) {
		t.Fatalf("unmatched var-zero report should play movie 102 and fall through: movie=%x", packets.last(smPlayMovie))
	}
	if dialog(reportNPC, 1353) || !bytes.Equal(packets.last(smPlayMovie), playMovie(102).Data) {
		t.Fatalf("dialog 1353 should play movie 102 and be echoed: movie=%x", packets.last(smPlayMovie))
	}
	if !dialog(startNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1011, fragmentOfMemory2QuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	if !dialog(startNPC, 10000) || questVar(p.quest(fragmentOfMemory2QuestID).Vars, 0) != 1 {
		t.Fatalf("opening dialogue did not advance: %+v", p.quest(fragmentOfMemory2QuestID))
	}
	if !dialog(reportNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(reportNPC.id, 1352, fragmentOfMemory2QuestID).Data) {
		t.Fatalf("report NPC var-one page = %x", packets.last(smDialogWindow))
	}
	if !dialog(reportNPC, 10001) || questVar(p.quest(fragmentOfMemory2QuestID).Vars, 0) != 2 {
		t.Fatalf("report NPC did not advance to collection: %+v", p.quest(fragmentOfMemory2QuestID))
	}
	if !dialog(collectionNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(collectionNPC.id, 1693, fragmentOfMemory2QuestID).Data) {
		t.Fatalf("collection NPC page = %x", packets.last(smDialogWindow))
	}
	if !dialog(collectionNPC, 33) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(collectionNPC.id, 10001, fragmentOfMemory2QuestID).Data) {
		t.Fatalf("missing collect items page = %x", packets.last(smDialogWindow))
	}
	for _, item := range template.CollectItems {
		if !s.addItem(p, item.ID, item.Count) {
			t.Fatalf("could not add collection item %d", item.ID)
		}
	}
	if !dialog(collectionNPC, 33) || questVar(p.quest(fragmentOfMemory2QuestID).Vars, 0) != 3 || s.countItems(p, 167000323) != 0 || s.countItems(p, 152000309) != 0 || s.countItems(p, fragmentOfMemory2MonsterFragment) != 0 || s.countItems(p, fragmentOfMemory2QuestItemID) != 1 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(collectionNPC.id, 10000, fragmentOfMemory2QuestID).Data) {
		t.Fatalf("collection did not consume materials and grant quest item: quest=%+v item=%d page=%x", p.quest(fragmentOfMemory2QuestID), s.countItems(p, fragmentOfMemory2QuestItemID), packets.last(smDialogWindow))
	}
	if !dialog(reportNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(reportNPC.id, 2034, fragmentOfMemory2QuestID).Data) {
		t.Fatalf("report NPC var-three page = %x", packets.last(smDialogWindow))
	}
	if !dialog(reportNPC, 10001) || questVar(p.quest(fragmentOfMemory2QuestID).Vars, 0) != 4 {
		t.Fatalf("10001 did not fall through to 10003 at var three: %+v", p.quest(fragmentOfMemory2QuestID))
	}
	var item *store.Item
	for _, candidate := range p.cube {
		if candidate.ItemID == fragmentOfMemory2QuestItemID {
			item = candidate
			break
		}
	}
	if item == nil || !c.fragmentOfMemory2ItemUse(item) || !bytes.Equal(packets.last(smItemUsageAnimation), itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 1000, 0, 0).Data) {
		t.Fatalf("quest item use did not start: item=%+v animation=%x", item, packets.last(smItemUsageAnimation))
	}
	time.Sleep(1100 * time.Millisecond)
	if questVar(p.quest(fragmentOfMemory2QuestID).Vars, 0) != 5 || s.countItems(p, fragmentOfMemory2QuestItemID) != 1 || !bytes.Equal(packets.last(smItemUsageAnimation), itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0).Data) || !bytes.Equal(packets.last(smPlayMovie), playMovie(fragmentOfMemory2ItemMovieID).Data) {
		t.Fatalf("quest item use did not advance without consuming item: quest=%+v item=%d animation=%x movie=%x", p.quest(fragmentOfMemory2QuestID), s.countItems(p, fragmentOfMemory2QuestItemID), packets.last(smItemUsageAnimation), packets.last(smPlayMovie))
	}
	if !s.addItem(p, fragmentOfMemory2QuestItemID, 1) {
		t.Fatal("could not add duplicate quest item for Java remove-all branch")
	}
	if !dialog(reportNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(reportNPC.id, 2716, fragmentOfMemory2QuestID).Data) {
		t.Fatalf("report NPC var-five page = %x", packets.last(smDialogWindow))
	}
	if !dialog(reportNPC, 10001) || questVar(p.quest(fragmentOfMemory2QuestID).Vars, 0) != 6 || s.countItems(p, fragmentOfMemory2QuestItemID) != 0 {
		t.Fatalf("10001 did not fall through through 10003 into 10005/remove-all: quest=%+v item=%d", p.quest(fragmentOfMemory2QuestID), s.countItems(p, fragmentOfMemory2QuestItemID))
	}
	if !dialog(endNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(endNPC.id, 3057, fragmentOfMemory2QuestID).Data) {
		t.Fatalf("end NPC page = %x", packets.last(smDialogWindow))
	}
	if !dialog(endNPC, 10255) || p.quest(fragmentOfMemory2QuestID).Status != "REWARD" {
		t.Fatalf("end NPC did not ready reward: %+v", p.quest(fragmentOfMemory2QuestID))
	}
	if !dialog(rewardNPC, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(rewardNPC.id, 10002, fragmentOfMemory2QuestID).Data) {
		t.Fatalf("reward preview = %x", packets.last(smDialogWindow))
	}
	if !dialog(rewardNPC, 1009) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(rewardNPC.id, 5, fragmentOfMemory2QuestID).Data) {
		t.Fatalf("reward selection page = %x", packets.last(smDialogWindow))
	}
	quest := p.quest(fragmentOfMemory2QuestID)
	beforeExperience, beforeKinah, beforeAP := p.Exp, p.kinah.Count, p.abyss.AP
	if !dialog(rewardNPC, 17) || quest.Status != "COMPLETE" || quest.CompleteCount != 1 || p.Exp-beforeExperience != 2108600 || p.kinah.Count-beforeKinah != 40000 || p.abyss.AP-beforeAP != 2500 {
		t.Fatalf("fixed reward failed: quest=%+v XP=%d kinah=%d AP=%d", quest, p.Exp-beforeExperience, p.kinah.Count-beforeKinah, p.abyss.AP-beforeAP)
	}
}
