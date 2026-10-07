package game

import (
	"math"
	"math/rand/v2"
	"slices"
	"time"
)

// Aetherogenetics Lab (IDLF3Lp). The 1.9 client and 4.6 data give every mob the same AI
// name. 4.6 added skills to most Lepharist lists, so their skill slots do not map onto
// 1.9; only the patterns' spawns, flees and calls for help are run here.
const (
	perfectedPretor  = 212205
	perfectedMud     = 212206 // spawned where the pretor fell (D2_AnH)
	rm78c            = 212211
	strangeCreature  = 280790 // RM-78c's dying burst (NLehpar_BhA)
	summonedPretor   = 280262 // a scholar's add (NLehpar_WeA)
	lepharistSnare   = 280466 // a sniper's ice-snare trap (XLehpar_ReB_S1)
	lepharistFleeHP  = 35
	lepharistHelpR   = 10
	lepharistTimer   = 6 * time.Second
	lepharistFleeFor = 5 * time.Second
)

var (
	lepharistScholars = []int32{212175, 212176, 212177, 212183} // NLehpar_WeA
	lepharistSnipers  = []int32{212171, 212190}                 // XLehpar_ReB_S1
	lepharistFleers   = []int32{212172, 212179, 212186, 212187, // NLehpar_FeA
		212178, 212185, 212184, 212169, 212180, 212188, 212189} // NLehpar_KeA, PnA, AnC
)

func init() {
	for _, list := range [][]int32{lepharistScholars, lepharistSnipers, lepharistFleers} {
		for _, id := range list {
			npcEventHooks[id] = (*Server).lepharistEvent
		}
	}
	npcEventHooks[summonedPretor] = (*Server).summonedPretorEvent
	for id := range labBosses {
		npcEventHooks[id] = (*Server).labBossEvent
		scriptedCasters[id] = true
	}
}

// lepharistEvent runs the Lepharist patterns' battle timer: below 35% HP, once, the npc
// flees its target for 5 seconds (a sniper first drops an ice-snare trap and runs for 4)
// and then calls its allies within 10 metres; a scholar between 36% and 70% HP, once,
// summons a pretor beside itself.
func (s *Server) lepharistEvent(o *object, ev aiEvent) {
	switch ev {
	case evAttacked:
		if o.script == nil && !o.dead && o.ai.state == aiAttacking {
			s.startScript(o, lepharistTimer, time.Hour, (*Server).lepharistStep)
		}
	case evDied, evDespawn, evBackHome:
		s.stopScript(o)
	}
}

func (s *Server) lepharistStep(o *object, sc *npcScript, _ creature) {
	if !sc.due(0, lepharistTimer) {
		return
	}
	hp := o.hp * 100 / max(o.maxHP, 1)
	help := scriptCast{act: func(s *Server, o *object, t creature) { s.callForHelp(o, t, lepharistHelpR) }}
	switch {
	case hp < lepharistFleeHP && sc.once(lepharistFleeHP):
		if slices.Contains(lepharistSnipers, o.npc.ID) {
			sc.add(scriptCast{act: func(s *Server, o *object, _ creature) { s.spawnBurst(o, lepharistSnare, o.x, o.y, o.z) }},
				scriptCast{flee: 4 * time.Second}, help)
		} else {
			sc.add(scriptCast{flee: lepharistFleeFor}, help)
		}
	case hp <= 70 && hp > lepharistFleeHP && slices.Contains(lepharistScholars, o.npc.ID) && sc.once(70):
		sc.add(scriptCast{act: (*Server).summonPretor})
	}
}

// summonPretor is the scholar's spawn of BLF2_Lehpar_Pretor_pretor_37 within 2 metres.
func (s *Server) summonPretor(o *object, target creature) {
	t := s.data.Npcs[summonedPretor]
	if t == nil {
		return
	}
	angle := rand.Float64() * 2 * math.Pi
	p := s.spawnTempNpc(o, t, o.x+2*float32(math.Cos(angle)), o.y+2*float32(math.Sin(angle)), o.z, 3000*time.Second)
	s.addHate(p, target, 1)
}

