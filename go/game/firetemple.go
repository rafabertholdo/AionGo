package game

import (
	"math"
	"math/rand/v2"
	"time"
)

// Fire Temple (IDDF2_Dflame). The 1.9 client and 4.6 data give every mob the same AI name;
// these are the scripted ones the generic AI does not cover.
const (
	fireTempleWorld   = 320100000
	kromedeTheCorrupt = 212846
	vileJudgeKromede  = 214621
	kromedeTrap       = 280501
)

// fireTempleObscuras are the ND2_AnN obscuras: once below half HP they may spin, hide and flee.
var fireTempleObscuras = []int32{212795, 212797, 212842}

func init() {
	for _, id := range []int32{kromedeTheCorrupt, vileJudgeKromede} {
		npcEventHooks[id] = (*Server).kromedeEvent
		scriptedCasters[id] = true
	}
	for _, id := range fireTempleObscuras {
		npcEventHooks[id] = (*Server).obscuraEvent
	}
}

// fireTempleVariant is the 4.6 spawn territory DF2_C5Dg_F_FireSanctuaryQueenBoss_37_1:
// group G1 (Kromede the Corrupt) at select_prob 900, G2 (Vile Judge Kromede) at 100.
// The 1.9 spawn data only places the Vile Judge, which quest 1470's 212846 never meets.
func fireTempleVariant(world, npcID int32) int32 {
	if world == fireTempleWorld && npcID == vileJudgeKromede && rand.IntN(10) != 0 {
		return kromedeTheCorrupt
	}
	return npcID
}

// kromedeEvent is ND2_Sum_B: a self buff while idle, a knockback opening, then traps
// around her on a timer, and a last stand below 20% HP.
func (s *Server) kromedeEvent(o *object, ev aiEvent) {
	switch ev {
	case evRespawned:
		s.castNpcSkill(o, 17052, o)
	case evAttacked:
		if o.script == nil && !o.dead && o.ai.state == aiAttacking {
			s.startScript(o, 6*time.Second, 30*time.Second, (*Server).kromedeStep,
				scriptCast{skill: 17047}, scriptCast{skill: 16847})
		}
	case evDied, evDespawn, evBackHome:
		s.stopScript(o)
	}
}

func (s *Server) kromedeStep(o *object, sc *npcScript, _ creature) {
	hp := o.hp * 100 / max(o.maxHP, 1)
	if sc.due(0, 6*time.Second) && hp < 20 && sc.once(20) {
		sc.add(scriptCast{skill: 17052, self: true}, scriptCast{skill: 17056, self: true})
	}
	traps, period := 2, 35*time.Second
	if hp <= 50 {
		traps, period = 3, 25*time.Second
	}
	if sc.due(1, period) {
		sc.add(scriptCast{skill: 16674, self: true})
		s.kromedeTraps(o, traps)
	}
}

// kromedeTraps are BDF2_NM_DMCromedeTrap_37_Ah (NTrap_A): each wakes within 4 metres of
// her, bursts its bleed at once and is gone.
func (s *Server) kromedeTraps(o *object, n int) {
	for range n {
		angle := rand.Float64() * 2 * math.Pi
		r := float32(rand.Float64() * 4)
		s.spawnBurst(o, kromedeTrap, o.x+r*float32(math.Cos(angle)), o.y+r*float32(math.Sin(angle)), o.z)
	}
}

// obscuraEvent is ND2_AnN: hit below half HP, a 30% chance (once a life) to spin at the
// target, hide, flee for 4 seconds, then come back behind it.
func (s *Server) obscuraEvent(o *object, ev aiEvent) {
	switch ev {
	case evAttacked:
		if o.fled || o.dead || o.hp*2 >= o.maxHP || rand.IntN(10) >= 3 {
			return
		}
		o.fled = true
		s.startScript(o, time.Hour, time.Hour, (*Server).obscuraStep,
			scriptCast{skill: 16794}, scriptCast{skill: 16677, self: true},
			scriptCast{flee: 4 * time.Second}, scriptCast{skill: 16556})
	case evDied, evDespawn, evBackHome:
		s.stopScript(o)
	}
}

// obscuraStep ends the script once its sequence has played.
func (s *Server) obscuraStep(o *object, sc *npcScript, _ creature) {
	if len(sc.queue) == 0 && o.cast == nil {
		s.stopScript(o)
	}
}
