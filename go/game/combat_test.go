package game

import (
	"sync"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// tapped counts the packets a test player is sent, by opcode.
type tapped struct {
	mu     sync.Mutex
	counts map[byte]int
}

func (t *tapped) tap(w *wire.Writer) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts[w.Data[0]]++
}

func (t *tapped) count(op byte) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.counts[op]
}

// fighter is a level 1 elyos mage with a staff, standing at x, in a test server, whose packets are counted.
func fighter(t *testing.T, s *Server, x float32) (*player, *tapped) {
	t.Helper()
	p := wrathchild(s)
	p.ID = 0x20000
	p.X, p.Y, p.Z, p.WorldID = x, 1000, 130, 210010000
	p.known = map[int32]*player{}
	p.seen = map[int32]*object{}
	p.attackCounter = 1
	p.cube = nil
	p.fx = newEffectController(p)
	p.stats = s.playerStats(p)
	p.life.HP, p.life.MP = p.stats.current(data.MaxHP), p.stats.current(data.MaxMP)
	p.kinah = &store.Item{UniqueID: 0x10578, ItemID: data.Kinah, Count: 0, Owner: p.ID}
	tap := &tapped{counts: map[byte]int{}}
	p.conn = &conn{s: s, tap: tap.tap, account: &account{}}
	p.conn.player = p
	return p, tap
}

// monster puts a striped kerub (a level 1 monster) at x.
func monster(t *testing.T, s *Server, x float32) *object {
	t.Helper()
	template := s.data.Npcs[210133]
	if template == nil {
		t.Skip("npc 210133 isn't in the static data")
	}
	o := &object{id: 0x30000, worldID: 210010000, x: x, y: 1000, z: 130, npc: template, homeX: x, homeY: 1000, homeZ: 130, interval: 5}
	s.initNpc(o)
	s.byID[o.id] = o
	s.addObject(o)
	o.ai.handleEvent(evRespawned)
	return o
}

func TestFightAMonster(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	s.startWorldTasks()
	s.drops = map[int32][]store.Drop{210133: {{ItemID: data.Kinah, Min: 5, Max: 5, Chance: 100}, {ItemID: 160010002, Min: 2, Max: 2, Chance: 100}}}
	p, tap := fighter(t, s, 1000)
	s.spawn(p)
	o := monster(t, s, 1005)
	s.visMu.Lock()
	s.updateNpcKnown(o)
	seen := p.seen[o.id] != nil
	s.visMu.Unlock()
	if !seen {
		t.Fatal("the player doesn't see the monster")
	}
	if tap.count(smNpcInfo) != 1 {
		t.Fatalf("the player got %d npc infos", tap.count(smNpcInfo))
	}

	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		s.visMu.Lock()
		done := o.dead
		p.lastAttack = time.Time{}
		p.life.HP = max(p.life.HP, p.stats.current(data.MaxHP)/2) // the fight is about the monster's end, not the player's
		if !done && !p.dead {
			s.playerAttack(p, o)
		}
		gone := p.dead
		s.visMu.Unlock()
		if done || gone {
			break
		}
		time.Sleep(600 * time.Millisecond)
	}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if !o.dead {
		t.Fatalf("the monster lives: %d/%d, the player %d/%d", o.hp, o.maxHP, p.life.HP, p.stats.current(data.MaxHP))
	}
	if tap.count(smAttack) < 2 {
		t.Errorf("only %d attacks were seen: the monster should have fought back too", tap.count(smAttack))
	}
	if p.Exp == 206 {
		t.Errorf("the player got no experience")
	}
	if tap.count(smStatupdateExp) == 0 && tap.count(smLevelUpdate) == 0 {
		t.Errorf("the player wasn't told of its experience")
	}
	if tap.count(smEmotion) == 0 {
		t.Errorf("nobody died in view")
	}
	if o.loot == nil || len(o.loot.items) != 2 || tap.count(smLootStatus) == 0 {
		t.Fatalf("no loot was left: %+v", o.loot)
	}

	// Loot it: open the list, take both items, and the corpse goes.
	s.openLoot(p, o.id)
	if o.loot.looting != p.ID || p.state&stateLooting == 0 {
		t.Errorf("the player isn't looting")
	}
	s.takeLoot(p, o.id, 1)
	s.takeLoot(p, o.id, 2)
	if p.kinah.Count != 5 {
		t.Errorf("kinah: %d, want 5", p.kinah.Count)
	}
	if len(p.cube) != 1 || p.cube[0].ItemID != 160010002 || p.cube[0].Count != 2 {
		t.Errorf("cube: %+v", p.cube)
	}
	if p.state&stateLooting != 0 || p.state&stateActive == 0 {
		t.Errorf("the player still loots: state %#x", p.state)
	}
	if tap.count(smAddItems) != 1 || tap.count(smUpdateItem) == 0 {
		t.Errorf("items announced: %d added, %d updated", tap.count(smAddItems), tap.count(smUpdateItem))
	}
}
