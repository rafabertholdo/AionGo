package game

import (
	"encoding/binary"
	"testing"
	"testing/synctest"
	"time"

	"aionlightning/wire"
)

func TestNochsanaArtifactAppliesShieldOnlyInItsInstance(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, _ := fighter(t, s, 1000)
		other, _ := fighter(t, s, 1001)
		other.ID = p.ID + 1
		s.spawn(p)
		s.visMu.Lock()
		in := s.newInstance(nochsanaWorld)
		otherRun := s.newInstance(nochsanaWorld)
		defer func() {
			s.visMu.Lock()
			s.destroyInstance(in)
			s.destroyInstance(otherRun)
			s.visMu.Unlock()
		}()
		var artifact *object
		for _, o := range s.byID {
			if o.worldID == in.world && o.instance == in.id && o.npc != nil && o.npc.ID == 700437 {
				artifact = o
				break
			}
		}
		if artifact == nil {
			t.Fatal("Nochsana artifact is not spawned")
		}
		s.changePosition(p, in.world, in.id, artifact.x, artifact.y, artifact.z, 0)
		s.spawnLocked(p)
		other.WorldID, other.instance = otherRun.world, otherRun.id
		other.X, other.Y, other.Z = artifact.x, artifact.y, artifact.z
		s.spawnLocked(other)
		s.visMu.Unlock()
		if !p.conn.portalDialog(artifact.id) {
			t.Fatal("artifact click was not handled")
		}
		time.Sleep(3 * time.Second)
		synctest.Wait()
		if len(p.fx.shields) != 1 || p.fx.shields[0].shieldTotal != 3000 {
			t.Fatalf("artifact shields=%v, want one 3000-point shield", p.fx.shields)
		}
		if len(other.fx.shields) != 0 {
			t.Fatal("artifact shield leaked into another run")
		}
	})
}

func TestNochsanaFortressGateSpawnsAndCanBeDestroyed(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	p, _ := fighter(t, s, 1000)
	s.visMu.Lock()
	in := s.newInstance(nochsanaWorld)
	defer func() { s.destroyInstance(in); s.visMu.Unlock() }()
	var gate *object
	for _, o := range s.byID {
		if o.worldID == in.world && o.instance == in.id && o.npc != nil && o.npc.ID == 256694 {
			gate = o
			break
		}
	}
	if gate == nil {
		t.Fatal("Nochsana fortress gate is not spawned")
	}
	if gate.staticID != 23 {
		t.Fatalf("fortress gate static ID=%d, want client door 23", gate.staticID)
	}
	info := s.npcInfo(gate, p).Data[1:]
	if got := binary.LittleEndian.Uint16(info[len(info)-18:]); got != 23 {
		t.Fatalf("gate packet static ID=%d, want 23", got)
	}
	if info[24] != npcTypeIDs["ATTACKABLE"] {
		t.Fatalf("gate packet npc type=%d, want attackable", info[24])
	}
	if s.canAttackNpc(p, gate) || s.isEnemyOf(p, gate) {
		t.Fatal("player outside this run can attack the gate")
	}
	p.WorldID, p.instance = in.world, in.id
	if !s.canAttackNpc(p, gate) || !s.isEnemyOf(p, gate) {
		t.Fatal("fortress gate rejects player attacks")
	}
	gate.hp = 1
	s.npcHit(gate, p, 0, statusDamage, 1)
	if !gate.dead {
		t.Fatal("fortress gate did not die when its HP reached zero")
	}
}

func TestNochsanaSpawnedMobSkillsHaveExecutableEffects(t *testing.T) {
	d := staticDataOrSkip(t)
	checked := 0
	for _, group := range d.Spawns[nochsanaWorld] {
		if group.NpcID < 256677 || group.NpcID > 256693 {
			continue
		}
		if d.Npcs[group.NpcID] == nil {
			t.Fatalf("spawned mob %d has no template", group.NpcID)
		}
		skills := d.NpcSkills[group.NpcID]
		if len(skills) == 0 {
			t.Fatalf("spawned mob %d has no skill list", group.NpcID)
		}
		for _, pick := range skills {
			skill := d.Skills[pick.SkillID]
			if skill == nil {
				t.Fatalf("mob %d skill %d has no template", group.NpcID, pick.SkillID)
			}
			for _, effect := range skill.Effects {
				if _, ok := effectHandlers[effect.Kind]; !ok {
					t.Fatalf("mob %d skill %d effect %s has no handler", group.NpcID, pick.SkillID, effect.Kind)
				}
			}
		}
		checked++
	}
	if checked < 17 {
		t.Fatalf("checked %d mob groups, want at least 17", checked)
	}
}

