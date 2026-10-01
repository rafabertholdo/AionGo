package game

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func starterQuestPlayer(t *testing.T, race string) (*Server, *player, *conn, *questPackets, *recordedQuests) {
	t.Helper()
	s := testServer(staticDataOrSkip(t))
	saver := &recordedQuests{}
	s.quests = saver
	p := wrathchild(s)
	p.Race = race
	p.seen = map[int32]*object{}
	p.cube = nil
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	return s, p, c, packets, saver
}

func questNPC(s *Server, p *player, templateID, objectID int32) *object {
	o := &object{id: objectID, worldID: p.WorldID, x: p.X + 2, y: p.Y, z: p.Z,
		npc: s.data.Npcs[templateID], watchers: map[int32]*player{p.ID: p}}
	p.seen[objectID] = o
	s.byID[objectID] = o
	return o
}

func selectQuestDialog(c *conn, o *object, dialogID uint16, questID int32) {
	c.dialogSelect(dialogRequest(cmDialogSelect, o.id, dialogID, questID))
}

func TestKerubarHuntProgressAndTurnInMatchesClient19(t *testing.T) {
	s, p, c, packets, saver := starterQuestPlayer(t, "ELYOS")
	p.quests = []store.Quest{{ID: 1101, Status: "COMPLETE", CompleteCount: 1}}
	mires := questNPC(s, p, 203057, 0x30001)
	if got := s.nearbyQuests(p).Data; !bytes.Equal(got, []byte{smNearbyQuests, 1, 0, 0, 0, 0x4e, 4, 0, 0}) {
		t.Fatalf("1102 marker = %x", got)
	}
	selectQuestDialog(c, mires, 25, 1102)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(mires.id, 1011, 1102).Data) {
		t.Errorf("quest offer = %x", got)
	}
	selectQuestDialog(c, mires, 1007, 1102)
	selectQuestDialog(c, mires, 1002, 1102)
	if q := p.quest(1102); q == nil || q.Status != "START" || len(saver.saved) != 1 {
		t.Fatalf("hunt did not start: %+v, %+v", q, saver.saved)
	}
	selectQuestDialog(c, mires, 25, 1102)
	if got := packets.last(smDialogWindow); bytes.Equal(got, dialogWindow(mires.id, 1352, 1102).Data) {
		t.Fatal("turn-in opened before the kills")
	}
	for index, templateID := range []int32{210133, 210134, 210705} {
		killed := &object{npc: s.data.Npcs[templateID]}
		s.recordQuestKill(killed, p)
		if q := p.quest(1102); q.Vars != int32(index+1) {
			t.Fatalf("after kill %d, quest vars = %d", index+1, q.Vars)
		}
		want := "024e0400000300" + []string{"010000000000", "020000000000", "030000000000"}[index]
		if got := hex.EncodeToString(packets.last(smQuestAccepted)[1:]); got != want {
			t.Errorf("kill %d packet = %s, want %s", index+1, got, want)
		}
	}
	s.recordQuestKill(&object{npc: s.data.Npcs[210133]}, p)
	if p.quest(1102).Vars != 3 || len(saver.saved) != 4 {
		t.Fatal("extra kill increased progress")
	}
	selectQuestDialog(c, mires, 25, 1102)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(mires.id, 1352, 1102).Data) {
		t.Errorf("turn-in dialog = %x", got)
	}
	selectQuestDialog(c, mires, 1009, 1102)
	if got := hex.EncodeToString(packets.last(smQuestAccepted)[1:]); got != "024e0400000400040000000000" {
		t.Errorf("reward state = %s", got)
	}
	selectQuestDialog(c, mires, 17, 1102)
	if q := p.quest(1102); q.Status != "COMPLETE" || q.Vars != 4 || len(saver.saved) != 6 {
		t.Fatalf("hunt not complete: %+v, saves=%d", q, len(saver.saved))
	}
	if p.Exp != 536 || p.kinah.Count != 100 {
		t.Errorf("hunt rewards: exp=%d kinah=%d", p.Exp, p.kinah.Count)
	}
	if got := s.nearbyQuests(p).Data; !bytes.Equal(got, []byte{smNearbyQuests, 2, 0, 0, 0, 0x4e, 4, 0, 0, 0x4f, 4, 0, 0}) {
		t.Errorf("follow-up markers = %x", got)
	}
}

