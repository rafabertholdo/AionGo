package game

import (
	"bytes"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func sealingAbyssGateTestFixture(t *testing.T, status string, vars int32) (*Server, *player, *conn, *data.QuestScript) {
	t.Helper()
	d := staticDataOrSkip(t)
	s := testServer(d)
	p := wrathchild(s)
	p.Race = "ELYOS"
	p.level = 16
	p.WorldID = 210030000
	p.cube = []*store.Item{}
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: p.ID}
	p.seen = map[int32]*object{}
	p.spawned = true
	p.quests = []store.Quest{{ID: sealingAbyssGateQuestID, Status: status, Vars: vars}}
	for _, id := range sealingAbyssGatePrerequisites {
		p.quests = append(p.quests, store.Quest{ID: id, Status: "COMPLETE"})
	}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	return s, p, c, &data.QuestScript{ID: sealingAbyssGateQuestID, Kind: data.QuestCustom, StartNPC: sealingAbyssGatePernosID, EndNPC: sealingAbyssGatePernosID}
}

func sealingAbyssGateTestObject(s *Server, p *player, npcID, objectID int32) *object {
	o := &object{id: objectID, worldID: p.WorldID, instance: p.instance, x: p.X + 2, y: p.Y, z: p.Z,
		npc: s.data.Npcs[npcID], watchers: map[int32]*player{p.ID: p}}
	if o.npc == nil {
		o.npc = &data.NpcTemplate{ID: npcID, Level: 1}
	}
	s.initNpc(o)
	o.watchers[p.ID] = p
	p.seen[objectID] = o
	s.byID[objectID] = o
	return o
}

func TestSealingAbyssGatePrerequisitesAndPernos(t *testing.T) {
	s, p, c, script := sealingAbyssGateTestFixture(t, "LOCKED", 0)
	packets := &questPackets{}
	c.tap = packets.tap
	pernos := sealingAbyssGateTestObject(s, p, sealingAbyssGatePernosID, 0x31020)

	p.level = 14
	if c.sealingAbyssGateStart() {
		t.Fatal("quest unlocked below level 15")
	}
	p.level = 15
	p.quest(1023).Status = "START"
	if c.sealingAbyssGateStart() {
		t.Fatal("quest unlocked with an incomplete prerequisite")
	}
	p.quest(1023).Status = "COMPLETE"
	if !c.sealingAbyssGateStart() || c.sealingAbyssGateStart() || p.quest(sealingAbyssGateQuestID).Status != "START" {
		t.Fatalf("quest start = %+v", p.quest(sealingAbyssGateQuestID))
	}
	if !c.sealingAbyssGateDialog(pernos, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(pernos.id, 1011, sealingAbyssGateQuestID).Data) {
		t.Fatalf("Pernos opening dialog = %x", packets.last(smDialogWindow))
	}
	if !c.sealingAbyssGateDialog(pernos, script, 10000) || c.sealingAbyssGateDialog(pernos, script, 10000) || questVar(p.quest(sealingAbyssGateQuestID).Vars, 0) != 1 {
		t.Fatalf("Pernos advancement = %+v", p.quest(sealingAbyssGateQuestID))
	}
	if c.sealingAbyssGateDialog(pernos, &data.QuestScript{ID: 1021}, 10000) {
		t.Fatal("wrong quest script was handled")
	}
}

