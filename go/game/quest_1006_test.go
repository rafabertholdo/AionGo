package game

import (
	"bytes"
	"testing"
	"testing/synctest"
	"time"

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

// The website's preset stops before the trial; Pernos creates the instance.
func TestAscensionPanelPresetEntersTrial(t *testing.T) {
	d := staticDataOrSkip(t)
	for _, class := range []string{"WARRIOR", "SCOUT", "MAGE", "PRIEST"} {
		t.Run(class, func(t *testing.T) {
			s := testServer(d)
			t.Cleanup(func() {
				s.visMu.Lock()
				defer s.visMu.Unlock()
				for _, instance := range s.instances {
					instance.check.cancel()
				}
			})
			saver := &recordedQuests{}
			s.quests = saver
			p := wrathchild(s)
			p.Race, p.Class, p.level, p.Exp = "ELYOS", class, 9, 140329
			p.WorldID, p.instance = 210010000, 1
			p.X, p.Y, p.Z = 242, 1638, 100
			p.quests = []store.Quest{{ID: ascensionQuestID, Status: "START", Vars: 3}}
			p.seen = map[int32]*object{}
			p.cube = []*store.Item{}
			p.spawned = true
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			s.spawned[p.ID] = p
			npc := questCatalogNPC(s, p, ascensionStartNPC, 0x31006)
			npc.npc = d.Npcs[ascensionStartNPC]
			s.initNpc(npc)
			c.ascensionEnterWorld()
			c.dialogSelect(dialogRequest(cmDialogSelect, npc.id, 25, ascensionQuestID))
			if got := packets.last(smDialogWindow); !bytes.Equal(got, dialogWindow(npc.id, 1693, ascensionQuestID).Data) {
				t.Fatalf("trial entry dialog = %x", got)
			}
			c.dialogSelect(dialogRequest(cmDialogSelect, npc.id, 10002, ascensionQuestID))
			if p.WorldID != ascensionInstanceMap || p.X != 52 || p.Y != 174 || p.Z != 229 {
				t.Fatalf("trial entry: world=%d instance=%d position=%f,%f,%f", p.WorldID, p.instance, p.X, p.Y, p.Z)
			}
			if p.Class != class || p.quest(ascensionQuestID).Status != "START" || p.quest(ascensionQuestID).Vars != 99 {
				t.Fatalf("trial state: class=%s quest=%+v", p.Class, p.quest(ascensionQuestID))
			}
			instance := s.instances[[2]int32{p.WorldID, p.instance}]
			if instance == nil || !instance.registered[p.ID] {
				t.Fatal("character not registered in its trial instance")
			}
			if packets.last(smPlayerSpawn) == nil {
				t.Fatal("missing instance map-load packet")
			}
			c.ascensionEnterWorld()
			if p.quest(ascensionQuestID).Vars != 99 || packets.last(smAscensionMorph) == nil {
				t.Fatal("trial entry state did not survive the instance map load")
			}
			transport := questCatalogNPC(s, p, ascensionTransportNPC, 0x31007)
			s.initNpc(transport)
			if !c.ascensionDialog(transport, d.QuestScripts[ascensionQuestID], 25) || p.quest(ascensionQuestID).Vars != 50 {
				t.Fatalf("trial transport did not start: %+v", p.quest(ascensionQuestID))
			}
		})
	}
}

func TestAscensionFourMinionsDealOneDamage(t *testing.T) {
	d := staticDataOrSkip(t)
	template := d.Npcs[ascensionMinionNPC]
	if template == nil {
		t.Fatal("missing trial minion template")
	}
	originalPower := template.Stats.Power
	for _, class := range []string{"WARRIOR", "SCOUT", "MAGE", "PRIEST"} {
		t.Run(class, func(t *testing.T) {
			s := testServer(d)
			p := wrathchild(s)
			p.Class, p.Race, p.level = class, "ELYOS", 9
			p.WorldID, p.instance = ascensionInstanceMap, 77
			p.stats = s.playerStats(p)
			p.stats.set(data.Evasion, 0, false)
			p.stats.set(data.Parry, 0, false)
			p.stats.set(data.Block, 0, false)
			p.equipment = []*store.Item{}
			packets := &questPackets{}
			c := &conn{s: s, player: p, tap: packets.tap}
			p.conn = c
			for minionIndex := range 4 {
				minion := c.spawnAscensionNPC(ascensionMinionNPC, ascensionInstanceMap, p.instance, 224, 239, 206, 0, true)
				if minion == nil {
					t.Fatal("failed to spawn trial minion")
				}
				minion.stats.recompute(false)
				for range 50 {
					if damage := s.physicalDamage(minion, p, 0); damage != 1 {
						t.Fatalf("minion %d damage = %d", minionIndex, damage)
					}
					results := s.physicalAttack(minion, p)
					// Dodges and parries can prevent the single point of damage.
					if len(results) != 1 || results[0].damage < 0 || results[0].damage > 1 || results[0].status == statusNormalHit && results[0].damage != 1 {
						t.Fatalf("minion %d attacks: %+v", minionIndex, results)
					}
				}
			}
			if template.Stats.Power != originalPower {
				t.Fatal("trial changed the shared NPC template")
			}
			normal := &object{npc: template}
			s.initNpc(normal)
			if normal.stats.current(data.MainHandPower) != originalPower {
				t.Fatal("normal NPC attack power changed")
			}
		})
	}
}

func TestAscensionBottleInOverlappingLakeZone(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		s.quests = &recordedQuests{}
		p, _ := fighter(t, s, 640)
		p.Y, p.Z = 1060, 99
		p.quests = []store.Quest{{ID: ascensionQuestID, Status: "START", Vars: 1}}
		for _, z := range d.Zones[p.WorldID] {
			if z.Name == "CLIONA_LAKE" {
				p.zone = z
			}
		}
		if p.zone == nil {
			t.Fatal("missing lake zone")
		}
		item := &store.Item{UniqueID: 0x40000, ItemID: ascensionJournalItem, Owner: p.ID, Count: 1}
		p.cube = []*store.Item{item}
		if !p.conn.ascensionItemUse(item) {
			t.Fatal("bottle use rejected inside overlapping item area")
		}
		time.Sleep(3 * time.Second)
		synctest.Wait()
		if p.quest(ascensionQuestID).Vars != 2 || s.countItems(p, ascensionJournalItem) != 0 || s.countItems(p, ascensionProofItem) != 1 {
			t.Fatal("bottle did not fill and advance Ascension")
		}
	})
}

func TestAscensionBottleRejectsOutsideAndLeavingArea(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, _ := fighter(t, s, 600)
		p.Y, p.Z = 1060, 99
		p.quests = []store.Quest{{ID: ascensionQuestID, Status: "START", Vars: 1}}
		item := &store.Item{UniqueID: 0x40000, ItemID: ascensionJournalItem, Owner: p.ID, Count: 1}
		p.cube = []*store.Item{item}
		if p.conn.ascensionItemUse(item) {
			t.Fatal("bottle accepted outside the filling area")
		}
		p.X = 640
		if !p.conn.ascensionItemUse(item) {
			t.Fatal("bottle rejected inside the filling area")
		}
		p.X = 600
		time.Sleep(3 * time.Second)
		synctest.Wait()
		if p.cubeItem(item.UniqueID) != item || p.quest(ascensionQuestID).Vars != 1 || s.countItems(p, ascensionProofItem) != 0 {
			t.Fatal("leaving the area still filled the bottle")
		}
	})
}