func TestNochsanaSiegeItemSummonsWeaponAndHitsOwnGate(t *testing.T) {
	d := staticDataOrSkip(t)
	item := d.Items[182202179]
	if item == nil || len(item.Actions) != 1 || item.Actions[0].Name != "skilluse" || item.Actions[0].Int("skillid") != 9953 {
		t.Fatal("Elyos General quest item does not cast the siege summon")
	}
	if d.Skills[9953] == nil || d.Skills[18008] == nil {
		t.Fatal("siege item skills missing")
	}
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, _ := fighter(t, s, 1000)
		s.visMu.Lock()
		in := s.newInstance(nochsanaWorld)
		other := s.newInstance(nochsanaWorld)
		defer func() {
			s.visMu.Lock()
			s.destroyInstance(in)
			s.destroyInstance(other)
			s.visMu.Unlock()
		}()
		var gate, foreign *object
		for _, o := range s.byID {
			if o.npc == nil || o.npc.ID != 256694 || o.worldID != nochsanaWorld {
				continue
			}
			if o.instance == in.id {
				gate = o
			} else if o.instance == other.id {
				foreign = o
			}
		}
		if gate == nil || foreign == nil {
			t.Fatal("missing instance gate")
		}
		p.WorldID, p.instance = in.world, in.id
		p.X, p.Y, p.Z = gate.x-2, gate.y, gate.z
		s.spawnLocked(p)
		(&skill{s: s, tmpl: d.Skills[9953], effector: p, first: p, level: 1}).use()
		s.visMu.Unlock()
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if p.summon == nil || p.summon.npc.ID != 201054 {
			t.Fatal("quest item's skill did not summon the siege weapon")
		}
		foreignHP := foreign.hp
		p.conn.summonCastspell(packet(cmSummonCastspell, func(w *wire.Writer) {
			w.D(p.summon.id)
			w.H(18008)
			w.C(0)
			w.D(foreign.id)
			w.F(0)
		}))
		time.Sleep(21 * time.Second)
		synctest.Wait()
		if foreign.hp != foreignHP {
			t.Fatal("siege weapon damaged a gate in another run")
		}
		gateHP := gate.hp
		// A single siege blow can be dodged, parried or blocked by the gate.
		for range 10 {
			p.conn.summonCastspell(packet(cmSummonCastspell, func(w *wire.Writer) {
				w.D(p.summon.id)
				w.H(18008)
				w.C(0)
				w.D(gate.id)
				w.F(0)
			}))
			time.Sleep(21 * time.Second)
			synctest.Wait()
			if gate.hp < gateHP {
				break
			}
		}
		if gate.hp >= gateHP {
			t.Fatal("siege weapon did not damage its own gate")
		}
		// 4.6 data: each blow adds 19900 against castle doors, about a quarter of the gate.
		if hit := gateHP - gate.hp; hit < gate.maxHP/10 {
			t.Fatalf("siege blow did %d of %d gate HP, want the castle door bonus", hit, gate.maxHP)
		}
		t.Logf("siege blow: %d of %d", gateHP-gate.hp, gate.maxHP)
	})
}