func TestSealingAbyssGateInstanceBossSealAndExit(t *testing.T) {
	s, p, c, script := sealingAbyssGateTestFixture(t, "START", 1)
	p.X, p.Y, p.Z = 270, 174, 204
	entry := sealingAbyssGateTestObject(s, p, sealingAbyssGateEntryID, 0x32020)
	p.targetID = entry.id
	if !c.sealingAbyssGateDialog(entry, script, -1) || c.sealingAbyssGateDialog(entry, script, -1) {
		t.Fatal("entry gate did not start one use animation")
	}
	time.Sleep(3200 * time.Millisecond)
	if p.WorldID != sealingAbyssGateMapID || questVar(p.quest(sealingAbyssGateQuestID).Vars, 0) != 2 || s.registeredInstance(sealingAbyssGateMapID, p.ID) == nil {
		t.Fatalf("instance entry failed: world=%d instance=%d quest=%+v", p.WorldID, p.instance, p.quest(sealingAbyssGateQuestID))
	}
	// Teleporting to a new map leaves the player despawned until the client
	// finishes loading it and sends LEVEL_READY.
	s.spawn(p)
	guardian := sealingAbyssGateTestObject(s, p, sealingAbyssGateGuardianID, 0x32021)
	p.targetID = guardian.id
	if !c.sealingAbyssGateDialog(guardian, script, -1) {
		t.Fatal("guardian stone did not start its use")
	}
	time.Sleep(3200 * time.Millisecond)
	var bossSpawned bool
	for _, candidate := range s.byID {
		if candidate.npc != nil && candidate.npc.ID == sealingAbyssGateBossID && candidate.worldID == p.WorldID && candidate.instance == p.instance {
			bossSpawned = true
			break
		}
	}
	if !bossSpawned {
		t.Fatal("guardian stone did not spawn Kuninasha in the player's instance")
	}
	dropFound := false
	for _, drop := range s.data.Quests[sealingAbyssGateQuestID].QuestDrops {
		if drop.NPCID == sealingAbyssGateBossID && drop.ItemID == sealingAbyssGateItemID && drop.Chance == 100 {
			dropFound = true
		}
	}
	if !dropFound {
		t.Fatal("Kuninasha's guaranteed quest key drop is missing from quest metadata")
	}
	if !s.addItem(p, sealingAbyssGateItemID, 1) {
		t.Fatal("could not add the quest key for the seal branch")
	}
	seal := sealingAbyssGateTestObject(s, p, sealingAbyssGateSealID, 0x32022)
	p.targetID = seal.id
	if !c.sealingAbyssGateDialog(seal, script, -1) {
		t.Fatal("seal object rejected the quest key")
	}
	time.Sleep(3200 * time.Millisecond)
	if questVar(p.quest(sealingAbyssGateQuestID).Vars, 0) != 3 || s.countItems(p, sealingAbyssGateItemID) != 1 {
		t.Fatalf("seal stage = %+v, key count=%d", p.quest(sealingAbyssGateQuestID), s.countItems(p, sealingAbyssGateItemID))
	}
	exit := sealingAbyssGateTestObject(s, p, sealingAbyssGateEntryID, 0x32023)
	p.targetID = exit.id
	if !c.sealingAbyssGateDialog(exit, script, -1) {
		t.Fatal("entry gate did not start the return use")
	}
	time.Sleep(3200 * time.Millisecond)
	if p.WorldID != 210030000 || p.instance != 0 || p.quest(sealingAbyssGateQuestID).Status != "REWARD" || s.countItems(p, sealingAbyssGateItemID) != 0 {
		t.Fatalf("instance exit failed: world=%d instance=%d quest=%+v key=%d", p.WorldID, p.instance, p.quest(sealingAbyssGateQuestID), s.countItems(p, sealingAbyssGateItemID))
	}
}

func TestSealingAbyssGateDeathAndWorldRecovery(t *testing.T) {
	for _, stage := range []int32{2, 3} {
		t.Run(string(rune('0'+stage)), func(t *testing.T) {
			_, p, c, _ := sealingAbyssGateTestFixture(t, "START", stage)
			if !c.s.addItem(p, sealingAbyssGateItemID, 1) {
				t.Fatal("could not add quest key")
			}
			if !c.sealingAbyssGateDeath() || questVar(p.quest(sealingAbyssGateQuestID).Vars, 0) != 1 || c.s.countItems(p, sealingAbyssGateItemID) != 0 {
				t.Fatalf("death did not reset the active expedition: quest=%+v key=%d", p.quest(sealingAbyssGateQuestID), c.s.countItems(p, sealingAbyssGateItemID))
			}
		})
	}
	_, p, c, _ := sealingAbyssGateTestFixture(t, "START", 3)
	if !c.s.addItem(p, sealingAbyssGateItemID, 1) {
		t.Fatal("could not add quest key")
	}
	p.WorldID = 210030000
	if !c.sealingAbyssGateEnterWorld() || questVar(p.quest(sealingAbyssGateQuestID).Vars, 0) != 1 || c.s.countItems(p, sealingAbyssGateItemID) != 0 {
		t.Fatalf("world recovery did not reset the quest: quest=%+v key=%d", p.quest(sealingAbyssGateQuestID), c.s.countItems(p, sealingAbyssGateItemID))
	}
}

