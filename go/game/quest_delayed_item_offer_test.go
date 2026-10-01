package game

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestDelayedItemQuestStarterConversation(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, script := range []*data.QuestScript{
		{ID: 1309, Kind: data.QuestCustom, StartNPC: 203932, EndNPC: 203830, ItemID: 182201304, ItemUseDelay: 3000},
		{ID: 1323, Kind: data.QuestCustom, StartNPC: 730019, EndNPC: 203939, ItemID: 182201309, ItemUseDelay: 3000},
		{ID: 2274, Kind: data.QuestCustom, StartNPC: 203668, EndNPC: 203560, ItemID: 182203249, ItemUseDelay: 3000},
	} {
		t.Run(fmt.Sprintf("quest_%d", script.ID), func(t *testing.T) {
			s := testServer(d)
			recorded := &recordedQuests{}
			s.quests = recorded
			p := wrathchild(s)
			p.quests = []store.Quest{{ID: script.ID, Status: "START", Vars: setQuestVar(0, 0, 2)}}
			p.seen = map[int32]*object{}
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			start := questCatalogNPC(s, p, script.StartNPC, 0x40000+script.ID)
			end := questCatalogNPC(s, p, script.EndNPC, 0x41000+script.ID)

			if !c.delayedItemQuestNPCDialog(start, script, ^uint16(0)) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1352, script.ID).Data) {
				t.Fatal("active starter click did not open page 1352")
			}
			if !c.delayedItemQuestNPCDialog(end, script, ^uint16(0)) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 2375, script.ID).Data) {
				t.Fatal("active end NPC click did not open page 2375")
			}
			p.quest(script.ID).Status = "REWARD"
			if !c.delayedItemQuestNPCDialog(end, script, ^uint16(0)) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(end.id, 5, script.ID).Data) {
				t.Fatal("reward NPC click did not open page 5")
			}
			p.quest(script.ID).Status = "START"

			if !c.delayedItemQuestNPCDialog(start, script, 25) || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 1352, script.ID).Data) {
				t.Fatal("starter conversation did not show page 1352")
			}
			if !c.delayedItemQuestNPCDialog(start, script, 10000) || questVar(p.quest(script.ID).Vars, 0) != 3 || len(recorded.saved) != 1 || questVar(recorded.saved[0].Vars, 0) != 3 || !bytes.Equal(packets.last(smDialogWindow), dialogWindow(start.id, 10, 0).Data) {
				t.Fatalf("starter report did not increment quest var and close dialog: quest=%+v", p.quest(script.ID))
			}
			if c.delayedItemQuestNPCDialog(start, script, 10001) {
				t.Fatal("unexpected starter dialog was handled")
			}
			wrongNPC := questCatalogNPC(s, p, script.StartNPC+1, 0x50000+script.ID)
			if c.delayedItemQuestNPCDialog(wrongNPC, script, 25) {
				t.Fatal("unregistered NPC was handled")
			}

			failedServer := testServer(d)
			failedServer.quests = &recordedQuests{err: errors.New("storage unavailable")}
			failedPlayer := wrathchild(failedServer)
			failedPlayer.quests = []store.Quest{{ID: script.ID, Status: "START", Vars: setQuestVar(0, 0, 4)}}
			failedPlayer.seen = map[int32]*object{}
			failedPackets := &questPackets{}
			failedConn := &conn{s: failedServer, player: failedPlayer, tap: failedPackets.tap}
			failedStart := questCatalogNPC(failedServer, failedPlayer, script.StartNPC, 0x60000+script.ID)
			if failedConn.delayedItemQuestNPCDialog(failedStart, script, 10000) || questVar(failedPlayer.quest(script.ID).Vars, 0) != 4 || failedPackets.last(smDialogWindow) != nil {
				t.Fatal("failed persistence changed quest state or sent a dialog")
			}
		})
	}
}
