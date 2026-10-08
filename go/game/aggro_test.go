package game

import (
	"testing"
	"time"

	"aionlightning/game/data"
)

// aggressiveMonster finds a level 1..5 monster of the static data that attacks elyos on sight, and puts it at x.
func aggressiveMonster(t *testing.T, s *Server, x float32) *object {
	t.Helper()
	return placeMonster(s, aggressiveTemplate(t, s), x)
}

func aggressiveTemplate(t *testing.T, s *Server) *data.NpcTemplate {
	t.Helper()
	var template *data.NpcTemplate
	for _, n := range s.data.Npcs {
		if n.Type == "ATTACKABLE" && n.Level >= 1 && n.Level <= 5 && n.SRange >= 5 && s.data.Tribes.AggroIcon(false, n.Tribe) && (template == nil || n.ID < template.ID) {
			template = n
		}
	}
	if template == nil {
		t.Skip("no aggressive npc in the static data")
	}
	return template
}

func placeMonster(s *Server, template *data.NpcTemplate, x float32) *object {
	o := &object{id: s.ids.nextID(), worldID: 210010000, x: x, y: 1000, z: 130, npc: template, homeX: x, homeY: 1000, homeZ: 130, interval: 5}
	s.initNpc(o)
	s.byID[o.id] = o
	s.addObject(o)
	o.ai.handleEvent(evRespawned)
	return o
}

// movePlayer is what CM_MOVE does to the world.
func movePlayer(s *Server, p *player, x float32) {
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.updatePosition(p, x, p.Y, p.Z, 0)
}

func hates(s *Server, o *object, p *player) bool {
	s.visMu.Lock()
	defer s.visMu.Unlock()
	info := o.aggro[p.ID]
	return info != nil && info.hate > 0
}

func waitHate(s *Server, o *object, p *player, within time.Duration) bool {
	for end := time.Now().Add(within); time.Now().Before(end); time.Sleep(100 * time.Millisecond) {
		if hates(s, o, p) {
			return true
		}
	}
	return false
}

func TestAggroOnApproach(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p, _ := fighter(t, s, 1000)
	s.spawn(p)
	o := aggressiveMonster(t, s, 1000+float32(80))
	if p.seen[o.id] == nil {
		t.Fatal("the player doesn't see the monster")
	}
	if waitHate(s, o, p, 4*time.Second) {
		t.Fatalf("aggro from %d m away (range %d)", 80, o.npc.SRange)
	}
	movePlayer(s, p, 1000+float32(80)-float32(o.npc.SRange)/2)
	if !waitHate(s, o, p, 6*time.Second) {
		t.Fatalf("npc %d (range %d) didn't aggro on approach; ai state %d, %d desires", o.npc.ID, o.npc.SRange, o.ai.state, len(o.ai.desires))
	}
}

func TestAggroWhenEnteringTheWorld(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p, _ := fighter(t, s, 1000)
	o := aggressiveMonster(t, s, 1002)
	s.spawn(p)
	if !waitHate(s, o, p, 6*time.Second) {
		t.Fatal("no aggro on entering the world next to it")
	}
}

func TestNoAggroWhileBlinking(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p, _ := fighter(t, s, 1000)
	o := aggressiveMonster(t, s, 1002)
	s.spawn(p)
	s.visMu.Lock()
	s.startProtection(p)
	s.visMu.Unlock()
	if waitHate(s, o, p, 5*time.Second) {
		t.Fatal("aggro on a blinking player")
	}
	s.visMu.Lock()
	s.playerHit(p, o, 0, 0, 10)
	hp := p.life.HP
	s.stopProtection(p)
	s.visMu.Unlock()
	if hp != p.stats.current(data.MaxHP) {
		t.Fatal("a blinking player took damage")
	}
	if !waitHate(s, o, p, 6*time.Second) {
		t.Fatal("no aggro once the blinking stopped")
	}
}

func TestNoAggroOutOfRange(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p, _ := fighter(t, s, 1000)
	s.spawn(p)
	o := aggressiveMonster(t, s, 1000+float32(aggressiveTemplate(t, s).SRange)+10)
	if waitHate(s, o, p, 5*time.Second) {
		t.Fatal("aggro out of range")
	}
}