func TestSealingAbyssGateEnterWorldUnlockAndRewards(t *testing.T) {
	s, p, c, _ := sealingAbyssGateTestFixture(t, "LOCKED", 0)
	p.level = 15
	if c.sealingAbyssGateEnterWorld() {
		t.Fatal("level 15 enter-world event should not use Java's strict greater-than-15 gate")
	}
	p.level = 16
	p.quest(1011).Status = "START"
	if c.sealingAbyssGateEnterWorld() {
		t.Fatal("enter-world event unlocked the quest with a missing prerequisite")
	}
	p.quest(1011).Status = "COMPLETE"
	if !c.sealingAbyssGateEnterWorld() || p.quest(sealingAbyssGateQuestID).Status != "START" {
		t.Fatalf("enter-world unlock failed: %+v", p.quest(sealingAbyssGateQuestID))
	}
	template := s.data.Quests[sealingAbyssGateQuestID]
	if template == nil || len(template.Rewards) != 1 || len(template.Rewards[0].SelectableItems) != 2 {
		t.Fatal("Sealing the Abyss Gate selectable rewards are missing from quest metadata")
	}
	for index, reward := range template.Rewards[0].SelectableItems {
		t.Run(string(rune('A'+index)), func(t *testing.T) {
			testServer := testServer(s.data)
			player := wrathchild(testServer)
			player.Race, player.Class = "ELYOS", "SORCERER"
			player.level = template.MinLevel
			player.Exp = s.data.ExpStart(player.level)
			player.cube = nil
			player.seen = map[int32]*object{}
			player.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Owner: player.ID}
			player.quests = []store.Quest{{ID: sealingAbyssGateQuestID, Status: "REWARD", Vars: 3}}
			packets := &questPackets{}
			connection := &conn{s: testServer, player: player, tap: packets.tap}
			player.conn = connection
			testServer.spawned[player.ID] = player
			end := sealingAbyssGateTestObject(testServer, player, sealingAbyssGatePernosID, int32(0x33020+index))
			testScript := &data.QuestScript{ID: sealingAbyssGateQuestID, Kind: data.QuestCustom, EndNPC: sealingAbyssGatePernosID}
			if !connection.sealingAbyssGateShowDialog(end, testScript) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 1352, sealingAbyssGateQuestID).Data) {
				t.Fatal("reward preview did not show the Java reward page")
			}
			beforeExp := player.Exp
			connection.sealingAbyssGateDialog(end, testScript, int32(8+index))
			if q := player.quest(sealingAbyssGateQuestID); q.Status != "COMPLETE" || q.CompleteCount != 1 || player.Exp-beforeExp != template.Rewards[0].Experience || testServer.countItems(player, reward.ID) != reward.Count {
				t.Fatalf("reward %d mismatch: quest=%+v exp=%d item=%d", index, q, player.Exp-beforeExp, testServer.countItems(player, reward.ID))
			}
			connection.sealingAbyssGateDialog(end, testScript, int32(8+index))
			if player.quest(sealingAbyssGateQuestID).CompleteCount != 1 || testServer.countItems(player, reward.ID) != reward.Count {
				t.Fatal("repeated reward dialog duplicated the reward")
			}
		})
	}
}
