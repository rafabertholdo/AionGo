package game

import (
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"aionlightning/game/data"
)

const (
	nochsanaWorld      = 300030000
	nochsanaTeleporter = 256691
	nochsanaReservist  = 290163
	nochsanaGeneral    = 256693
	nochsanaGate       = 256694
)

// isNochsanaGate is the fortress gate: a USEITEM template the 1.9 client
// must still be told is attackable.
func isNochsanaGate(o *object) bool {
	return o.worldID == nochsanaWorld && o.npc != nil && o.npc.ID == 256694
}

// siegeDoorBonus is the damage the siege pets' blows add against castle doors.
// The 1.9 XML omits it; the 4.6 skill_base gives 18008/17814 +19900 for the
// PC_LIGHT, PC_DARK and DRAGON castle door races (about a quarter of the NTC gate).
var siegeDoorBonus = map[int32]int32{17814: 19900, 18008: 19900}

func (e *effect) siegeDoorBonus() int32 {
	o, ok := e.effected.(*object)
	if !ok || o.npc == nil || !strings.HasSuffix(o.npc.Race, "_CASTLE_DOOR") {
		return 0
	}
	return siegeDoorBonus[e.tmpl.ID]
}

// nochsanaArtifact activates the shield declared for the camp's artifact.
func (c *conn) nochsanaArtifact(p *player, o *object) bool {
	if distance3D(p.X, p.Y, p.Z, o.x, o.y, o.z) > 5 || o.useTask != nil {
		return true
	}
	s := c.s
	tmpl := s.data.Skills[1872]
	if tmpl == nil {
		return true
	}
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = s.later(3*time.Second, func() {
		o.useTask = nil
		if p.conn != c || !p.spawned || p.dead || o.dead || p.seen[o.id] != o ||
			s.byID[o.id] != o || p.WorldID != nochsanaWorld || p.instance != o.instance ||
			distance3D(p.X, p.Y, p.Z, o.x, o.y, o.z) > 5 {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(s.playerEmotionTo(p, emoteEndQuestLoot, 0, o.id, 0, 0, 0, 0), true)
		// The skill catalog describes an area shield but the generic area
		// selector also counts monsters. Keep its six nearby PC targets and
		// use the original spell effects and duration.
		targets := []creature{p}
		for _, other := range s.creaturesNear(o, 25) {
			ally, ok := other.(*player)
			if !ok || ally == p || ally.dead || len(targets) > 6 {
				continue
			}
			targets = append(targets, ally)
		}
		areaSkill := *tmpl
		areaSkill.SetProperties = nil
		(&skill{s: s, tmpl: &areaSkill, effector: o, first: p, targets: targets, level: 1}).use()
	})
	return true
}

func init() {
	npcEventHooks[nochsanaTeleporter] = (*Server).nochsanaTeleporterEvent
	npcEventHooks[nochsanaGeneral] = (*Server).nochsanaGeneralEvent
	scriptedCasters[nochsanaGeneral] = true
}

// nochsanaTeleporterEvent is the Teleporter's MiNaga_WeB pattern (the AI name in
// both the 1.9 client and 4.6 data): a reservist joins beside her target when the
// fight starts and every 30 seconds while she is above 71% HP. They live 300
// seconds and leave when she dies or stops fighting.
// ponytail: her timed stun/water casts stay on the generic random skill choice.
func (s *Server) nochsanaTeleporterEvent(o *object, ev aiEvent) {
	switch ev {
	case evAttacked:
		if o.reserveTask != nil || o.dead || o.ai.state != aiAttacking {
			return
		}
		s.nochsanaReservist(o)
		o.reserveTask = s.every(30*time.Second, 30*time.Second, func() {
			if o.hp*100 > o.maxHP*71 {
				s.nochsanaReservist(o)
			}
		})
	case evDied, evDespawn, evBackHome:
		o.reserveTask.cancel()
		o.reserveTask = nil
		for _, r := range o.reservists {
			s.removeTempNpc(r)
		}
		o.reservists = nil
	}
}

// nochsanaReservist puts one reservist within 5 metres of the Teleporter's target.
func (s *Server) nochsanaReservist(o *object) {
	t := s.data.Npcs[nochsanaReservist]
	target := s.mostHated(o)
	if t == nil || target == nil {
		return
	}
	_, x, y, z := target.loc()
	angle := rand.Float64() * 2 * math.Pi
	x += float32(5 * math.Cos(angle))
	y += float32(5 * math.Sin(angle))
	o.reservists = append(o.reservists, s.spawnTempNpc(o, t, x, y, z, 300*time.Second))
}

// spawnTempNpc puts a scripted spawn into the npc's run that leaves after life.
func (s *Server) spawnTempNpc(o *object, t *data.NpcTemplate, x, y, z float32, life time.Duration) *object {
	r := &object{id: s.ids.nextID(), worldID: o.worldID, instance: o.instance, x: x, y: y, z: z,
		homeX: x, homeY: y, homeZ: z, npc: t, noRespawn: true}
	s.initNpc(r)
	s.byID[r.id] = r
	s.addObject(r)
	r.ai.handleEvent(evRespawned)
	r.timers = append(r.timers, s.later(life, func() { s.removeTempNpc(r) }))
	return r
}

// removeTempNpc takes a scripted spawn out of the world for good, its movement and AI too.
func (s *Server) removeTempNpc(r *object) {
	if s.byID[r.id] != r {
		return
	}
	for _, t := range r.timers {
		t.cancel()
	}
	r.timers = nil
	r.decay.cancel()
	s.despawnNpc(r, true)
	r.dead = true
	delete(s.byID, r.id)
	s.ids.release(r.id)
}

// nochsanaGateFell is MiDoor: when the gate falls, an assassin and a fighter rush
// out from inside the fort (the starts of the 4.6 event paths) at whoever broke it.
// ponytail: they chase straight at the breaker; the 4.6 one-way run paths are not walker routes here.
func (s *Server) nochsanaGateFell(o *object, attacker creature) {
	if o.worldID != nochsanaWorld || o.npc.ID != nochsanaGate {
		return
	}
	s.nochsanaGeneralGateCredit(o)
	if summon, ok := attacker.(*object); ok && summon.owner != nil {
		attacker = summon.owner
	}
	for _, e := range []struct {
		npc     int32
		x, y, z float32
	}{{256686, 338.27417, 330.2651, 381.74893}, {256682, 344.71103, 329.13208, 381.74893}} {
		t := s.data.Npcs[e.npc]
		if t == nil {
			continue
		}
		elite := s.spawnTempNpc(o, t, e.x, e.y, e.z, 600*time.Second)
		if attacker != nil && !attacker.isDead() {
			s.addHate(elite, attacker, 1)
		}
	}
}

// nochsanaGeneralEvent runs the General's MiBGuard_ChiefC script (the 1.9 client's AI name; the
// 4.6 data keeps it beside its ver40 successor): a pull on engage, then two timers.
func (s *Server) nochsanaGeneralEvent(o *object, ev aiEvent) {
	switch ev {
	case evAttacked:
		if o.script == nil && !o.dead && o.ai.state == aiAttacking {
			s.startScript(o, 5*time.Second, 12*time.Second, (*Server).nochsanaGeneralStep,
				scriptCast{skill: 17303}, scriptCast{skill: 16796})
		}
	case evDied, evDespawn, evBackHome:
		s.stopScript(o)
	}
}

func (s *Server) nochsanaGeneralStep(o *object, sc *npcScript, target creature) {
	hp := o.hp * 100 / max(o.maxHP, 1)
	if sc.due(0, 5*time.Second) {
		switch {
		case dist(o, target) > 12: // he pulls a target that keeps its distance
			sc.add(scriptCast{skill: 17303})
		case hp < 20 && sc.once(20), hp < 70 && sc.once(70):
			sc.add(scriptCast{skill: 17305}, scriptCast{skill: 16736, self: true})
		case hp < 25 && sc.once(25):
			sc.add(scriptCast{skill: 16704})
		case hp < 85 && sc.once(85):
			sc.add(scriptCast{skill: 17290, self: true})
		}
	}
	if sc.due(1, 15*time.Second) {
		switch {
		case hp < 35 && rand.IntN(2) == 0:
			sc.add(scriptCast{skill: 16684})
		case hp < 35:
			sc.add(scriptCast{skill: 16531})
		case hp <= 50:
			sc.add(scriptCast{skill: 16796})
		default:
			sc.add(scriptCast{skill: 16519})
		}
	}
}
