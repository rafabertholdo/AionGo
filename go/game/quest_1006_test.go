package game

import (
	"bytes"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

func TestAscensionLevelUpOfferAndStart(t *testing.T) {
	d := staticDataOrSkip(t)
	script := d.QuestScripts[ascensionQuestID]
	if script == nil || script.Kind != data.QuestCustom || script.StartNPC != ascensionStartNPC || !script.LevelUpStart ||
		script.ItemID != ascensionJournalItem || len(script.MonsterInfos) != 1 || script.MonsterInfos[0].NPCID != ascensionMinionNPC || script.MonsterInfos[0].MaxKill != 54 {
		t.Fatalf("unexpected Ascension registration: %+v", script)
	}

	s := testServer(d)
	saver := &recordedQuests{}
	s.quests = saver
	p := wrathchild(s)
	p.Race, p.Class, p.level = "ELYOS", "MAGE", 9
	p.seen = map[int32]*object{}
	p.cube = []*store.Item{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p

	c.levelUpStartQuests()
	if q := p.quest(ascensionQuestID); q == nil || q.Status != "START" || len(saver.saved) != 1 {
		t.Fatalf("level-up did not start Ascension: quest=%+v saves=%+v", q, saver.saved)
	}
	if got := packets.last(smQuestAccepted); len(got) == 0 || got[0] != smQuestAccepted {
		t.Fatalf("missing quest accepted packet: %x", got)
	}

	npc := questCatalogNPC(s, p, ascensionStartNPC, 0x31006)
	npc.npc = d.Npcs[ascensionStartNPC]
	if npc.npc == nil {
		t.Fatal("High Priest's NPC template is missing")
	}
	c.dialogSelect(dialogRequest(cmDialogSelect, npc.id, 25, ascensionQuestID))
	if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1011, ascensionQuestID).Data) {
		t.Fatalf("Ascension offer page = %x", got)
	}
	c.dialogSelect(dialogRequest(cmDialogSelect, npc.id, 10000, ascensionQuestID))
	if q := p.quest(ascensionQuestID); q == nil || questVar(q.Vars, 0) != 1 || s.countItems(p, ascensionJournalItem) != 1 {
		t.Fatalf("accepting Ascension did not grant the journal: quest=%+v journal=%d", q, s.countItems(p, ascensionJournalItem))
	}
}

func TestAscensionTrialKillBossAndRecovery(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	saver := &recordedQuests{}
	s.quests = saver
	p := wrathchild(s)
	p.WorldID, p.instance = ascensionInstanceMap, 77
	p.quests = []store.Quest{{ID: ascensionQuestID, Status: "START", Vars: setQuestVar(0, 0, 51)}}
	p.seen = map[int32]*object{}
	packets := &questPackets{}
	c := &conn{s: s, player: p, tap: packets.tap}
	p.conn = c
	s.spawned[p.ID] = p

	minion := &object{npc: d.Npcs[ascensionMinionNPC]}
	if minion.npc == nil {
		t.Fatal("Ascension minion template is missing")
	}
	for range 4 {
		c.ascensionKill(minion)
	}
	if q := p.quest(ascensionQuestID); questVar(q.Vars, 0) != 4 {
		t.Fatalf("four minion kills left quest at %d", questVar(q.Vars, 0))
	}
	var boss *object
	for _, o := range s.byID {
		if o.npc != nil && o.npc.ID == ascensionBossNPC && o.instance == p.instance {
			boss = o
			break
		}
	}
	if boss == nil {
		t.Fatal("fourth minion did not spawn the trial boss")
	}
	boss.hp, boss.maxHP = 100, 100
	if got := c.ascensionBossDamage(boss, 80); got != 51 {
		t.Fatalf("boss threshold damage = %d, want 51", got)
	}
	boss.hp = 49
	c.ascensionAttack(boss)
	if packets.last(smPlayMovie) == nil {
		t.Fatal("boss threshold did not play the end movie and despawn the boss")
	}
	for _, o := range s.grid[cellAt(boss.worldID, boss.instance, boss.x, boss.y)] {
		if o == boss {
			t.Fatal("boss remained in the world after the threshold movie")
		}
	}
	if !c.ascensionMovieEnd(151) {
		t.Fatal("Ascension end movie did not advance the quest")
	}
	if q := p.quest(ascensionQuestID); questVar(q.Vars, 0) != 5 {
		t.Fatalf("movie end left quest at %d", questVar(q.Vars, 0))
	}

	if !c.customQuestProgress(ascensionQuestID, setQuestVar(p.quest(ascensionQuestID).Vars, 0, 4), "") {
		t.Fatal("could not set quest up for death recovery")
	}
	c.ascensionDeath()
	if q := p.quest(ascensionQuestID); questVar(q.Vars, 0) != 3 {
		t.Fatalf("death did not recover the player to step 3: %+v", q)
	}
}