// summonedPretorEvent is NLehpar_Sum: it is gone once it stops fighting.
func (s *Server) summonedPretorEvent(o *object, ev aiEvent) {
	if ev == evBackHome {
		s.removeTempNpc(o)
	}
}

// aetherLabDied is the on_killed patterns: the Perfected Pretor leaves a Perfected
// Mudthorn where it fell (600 seconds), and RM-78c a strange creature that bursts.
func (s *Server) aetherLabDied(o *object) {
	switch o.npc.ID {
	case perfectedPretor:
		if t := s.data.Npcs[perfectedMud]; t != nil {
			s.spawnTempNpc(o, t, o.x, o.y, o.z, 600*time.Second)
		}
	case rm78c:
		s.spawnBurst(o, strangeCreature, o.x, o.y, o.z)
	}
}

// labBoss is a named mob's 4.6 pattern: its opening (timers armed, first casts) and what
// each battle timer does when it goes off. Skill slots are mapped by name onto the 1.9
// list; the few 4.6-only skills (RM-108c's and RM-78c's idle poison stance, RM-78c's
// self buff) are left out.
type labBoss struct {
	open  func(sc *npcScript)
	timer func(sc *npcScript, hp int32, timer int)
}

func npcCast(id int32) scriptCast          { return scriptCast{skill: id} }
func selfCast(id int32) scriptCast         { return scriptCast{skill: id, self: true} }
func castOn(id int32, pick int) scriptCast { return scriptCast{skill: id, pick: pick} }
func between(hp, low, high int32) bool     { return hp >= low && hp <= high }