func TestNochsanaSiegeCommandMovesAndHitsOwnGate(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, tap := fighter(t, s, 1000)
		s.visMu.Lock()
		in := s.newInstance(nochsanaWorld)
		other := s.newInstance(nochsanaWorld)
		defer func() {
			s.visMu.Lock()
			s.destroyInstance(in)
			s.destroyInstance(other)
			s.visMu.Unlock()
		}()
		var gate, foreign *object
		for _, o := range s.byID {
			if o.npc == nil || o.npc.ID != 256694 || o.worldID != nochsanaWorld {
				continue
			}
			if o.instance == in.id {
				gate = o
			} else if o.instance == other.id {
				foreign = o
			}
		}
		if gate == nil || foreign == nil {
			t.Fatal("missing fortress gates")
		}
		p.WorldID, p.instance = in.world, in.id
		p.X, p.Y, p.Z = gate.x, gate.y+18, gate.z
		s.spawnLocked(p)
		s.createSummon(p, 201054, 1)
		s.visMu.Unlock()
		if p.summon == nil {
			t.Fatal("siege weapon was not summoned")
		}
		if tap.count(smDelete) != 0 || tap.count(smNpcInfo) == 0 {
			t.Fatal("siege weapon was deleted on the client right after it was summoned")
		}
		command := func(id int32) {
			p.conn.summonCommand(packet(cmSummonCommand, func(w *wire.Writer) {
				w.C(summonAttack)
				w.D(0)
				w.D(0)
				w.D(id)
			}))
		}
		command(foreign.id)
		if p.summon.summonMode != summonGuard {
			t.Fatal("siege weapon accepted a gate in another run")
		}
		gateHP, foreignHP := gate.hp, foreign.hp
		command(gate.id)
		// Long enough for several blows: any single one can be dodged.
		time.Sleep(200 * time.Second)
		synctest.Wait()
		if gate.hp >= gateHP || foreign.hp != foreignHP {
			t.Fatalf("own gate HP=%d/%d; other gate HP=%d/%d; summon=(%.1f,%.1f,%.1f) mode=%d cast=%v target=%d",
				gate.hp, gateHP, foreign.hp, foreignHP, p.summon.x, p.summon.y, p.summon.z,
				p.summon.summonMode, p.summon.cast != nil, p.summon.targetID)
		}
		if !inRange3D(p.summon, gate, 6) {
			t.Fatal("siege weapon did not approach the gate")
		}
		p.conn.summonCommand(packet(cmSummonCommand, func(w *wire.Writer) {
			w.C(summonGuard)
			w.D(0)
			w.D(0)
			w.D(0)
		}))
		if p.summon.siegeTask != nil || p.summon.summonMode != summonGuard {
			t.Fatal("guard command did not stop the siege")
		}
	})
}

func TestNochsanaTeleporterReservistsJoinAndLeaveWithHer(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, _ := fighter(t, s, 1000)
		s.visMu.Lock()
		in := s.newInstance(nochsanaWorld)
		defer func() {
			s.visMu.Lock()
			s.destroyInstance(in)
			s.visMu.Unlock()
		}()
		var tele *object
		for _, o := range s.byID {
			if o.npc != nil && o.npc.ID == nochsanaTeleporter && o.instance == in.id {
				tele = o
			}
		}
		if tele == nil {
			t.Fatal("Teleporter is not spawned")
		}
		p.WorldID, p.instance = in.world, in.id
		p.X, p.Y, p.Z = tele.x+3, tele.y, tele.z
		p.adminInvulnerable = true
		s.spawnLocked(p)
		reservists := func() (n int) {
			for _, o := range s.byID {
				if o.npc != nil && o.npc.ID == nochsanaReservist && o.instance == in.id {
					n++
					if distance3D(o.x, o.y, o.z, p.X, p.Y, p.Z) > 5.01 {
						t.Errorf("reservist %.1f m from the target, want within 5", distance3D(o.x, o.y, o.z, p.X, p.Y, p.Z))
					}
				}
			}
			return n
		}
		s.npcHit(tele, p, 0, statusDamage, 1)
		s.visMu.Unlock()
		if n := reservists(); n != 1 {
			t.Fatalf("%d reservists when the fight started, want 1", n)
		}
		time.Sleep(30 * time.Second)
		synctest.Wait()
		if n := reservists(); n != 2 {
			t.Fatalf("%d reservists after 30s above 71%% HP, want 2", n)
		}
		s.visMu.Lock()
		tele.hp = tele.maxHP / 2
		s.visMu.Unlock()
		time.Sleep(30 * time.Second)
		synctest.Wait()
		if n := reservists(); n != 2 {
			t.Fatalf("%d reservists after 30s below 71%% HP, want still 2", n)
		}
		s.visMu.Lock()
		s.npcHit(tele, p, 0, statusDamage, tele.hp)
		s.visMu.Unlock()
		if !tele.dead || reservists() != 0 {
			t.Fatalf("dead=%v with %d reservists left, want none", tele.dead, reservists())
		}
	})
}