func TestGrainThievesCollectAndTurnIn(t *testing.T) {
	s, p, c, packets, saver := starterQuestPlayer(t, "ELYOS")
	p.quests = []store.Quest{{ID: 1101, Status: "COMPLETE", CompleteCount: 1}, {ID: 1102, Status: "COMPLETE", Vars: 4, CompleteCount: 1}}
	mires := questNPC(s, p, 203057, 0x30001)
	selectQuestDialog(c, mires, 25, 1103)
	selectQuestDialog(c, mires, 1007, 1103)
	selectQuestDialog(c, mires, 1002, 1103)
	if p.quest(1103) == nil || p.quest(1103).Status != "START" {
		t.Fatal("Grain Thieves did not start")
	}
	selectQuestDialog(c, mires, 33, 1103)
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(mires.id, 2716, 1103).Data) {
		t.Errorf("missing-item dialog = %x", got)
	}
	for i := int32(0); i < 4; i++ {
		sack := &object{id: 0x31000 + i, npc: s.data.Npcs[700105]}
		s.registerDrop(sack, p)
		if len(sack.loot.items) != 1 || sack.loot.items[0].item != 182200201 {
			t.Fatalf("sack %d quest drop = %+v", i, sack.loot.items)
		}
		if !s.addItem(p, sack.loot.items[0].item, 1) {
			t.Fatal("could not collect grain")
		}
	}
	selectQuestDialog(c, mires, 25, 1103)
	selectQuestDialog(c, mires, 33, 1103)
	if q := p.quest(1103); q.Status != "REWARD" || q.Vars != 1 || len(p.cube) != 0 {
		t.Fatalf("grain not turned in: %+v cube=%+v", q, p.cube)
	}
	selectQuestDialog(c, mires, 17, 1103)
	if q := p.quest(1103); q.Status != "COMPLETE" || len(saver.saved) != 3 || p.Exp != 586 || p.kinah.Count != 80 {
		t.Fatalf("Grain Thieves reward: %+v saves=%d exp=%d kinah=%d", q, len(saver.saved), p.Exp, p.kinah.Count)
	}
	if got := s.nearbyQuests(p).Data; !bytes.Equal(got, []byte{smNearbyQuests, 3, 0, 0, 0, 0x4e, 4, 0, 0, 0x4f, 4, 0, 0, 0x50, 4, 0, 0}) {
		t.Errorf("Report to Polinia marker = %x", got)
	}
}

func TestAsmodianHuntGivesFixedItemReward(t *testing.T) {
	s, p, c, _, _ := starterQuestPlayer(t, "ASMODIANS")
	p.quests = []store.Quest{{ID: 2101, Status: "COMPLETE", CompleteCount: 1}}
	vandar := questNPC(s, p, 203504, 0x30001)
	selectQuestDialog(c, vandar, 1002, 2102)
	for _, id := range []int32{210364, 210363, 210364, 210363} {
		s.recordQuestKill(&object{npc: s.data.Npcs[id]}, p)
	}
	selectQuestDialog(c, vandar, 1009, 2102)
	selectQuestDialog(c, vandar, 17, 2102)
	if q := p.quest(2102); q == nil || q.Status != "COMPLETE" || len(p.cube) != 1 || p.cube[0].ItemID != 169300002 || p.cube[0].Count != 10 {
		t.Fatalf("Asmodian hunt reward: quest=%+v cube=%+v", q, p.cube)
	}
	if p.Exp != 386 || p.kinah.Count != 30 {
		t.Errorf("Asmodian hunt exp=%d kinah=%d", p.Exp, p.kinah.Count)
	}
}

func TestGrainSackUseOpensQuestLoot(t *testing.T) {
	s, p, c, packets, _ := starterQuestPlayer(t, "ELYOS")
	p.quests = []store.Quest{{ID: 1103, Status: "START"}}
	p.spawned = true
	sack := questNPC(s, p, 700105, 0x31000)
	s.initNpc(sack)
	sack.watchers[p.ID] = p
	sack.interval = 295
	sack.homeX, sack.homeY, sack.homeZ = sack.x, sack.y, sack.z
	s.grid[cellOf(sack.worldID, sack.x, sack.y)] = []*object{sack}
	c.showDialog(dialogRequest(cmShowDialog, sack.id, 0, 0))
	if got := packets.last(smUseObject); !bytes.Equal(got, useObject(p.ID, sack.id, 1).Data) {
		t.Fatalf("grain sack did not start its use animation: %x", got)
	}
	time.Sleep(3200 * time.Millisecond)
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if got := packets.last(smUseObject); !bytes.Equal(got, useObject(p.ID, sack.id, 0).Data) {
		t.Errorf("grain sack use did not finish: %x", got)
	}
	if !sack.dead || sack.loot == nil || len(sack.loot.items) != 1 || sack.loot.items[0].item != 182200201 || sack.loot.looting != p.ID {
		t.Errorf("grain sack loot = %+v, dead=%t", sack.loot, sack.dead)
	}
	if packets.last(smLootItemlist) == nil {
		t.Error("loot window did not open")
	}
}