var labBosses = map[int32]labBoss{
	// Perfected Pretor (D2_AnH).
	perfectedPretor: {
		open: func(sc *npcScript) { sc.arm(0, 12*time.Second); sc.add(npcCast(16729)) },
		timer: func(sc *npcScript, _ int32, timer int) {
			if timer == 0 {
				sc.arm(0, 20*time.Second)
				sc.add(npcCast(16715))
			}
		},
	},
	// Pretor Key Keeper (ND2_AhA).
	212193: {
		open: func(sc *npcScript) {
			sc.arm(1, 30*time.Second)
			sc.arm(0, 6*time.Second)
			sc.add(npcCast(17934), npcCast(16556))
		},
		timer: func(sc *npcScript, hp int32, timer int) {
			switch {
			case timer == 4 && hp < 35:
				sc.arm(3, 10*time.Second)
				sc.add(npcCast(16729))
			case timer == 3 && hp < 35:
				sc.arm(4, 35*time.Second)
				sc.add(npcCast(16715), selfCast(17949))
			case timer == 2 && between(hp, 35, 76):
				sc.arm(2, 35*time.Second)
				sc.add(selfCast(17949))
			case timer == 1 && hp >= 76:
				sc.arm(0, 7*time.Second)
				sc.arm(1, 35*time.Second)
				sc.add(selfCast(17949))
			case timer == 0 && hp < 35 && sc.once(2):
				sc.arm(3, 10*time.Second)
				sc.add(npcCast(16729))
			case timer == 0 && between(hp, 35, 76) && sc.once(1):
				sc.arm(0, 7*time.Second)
				sc.arm(2, 35*time.Second)
				sc.add(npcCast(17934), npcCast(16556))
			case timer == 0:
				sc.arm(0, 6*time.Second)
			}
		},
	},
	// The Keykeeper (ND2_KeA).
	212341: {
		open: func(sc *npcScript) {
			sc.arm(0, 6*time.Second)
			sc.arm(1, 30*time.Second)
			sc.add(npcCast(17465), npcCast(16997))
		},
		timer: func(sc *npcScript, hp int32, timer int) {
			switch {
			case timer == 4:
				sc.arm(4, 35*time.Second)
				sc.add(npcCast(16673))
			case timer == 2 && between(hp, 36, 75):
				sc.arm(2, 30*time.Second)
				sc.add(npcCast(16673))
			case timer == 1 && hp >= 76:
				sc.arm(1, 30*time.Second)
				sc.add(npcCast(16673))
			case timer == 0 && hp < 35 && sc.once(2):
				sc.arm(4, 35*time.Second)
				sc.add(selfCast(16997), switchTo(pickLowestHP))
			case timer == 0 && between(hp, 36, 75) && sc.once(1):
				sc.arm(2, 7*time.Second)
				sc.arm(0, 6*time.Second)
				sc.add(selfCast(16522), switchTo(pickLowestHP))
			case timer == 0:
				sc.arm(0, 6*time.Second)
			}
		},
	},
	// Key Eater (ND2_ChA): every cast is an area around itself.
	212342: {
		open: func(sc *npcScript) {
			sc.arm(1, 35*time.Second)
			sc.arm(0, 12*time.Second)
			sc.add(selfCast(17938), selfCast(16837), selfCast(16838))
		},
		timer: func(sc *npcScript, hp int32, timer int) {
			switch {
			case timer == 3:
				sc.add(selfCast(17938), selfCast(16838))
			case timer == 2 && between(hp, 36, 70):
				sc.arm(2, 30*time.Second)
				sc.add(selfCast(17938), selfCast(16837))
			case timer == 1 && hp >= 71:
				sc.arm(1, 35*time.Second)
				sc.add(selfCast(17938))
			case timer == 0 && hp < 35 && sc.once(2):
				sc.arm(3, 30*time.Second)
				sc.add(selfCast(16563), selfCast(16837), selfCast(16838))
			case timer == 0 && between(hp, 36, 70) && sc.once(1):
				sc.arm(0, 10*time.Second)
				sc.arm(2, 35*time.Second)
				sc.add(selfCast(16917), selfCast(16837), selfCast(16838))
			case timer == 0:
				sc.arm(0, 7*time.Second)
			}
		},
	},
	// Head Chef Pamsey (XLehpar_CeB): she keeps turning on the weakest attacker.
	212181: {
		open: func(sc *npcScript) {
			sc.arm(0, 8*time.Second)
			sc.add(selfCast(17488), castOn(17446, pickLowestHP), switchTo(pickLowestHP))
		},
		timer: func(sc *npcScript, hp int32, timer int) {
			switch {
			case timer == 3:
				sc.arm(3, 25*time.Second)
				sc.add(npcCast(17447), castOn(16516, pickLowestHP), selfCast(17446), switchTo(pickLowestHP))
			case timer == 2 && between(hp, 36, 75):
				sc.arm(2, 25*time.Second)
				sc.add(npcCast(16516), selfCast(17446))
			case timer == 1 && hp >= 76:
				sc.arm(1, 25*time.Second)
				sc.add(npcCast(16516))
			case timer == 0 && hp < 35 && sc.once(3):
				sc.arm(3, 25*time.Second)
				sc.add(selfCast(16682), castOn(16516, pickLowestHP), selfCast(17446), switchTo(pickLowestHP))
			case timer == 0 && between(hp, 36, 75) && sc.once(2):
				sc.arm(0, 7*time.Second)
				sc.arm(2, 25*time.Second)
				sc.add(selfCast(17482), castOn(17447, pickLowestHP), switchTo(pickLowestHP))
			case timer == 0 && hp >= 76 && sc.once(1):
				sc.arm(1, 25*time.Second)
				sc.arm(0, 6*time.Second)
				sc.add(npcCast(16516))
			case timer == 0:
				sc.arm(0, 6*time.Second)
			}
		},
	},
	// RM-108c (NLehpar_AeC).
	212191: {
		open: func(sc *npcScript) { sc.arm(0, 6*time.Second) },
		timer: func(sc *npcScript, hp int32, timer int) {
			switch {
			case timer != 0:
			case hp < 35 && sc.once(3):
				sc.add(selfCast(17444), npcCast(16761), castOn(17977, pickLowestHP), castOn(16761, pickThird), switchTo(pickLowestHP))
			case between(hp, 36, 70) && sc.once(2):
				sc.arm(0, 12*time.Second)
				sc.add(npcCast(17984), npcCast(16761))
			case hp >= 71 && sc.once(1):
				sc.arm(0, 10*time.Second)
				sc.add(selfCast(17977))
			default:
				sc.arm(0, 6*time.Second)
			}
		},
	},
	// RM-78c (NLehpar_BhA): it spreads its signets over the second-most hated, and once
	// between 31% and 50% HP runs about before turning on the third.
	rm78c: {
		open: func(sc *npcScript) {
			sc.arm(0, 7*time.Second)
			sc.add(selfCast(17934), switchTo(pickRandom))
		},
		timer: func(sc *npcScript, hp int32, timer int) {
			switch {
			case timer == 8 && hp < 20:
				sc.arm(7, 15*time.Second)
				sc.add(castOn(17984, pickSecond), switchTo(pickSecond))
			case timer == 7 && hp < 20:
				sc.arm(8, 20*time.Second)
				sc.add(selfCast(16608), castOn(17910, pickSecond), switchTo(pickSecond))
			case timer == 6 && between(hp, 31, 50):
				sc.arm(1, 7*time.Second)
				sc.arm(5, 20*time.Second)
				sc.add(npcCast(16608))
			case timer == 5 && between(hp, 31, 50):
				sc.arm(1, 9*time.Second)
				sc.arm(6, 15*time.Second)
				sc.add(npcCast(17910), npcCast(17984))
			case timer == 3 && between(hp, 51, 80):
				sc.arm(1, 7*time.Second)
				sc.arm(3, 15*time.Second)
				sc.add(npcCast(16608))
			case timer == 2 && hp >= 81:
				sc.arm(2, 20*time.Second)
				sc.arm(1, 6*time.Second)
				sc.add(npcCast(16608))
			case timer == 0:
				sc.arm(1, 6*time.Second)
				sc.arm(2, 20*time.Second)
				sc.add(npcCast(16608))
			case timer == 1 && hp < 20 && sc.once(3):
				sc.arm(8, 20*time.Second)
				sc.add(selfCast(17913), castOn(17910, pickSecond), switchTo(pickSecond))
			case timer == 1 && between(hp, 31, 50) && sc.once(2):
				sc.arm(1, 6*time.Second)
				sc.add(selfCast(17912), scriptCast{wander: 3 * time.Second}, scriptCast{act: rm78cStopped})
			case timer == 1 && between(hp, 51, 80) && sc.once(1):
				sc.arm(1, 14*time.Second)
				sc.arm(3, 25*time.Second)
				sc.add(selfCast(17934), castOn(17910, pickSecond), castOn(17984, pickSecond), switchTo(pickSecond))
			case timer == 1:
				sc.arm(1, 6*time.Second)
			}
		},
	},
}