func TestNochsanaGateFallReleasesTwoElitesAtItsBreaker(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, _ := fighter(t, s, 1000)
		s.visMu.Lock()
		in := s.newInstance(nochsanaWorld)
		defer func() {
			s.visMu.Lock()
			s.destroyInstance(in)
			s.visMu.Unlock()
		}()
		var gate *object
		for _, o := range s.byID {
			if o.npc != nil && o.npc.ID == nochsanaGate && o.instance == in.id {
				gate = o
			}
		}
		p.WorldID, p.instance = in.world, in.id
		p.X, p.Y, p.Z = gate.x, gate.y+5, gate.z
		p.adminInvulnerable = true
		s.spawnLocked(p)
		before := map[int32]bool{}
		for id := range s.byID {
			before[id] = true
		}
		s.npcHit(gate, p, 0, statusDamage, gate.hp)
		elites := map[int32]*object{}
		for id, o := range s.byID {
			if !before[id] && o.npc != nil && o.instance == in.id && (o.npc.ID == 256686 || o.npc.ID == 256682) {
				elites[o.npc.ID] = o
			}
		}
		s.visMu.Unlock()
		if !gate.dead || len(elites) != 2 {
			t.Fatalf("gate dead=%v, %d elites inside the fort, want an assassin and a fighter", gate.dead, len(elites))
		}
		for _, e := range elites {
			if e.aggro[p.ID] == nil {
				t.Errorf("elite %d does not hate the gate's breaker", e.npc.ID)
			}
		}
		time.Sleep(601 * time.Second)
		synctest.Wait()
		for _, e := range elites {
			if s.byID[e.id] == e {
				t.Errorf("elite %d outlived its 600 seconds", e.npc.ID)
			}
		}
	})
}

func TestNochsanaGeneralScriptCastsHisThresholdsOnce(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, tap := fighter(t, s, 1000)
		s.visMu.Lock()
		in := s.newInstance(nochsanaWorld)
		defer func() {
			s.visMu.Lock()
			s.destroyInstance(in)
			s.visMu.Unlock()
		}()
		var gen *object
		for _, o := range s.byID {
			if o.npc != nil && o.npc.ID == nochsanaGeneral && o.instance == in.id {
				gen = o
			}
		}
		p.WorldID, p.instance = in.world, in.id
		p.X, p.Y, p.Z = gen.x+2, gen.y, gen.z
		p.adminInvulnerable = true
		s.spawnLocked(p)
		s.npcHit(gen, p, 0, statusDamage, 1)
		if gen.script == nil || gen.ai.has(&skillUseDesire{}) {
			t.Fatal("the General's script did not start, or he still casts at random")
		}
		s.visMu.Unlock()
		for _, step := range []struct{ hp, at int32 }{{80, 85}, {60, 70}, {22, 25}, {10, 20}} {
			s.visMu.Lock()
			gen.hp = gen.maxHP * step.hp / 100
			s.visMu.Unlock()
			time.Sleep(5 * time.Second)
			synctest.Wait()
			s.visMu.Lock()
			used := gen.script != nil && gen.script.used[int(step.at)]
			s.visMu.Unlock()
			if !used {
				t.Fatalf("at %d%% HP the %d%% threshold did not fire", step.hp, step.at)
			}
		}
		if tap.count(smCastspell) == 0 {
			t.Fatal("the General's script cast nothing")
		}
		s.visMu.Lock()
		s.npcHit(gen, p, 0, statusDamage, gen.hp)
		s.visMu.Unlock()
		if !gen.dead || gen.script != nil {
			t.Fatal("the script outlived the General")
		}
	})
}
