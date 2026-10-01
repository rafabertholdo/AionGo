package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestTheArchonOfStormsProgressTransformationItemAndReward(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[archonOfStormsQuestID], d.Quests[archonOfStormsQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || !script.LevelUpStart || script.StartNPC != archonOfStormsStartNPCID || script.EndNPC != archonOfStormsStartNPCID || script.ItemID != archonOfStormsItemID || template.Race != "ELYOS" || template.MinLevel != 36 || template.NameID != 2204217 ||
		len(template.Rewards) != 1 || template.Rewards[0].Experience != 2467700 || len(template.Rewards[0].Items) != 1 || template.Rewards[0].Items[0] != (data.QuestItem{ID: 123000885, Count: 1}) {
		t.Fatalf("unexpected The Archon of Storms metadata: script=%+v template=%+v", script, template)
	}
	for _, npcID := range []int32{archonOfStormsStartNPCID, archonOfStormsSecondNPCID, archonOfStormsGeyserObjectID, archonOfStormsItemNPCID} {
		if len(d.QuestCustomTalks[npcID])+len(d.QuestEnds[npcID]) == 0 {
			t.Fatalf("quest talk index is missing NPC/object %d", npcID)
		}
	}
	foundItemUse := false
	for _, registered := range d.QuestItemUses[archonOfStormsItemID] {
		foundItemUse = foundItemUse || registered.ID == archonOfStormsQuestID
	}
	if !foundItemUse {
		t.Fatal("quest item-use index is missing item 182201619")
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 35
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: archonOfStormsQuestID, Status: "LOCKED"}, {ID: 1500, Status: "START"}}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.archonOfStormsLevelUp() {
		t.Fatal("quest unlocked below level 36")
	}
	p.level = 36
	if c.archonOfStormsLevelUp() {
		t.Fatal("quest unlocked before quest 1500 completed")
	}
	p.quest(1500).Status = "COMPLETE"
	if !c.archonOfStormsLevelUp() || p.quest(archonOfStormsQuestID).Status != "START" || c.archonOfStormsLevelUp() {
		t.Fatalf("quest did not unlock exactly once: %+v", p.quest(archonOfStormsQuestID))
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
	startNPC := npc(archonOfStormsStartNPCID, 0x31151)
	secondNPC := npc(archonOfStormsSecondNPCID, 0x31152)
	geyser := npc(archonOfStormsGeyserObjectID, 0x31153)
	itemNPC := npc(archonOfStormsItemNPCID, 0x31154)
	selectDialog := func(o *object, dialogID int32) bool {
		return c.archonOfStormsDialog(o, script, dialogID)
	}
	if !selectDialog(startNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 1011, archonOfStormsQuestID).Data) {
		t.Fatalf("opening page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(startNPC, 10000) || questVar(p.quest(archonOfStormsQuestID).Vars, 0) != 1 {
		t.Fatalf("opening dialogue did not advance: %+v", p.quest(archonOfStormsQuestID))
	}
	if !selectDialog(secondNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(secondNPC.id, 1352, archonOfStormsQuestID).Data) {
		t.Fatalf("second NPC opening page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(secondNPC, 10001) || questVar(p.quest(archonOfStormsQuestID).Vars, 0) != 2 {
		t.Fatalf("second NPC did not advance to geyser: %+v", p.quest(archonOfStormsQuestID))
	}
	p.targetID = geyser.id
	if c.archonOfStormsDialog(geyser, script, -1) {
		t.Fatal("geyser Java branch returns false so its plain click should fall through")
	}
	if !bytes.Equal(packets.last(smUseObject), useObject(p.ID, geyser.id, 1).Data) {
		t.Fatalf("geyser interaction did not start: %x", packets.last(smUseObject))
	}
	time.Sleep(3100 * time.Millisecond)
	if !bytes.Equal(packets.last(smUseObject), useObject(p.ID, geyser.id, 0).Data) || !bytes.Equal(packets.last(smPlayMovie), playMovie(archonOfStormsMovieID).Data) || questVar(p.quest(archonOfStormsQuestID).Vars, 0) != 2 {
		t.Fatalf("geyser did not finish and play movie: use=%x movie=%x quest=%+v", packets.last(smUseObject), packets.last(smPlayMovie), p.quest(archonOfStormsQuestID))
	}
	if c.archonOfStormsMovieEnd(192) || questVar(p.quest(archonOfStormsQuestID).Vars, 0) != 2 {
		t.Fatal("unrelated movie advanced the quest")
	}
	if !c.archonOfStormsMovieEnd(archonOfStormsMovieID) || questVar(p.quest(archonOfStormsQuestID).Vars, 0) != 3 || p.transformed != archonOfStormsModelID || !p.fx.isSet(effectShapeChange) || !bytes.Equal(packets.last(smTransform), transformPacket(p).Data) {
		t.Fatalf("movie completion did not apply transformation: quest=%+v model=%d effect=%v packet=%x", p.quest(archonOfStormsQuestID), p.transformed, p.fx.isSet(effectShapeChange), packets.last(smTransform))
	}
	if c.archonOfStormsMovieEnd(archonOfStormsMovieID) {
		t.Fatal("repeated movie completion was handled")
	}
	if !selectDialog(secondNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(secondNPC.id, 2034, archonOfStormsQuestID).Data) {
		t.Fatalf("second NPC var-three page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(secondNPC, 10001) || questVar(p.quest(archonOfStormsQuestID).Vars, 0) != 4 {
		t.Fatalf("10001 fallthrough did not advance var three to four: %+v", p.quest(archonOfStormsQuestID))
	}
	if !selectDialog(itemNPC, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(itemNPC.id, 2375, archonOfStormsQuestID).Data) {
		t.Fatalf("item NPC page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(itemNPC, 10004) || questVar(p.quest(archonOfStormsQuestID).Vars, 0) != 5 || s.countItems(p, archonOfStormsItemID) != 1 {
		t.Fatalf("item NPC did not advance and grant the item: quest=%+v items=%d", p.quest(archonOfStormsQuestID), s.countItems(p, archonOfStormsItemID))
	}
	var item *store.Item
	for _, candidate := range p.cube {
		if candidate.ItemID == archonOfStormsItemID {
			item = candidate
			break
		}
	}
	if item == nil {
		t.Fatal("granted geyser item not found in cube")
	}
	if c.archonOfStormsItemUse(item) {
		t.Fatal("item use outside Patema Geyser was handled")
	}
	p.zone = &data.Zone{Name: archonOfStormsZoneName, MapID: 210040000}
	if !c.archonOfStormsItemUse(item) || !bytes.Equal(packets.last(smItemUsageAnimation), itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 3000, 0, 0).Data) {
		t.Fatalf("geyser item use did not start: %x", packets.last(smItemUsageAnimation))
	}
	time.Sleep(3100 * time.Millisecond)
	quest := p.quest(archonOfStormsQuestID)
	if quest.Status != "REWARD" || questVar(quest.Vars, 0) != 5 || s.countItems(p, archonOfStormsItemID) != 0 || !bytes.Equal(packets.last(smItemUsageAnimation), itemUsageAnimation(p.ID, item.UniqueID, item.ItemID, 0, 1, 0).Data) || !bytes.Equal(packets.last(smPlayMovie), playMovie(archonOfStormsItemMovieID).Data) {
		t.Fatalf("geyser item use did not finish quest: quest=%+v item=%d animation=%x movie=%x", quest, s.countItems(p, archonOfStormsItemID), packets.last(smItemUsageAnimation), packets.last(smPlayMovie))
	}
	if !selectDialog(startNPC, -1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(startNPC.id, 5, archonOfStormsQuestID).Data) {
		t.Fatalf("reward page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(startNPC, 17) || quest.Status != "COMPLETE" || quest.CompleteCount != 1 || s.countItems(p, 123000885) != 1 || p.Exp != d.ExpStart(p.level)+2467700 {
		t.Fatalf("fixed reward did not complete quest: quest=%+v reward=%d exp=%d", quest, s.countItems(p, 123000885), p.Exp)
	}
}