func TestNoAggroTenLevelsAbove(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p, _ := fighter(t, s, 1000)
	s.spawn(p)
	o := aggressiveMonster(t, s, 1002)
	p.level = int(o.npc.Level) + 10
	s.visMu.Lock()
	o.ai.handleEvent(evSeePlayer)
	s.visMu.Unlock()
	if waitHate(s, o, p, 5*time.Second) {
		t.Fatal("aggro on a player ten levels above")
	}
}

func TestNoAggroFromPassiveNpc(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p, _ := fighter(t, s, 1000)
	s.spawn(p)
	o := monster(t, s, 1002) // the striped kerub only fights back
	if s.isAggressive(o) {
		t.Skip("the test npc is aggressive")
	}
	if waitHate(s, o, p, 5*time.Second) {
		t.Fatal("a passive npc aggroed")
	}
}

// A monster that wanders around is already busy (it walks) when the player comes: it must still notice it.
func TestAggroOfAWanderingMonster(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p, _ := fighter(t, s, 1000)
	template := aggressiveTemplate(t, s)
	s.visMu.Lock()
	o := &object{id: 0x30000, worldID: 210010000, x: 1002, y: 1000, z: 130, npc: template, homeX: 1002, homeY: 1000, homeZ: 130, interval: 5, randomWalk: 5}
	s.initNpc(o)
	s.byID[o.id] = o
	s.addObject(o)
	o.ai.handleEvent(evRespawned)
	walking := o.ai.scheduled()
	s.visMu.Unlock()
	if !walking {
		t.Fatal("the monster doesn't wander")
	}
	s.spawn(p)
	if !waitHate(s, o, p, 6*time.Second) {
		t.Fatal("a wandering monster never noticed the player")
	}
}

// supportPair finds two attackable npcs whose tribes are linked: the first one's tribe helps the second's.
func supportPair(t *testing.T, s *Server) (helper, helped *data.NpcTemplate) {
	t.Helper()
	byTribe := map[string]*data.NpcTemplate{}
	for _, n := range s.data.Npcs {
		if n.Type == "ATTACKABLE" && n.Level >= 1 && n.Level <= 5 && (byTribe[n.Tribe] == nil || n.ID < byTribe[n.Tribe].ID) {
			byTribe[n.Tribe] = n
		}
	}
	for name, tribe := range s.data.Tribes {
		for _, other := range tribe.Support {
			if name != other && byTribe[name] != nil && byTribe[other] != nil && !s.data.Tribes.IsFriendly(name, "PC") {
				return byTribe[name], byTribe[other]
			}
		}
	}
	t.Skip("no supporting tribes in the static data")
	return nil, nil
}

func TestLinkedNpcsHelp(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p, _ := fighter(t, s, 1000)
	s.spawn(p)
	helper, helped := supportPair(t, s)
	s.visMu.Lock()
	victim := placeMonster(s, helped, 1010)
	friend := placeMonster(s, helper, 1005)
	s.npcHit(victim, p, 0, statusRegular, 1)
	got := friend.aggro[p.ID]
	s.visMu.Unlock()
	if got == nil || got.hate < 10 {
		t.Fatalf("npc %d (tribe %s) didn't help npc %d (tribe %s): %+v", helper.ID, helper.Tribe, helped.ID, helped.Tribe, got)
	}
}

// The player runs away from a monster that has left home: it goes back, and heals.
func TestMonsterGoesHomeAndHeals(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	p, _ := fighter(t, s, 1000)
	s.spawn(p)
	o := aggressiveMonster(t, s, 1002)
	if !waitHate(s, o, p, 6*time.Second) {
		t.Fatal("no aggro")
	}
	s.visMu.Lock()
	s.moveNpcTo(o, o.homeX+12, o.homeY, o.homeZ, 0, false)
	o.hp = o.maxHP / 2
	s.visMu.Unlock()
	movePlayer(s, p, 1000+3*visibilityDistance)
	for end := time.Now().Add(30 * time.Second); time.Now().Before(end); time.Sleep(200 * time.Millisecond) {
		s.visMu.Lock()
		done := o.atHome() && o.hp == o.maxHP
		s.visMu.Unlock()
		if done {
			return
		}
	}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	t.Fatalf("the monster is at %.1f,%.1f (home %.1f) with %d/%d life, ai state %d", o.x, o.y, o.homeX, o.hp, o.maxHP, o.ai.state)
}