func TestNextReportQuestsRequirePreviousCompletion(t *testing.T) {
	for _, tc := range []struct {
		race                    string
		questID, prerequisiteID int32
		startNPC, endNPC        int32
		experience, kinah       int64
	}{
		{"ELYOS", 1104, 1103, 203057, 203059, 210, 30},
		{"ASMODIANS", 2103, 2102, 203504, 203501, 590, 110},
	} {
		t.Run(tc.race, func(t *testing.T) {
			s, p, c, _, _ := starterQuestPlayer(t, tc.race)
			start := questNPC(s, p, tc.startNPC, 0x30001)
			end := questNPC(s, p, tc.endNPC, 0x30002)
			selectQuestDialog(c, start, 1002, tc.questID)
			if p.quest(tc.questID) != nil {
				t.Fatal("quest started without its prerequisite")
			}
			p.quests = append(p.quests, store.Quest{ID: tc.prerequisiteID, Status: "COMPLETE", CompleteCount: 1})
			selectQuestDialog(c, start, 1002, tc.questID)
			if q := p.quest(tc.questID); q == nil || q.Status != "START" {
				t.Fatalf("quest did not start after prerequisite: %+v", q)
			}
			selectQuestDialog(c, end, 25, tc.questID)
			selectQuestDialog(c, end, 1009, tc.questID)
			selectQuestDialog(c, end, 17, tc.questID)
			if q := p.quest(tc.questID); q.Status != "COMPLETE" || p.Exp != 206+tc.experience || p.kinah.Count != tc.kinah {
				t.Errorf("report turn-in: quest=%+v exp=%d kinah=%d", q, p.Exp, p.kinah.Count)
			}
		})
	}
}

func TestAsmodianFruitQuotaDropsAndTurnIn(t *testing.T) {
	s, p, c, _, _ := starterQuestPlayer(t, "ASMODIANS")
	collector := questNPC(s, p, 203502, 0x30001)
	selectQuestDialog(c, collector, 1002, 2104)
	if q := p.quest(2104); q == nil || q.Status != "START" {
		t.Fatalf("fruit quota did not start: %+v", q)
	}
	for i := int32(0); i < 5; i++ {
		fruit := &object{id: 0x31000 + i, npc: s.data.Npcs[700124]}
		s.registerDrop(fruit, p)
		if len(fruit.loot.items) != 1 || fruit.loot.items[0].item != 182203104 {
			t.Fatalf("fruit %d drop = %+v", i, fruit.loot.items)
		}
		if !s.addItem(p, fruit.loot.items[0].item, 1) {
			t.Fatal("could not collect fruit")
		}
	}
	selectQuestDialog(c, collector, 33, 2104)
	selectQuestDialog(c, collector, 17, 2104)
	if q := p.quest(2104); q.Status != "COMPLETE" || len(p.cube) != 0 || p.Exp != 726 || p.kinah.Count != 200 {
		t.Errorf("fruit quota turn-in: %+v cube=%+v exp=%d kinah=%d", q, p.cube, p.Exp, p.kinah.Count)
	}
}

func TestQuestRestrictionsFromJavaMetadata(t *testing.T) {
	s, p, _, _, _ := starterQuestPlayer(t, "ELYOS")
	if !s.canStartQuest(p, s.data.QuestScripts[9510]) {
		t.Error("a quest without a race restriction was hidden")
	}
	p.level = 40
	if s.canStartQuest(p, s.data.QuestScripts[1572]) {
		t.Error("spirit master quest offered to a mage")
	}
	p.Class = "SPIRIT_MASTER"
	if !s.canStartQuest(p, s.data.QuestScripts[1572]) {
		t.Error("spirit master quest was hidden from its class")
	}
}

