package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestAtroposRequestZoneStartTurnInAndFollowUps(t *testing.T) {
	d := staticDataOrSkip(t)
	script, template := d.QuestScripts[atroposRequestQuestID], d.Quests[atroposRequestQuestID]
	if script == nil || template == nil || script.Kind != data.QuestCustom || script.LevelUpStart || script.EndNPC != atroposRequestNPCID ||
		template.Race != "ELYOS" || template.MinLevel != 45 || template.NameID != 2204261 || len(template.Rewards) != 1 || template.Rewards[0].Experience != 100000 {
		t.Fatalf("unexpected A Request From Atropos metadata: script=%+v template=%+v", script, template)
	}
	zoneFound := false
	for _, zone := range d.Zones[atroposRequestMapID] {
		zoneFound = zoneFound || zone.Name == atroposRequestZone
	}
	if !zoneFound {
		t.Fatalf("quest start zone %s is missing from map %d", atroposRequestZone, atroposRequestMapID)
	}
	if len(d.QuestEnds[atroposRequestNPCID]) == 0 {
		t.Fatalf("quest end index is missing Atropos NPC %d", atroposRequestNPCID)
	}

	s := testServer(d)
	s.quests = &recordedQuests{}
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "CLERIC", 44
	p.appearance = &store.Appearance{}
	p.Exp = d.ExpStart(p.level)
	p.cube, p.seen = []*store.Item{}, map[int32]*object{}
	p.spawned = true
	p.WorldID = atroposRequestMapID
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p
	if c.atroposRequestEnterZone(atroposRequestZone) || p.quest(atroposRequestQuestID) != nil {
		t.Fatal("zone entry started the quest below level 45")
	}
	p.level = 45
	p.Race = "ASMODIANS"
	if c.atroposRequestEnterZone(atroposRequestZone) || p.quest(atroposRequestQuestID) != nil {
		t.Fatal("zone entry started the Elyos quest for an Asmodian")
	}
	p.Race = "ELYOS"
	if c.atroposRequestEnterZone("OTHER_ZONE") || c.atroposRequestEnterZone(atroposRequestZone) != true {
		t.Fatalf("eligible Q1091 entry failed to start the quest: %+v", p.quest(atroposRequestQuestID))
	}
	quest := p.quest(atroposRequestQuestID)
	if quest == nil || quest.Status != "START" || quest.Vars != 0 || !bytes.Equal(packets.last(smQuestAccepted), questAccepted(1, *quest).Data) {
		t.Fatalf("zone entry created unexpected quest state: %+v", quest)
	}
	if c.atroposRequestEnterZone(atroposRequestZone) {
		t.Fatal("re-entering Q1091 restarted the existing quest")
	}

	npc := questCatalogNPC(s, p, atroposRequestNPCID, 0x31191)
	npc.npc = d.Npcs[atroposRequestNPCID]
	if npc.npc == nil {
		t.Fatalf("Atropos NPC template %d is missing", atroposRequestNPCID)
	}
	s.initNpc(npc)
	selectDialog := func(dialogID int32) bool {
		return c.atroposRequestDialog(npc, script, dialogID)
	}
	if !selectDialog(25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 10002, atroposRequestQuestID).Data) {
		t.Fatalf("Atropos report page = %x", packets.last(smDialogWindow))
	}
	if !selectDialog(1009) || quest.Status != "REWARD" || quest.Vars != 1 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 5, atroposRequestQuestID).Data) {
		t.Fatalf("Atropos did not move the quest to reward: quest=%+v dialog=%x", quest, packets.last(smDialogWindow))
	}
	if !selectDialog(-1) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(npc.id, 5, atroposRequestQuestID).Data) {
		t.Fatalf("reward-state click did not show the reward menu: %x", packets.last(smDialogWindow))
	}
	beforeExperience := p.Exp
	if !selectDialog(17) || p.quest(atroposRequestQuestID).Status != "COMPLETE" || p.quest(atroposRequestQuestID).CompleteCount != 1 || p.Exp-beforeExperience != 100000 {
		t.Fatalf("A Request From Atropos did not finish: quest=%+v XP gained=%d", p.quest(atroposRequestQuestID), p.Exp-beforeExperience)
	}
	for _, childQuestID := range []int32{1092, 1093, 1094} {
		child := p.quest(childQuestID)
		if child == nil || child.Status != "LOCKED" {
			t.Fatalf("follow-up quest %d was not unlocked as LOCKED: %+v", childQuestID, child)
		}
	}
}