// rm78cStopped is RM-78c's on_stop_to_random_move.
func rm78cStopped(s *Server, o *object, _ creature) {
	sc := o.script
	if sc == nil || !between(o.hp*100/max(o.maxHP, 1), 31, 50) {
		return
	}
	sc.arm(6, 15*time.Second)
	sc.arm(1, 6*time.Second)
	sc.add(castOn(17910, pickSecond), castOn(17984, pickSecond), switchTo(pickThird))
}

// labBossEvent starts a named mob's pattern when it enters a fight and ends it with the fight.
func (s *Server) labBossEvent(o *object, ev aiEvent) {
	switch ev {
	case evAttacked:
		if o.script != nil || o.dead || o.ai.state != aiAttacking {
			return
		}
		b := labBosses[o.npc.ID]
		s.startScript(o, 0, 0, func(s *Server, o *object, sc *npcScript, _ creature) {
			hp := o.hp * 100 / max(o.maxHP, 1)
			for timer := len(sc.timers) - 1; timer >= 0; timer-- {
				if sc.fired(timer) {
					b.timer(sc, hp, timer)
				}
			}
		})
		o.script.timers = [10]time.Time{}
		b.open(o.script)
	case evDied:
		s.stopScript(o)
		s.aetherLabDied(o)
	case evDespawn, evBackHome:
		s.stopScript(o)
	}
}