func TestSelectableQuestRewardUsesDialogChoice(t *testing.T) {
	s, p, c, _, saver := starterQuestPlayer(t, "ELYOS")
	p.quests = []store.Quest{{ID: 1971, Status: "REWARD"}}
	npc := questNPC(s, p, 203812, 0x30001)
	selectQuestDialog(c, npc, 17, 1971)
	if p.quest(1971).Status != "REWARD" {
		t.Fatal("a selectable reward was completed without a choice")
	}
	selectQuestDialog(c, npc, 9, 1971)
	if q := p.quest(1971); q.Status != "COMPLETE" {
		t.Fatalf("selected quest reward was not completed: %+v", q)
	}
	saved1971 := 0
	for _, saved := range saver.saved {
		if saved.ID == 1971 && saved.Status == "COMPLETE" {
			saved1971++
		}
	}
	if saved1971 != 1 {
		t.Fatalf("quest 1971 completion saves = %d, want 1: %+v", saved1971, saver.saved)
	}
	if len(p.cube) != 2 || p.cube[0].ItemID != 169100000 || p.cube[1].ItemID != 169200004 || p.cube[1].Count != 5 {
		t.Errorf("wrong reward items: %+v", p.cube)
	}
}

func TestQuestRewardsNeedSpaceForEveryItem(t *testing.T) {
	s, p, c, packets, saver := starterQuestPlayer(t, "ELYOS")
	p.quests = []store.Quest{{ID: 1971, Status: "REWARD"}}
	p.cube = make([]*store.Item, p.cubeLimit()-1)
	npc := questNPC(s, p, 203812, 0x30001)
	selectQuestDialog(c, npc, 8, 1971)
	if p.quest(1971).Status != "REWARD" || len(saver.saved) != 0 || packets.last(smSystemMessage) == nil {
		t.Fatal("multi-item reward completed without enough cube slots")
	}
}

func TestMonsterHuntUsesSeparateQuestCounters(t *testing.T) {
	s, p, _, _, _ := starterQuestPlayer(t, "ELYOS")
	p.quests = []store.Quest{{ID: 1112, Status: "START"}}
	script := s.data.QuestScripts[1112]
	for i := 0; i < 5; i++ {
		s.recordQuestKill(&object{npc: s.data.Npcs[210259]}, p)
		s.recordQuestKill(&object{npc: s.data.Npcs[210065]}, p)
	}
	if got := p.quest(1112).Vars; got != 5|(5<<6) || !monsterHuntComplete(p.quest(1112), script) {
		t.Fatalf("two monster groups: vars=%d, complete=%t", got, monsterHuntComplete(p.quest(1112), script))
	}
	s.recordQuestKill(&object{npc: s.data.Npcs[210260]}, p)
	if got := p.quest(1112).Vars; got != 5|(5<<6) {
		t.Errorf("counter exceeded its cap: %d", got)
	}
}

func TestQuestCompletionPersistsSpecialRewards(t *testing.T) {
	for _, tc := range []struct {
		questID int32
		check   func(*testing.T, *player, store.QuestCompletion, *questPackets)
	}{
		{1143, func(t *testing.T, p *player, reward store.QuestCompletion, packets *questPackets) {
			if reward.TitleID != 6 || len(p.titles) != 1 || p.titles[0] != 6 || packets.last(smTitleList) == nil {
				t.Errorf("title reward: %+v, titles=%v", reward, p.titles)
			}
		}},
		{1947, func(t *testing.T, p *player, reward store.QuestCompletion, packets *questPackets) {
			if reward.CubeSize != 1 || p.CubeSize != 1 || packets.last(smCubeUpdate) == nil {
				t.Errorf("cube reward: %+v, size=%d", reward, p.CubeSize)
			}
		}},
		{1956, func(t *testing.T, p *player, reward store.QuestCompletion, packets *questPackets) {
			if reward.Abyss == nil || reward.Abyss.AP != 550 || p.abyss.AP != 550 || packets.last(smAbyssRank) == nil {
				t.Errorf("abyss reward: %+v, rank=%+v", reward, p.abyss)
			}
		}},
	} {
		t.Run(fmt.Sprint(tc.questID), func(t *testing.T) {
			s, p, c, packets, saver := starterQuestPlayer(t, "ELYOS")
			p.quests = []store.Quest{{ID: tc.questID, Status: "REWARD"}}
			c.finishQuest(s.data.QuestScripts[tc.questID], 0x30001, 17)
			if p.quest(tc.questID).Status != "COMPLETE" || len(saver.rewards) != 1 {
				t.Fatalf("quest completion: %+v, rewards=%+v", p.quest(tc.questID), saver.rewards)
			}
			tc.check(t, p, saver.rewards[0], packets)
		})
	}
}
