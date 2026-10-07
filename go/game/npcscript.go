package game

import (
	"cmp"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"aionlightning/game/data"
)

// npcScript is a 4.6 AI pattern's fight: two battle timers, once-a-fight flags, and casts
// queued while the npc is casting. A flee step pauses its chase and attack.
type npcScript struct {
	task      *task
	timers    [10]time.Time // battle timers: when each fires, zero while disarmed
	used      map[int]bool
	queue     []scriptCast
	fleeUntil time.Time
	step      func(*Server, *object, *npcScript, creature)
}

type scriptCast struct {
	skill  int32
	self   bool
	pick   int                              // or a target picked off the hate list
	wander time.Duration                    // a random move instead of a cast
	flee   time.Duration                    // a flee from the target instead of a cast
	act    func(*Server, *object, creature) // or a scripted action, on the npc's target
}

func (sc *npcScript) add(c ...scriptCast) { sc.queue = append(sc.queue, c...) }

// once is a set_flag_var condition: true the first time in the fight.
func (sc *npcScript) once(flag int) bool {
	if sc.used[flag] {
		return false
	}
	sc.used[flag] = true
	return true
}

// due is a battle timer firing: it re-arms itself every period.
func (sc *npcScript) due(timer int, period time.Duration) bool {
	if now := time.Now(); !now.Before(sc.timers[timer]) {
		sc.timers[timer] = now.Add(period)
		return true
	}
	return false
}

// arm is add_battle_timer: the timer fires once after d.
func (sc *npcScript) arm(timer int, d time.Duration) { sc.timers[timer] = time.Now().Add(d) }

// fired is a one-shot battle timer going off: it stays disarmed until armed again.
func (sc *npcScript) fired(timer int) bool {
	if t := sc.timers[timer]; !t.IsZero() && !time.Now().Before(t) {
		sc.timers[timer] = time.Time{}
		return true
	}
	return false
}

// The targets a script picks off its hate list (ATTACKERI_*).
const (
	pickTarget = iota
	pickLowestHP
	pickSecond
	pickThird
	pickRandom
)

// picked is the attacker the indicator names, or nil when there is none.
func (s *Server) picked(o *object, pick int) creature {
	type hated struct {
		c    creature
		hate int32
	}
	var list []hated
	for id, info := range o.aggro {
		if c := s.creatureByID(id); c != nil && !c.isDead() && o.sees(c) && info.hate > 0 {
			list = append(list, hated{c, info.hate})
		}
	}
	if len(list) == 0 {
		return nil
	}
	switch pick {
	case pickLowestHP:
		return slices.MinFunc(list, func(a, b hated) int {
			ha, _ := a.c.hitPoints()
			hb, _ := b.c.hitPoints()
			return cmp.Compare(ha, hb)
		}).c
	case pickRandom:
		return list[rand.IntN(len(list))].c
	}
	slices.SortFunc(list, func(a, b hated) int { return cmp.Compare(b.hate, a.hate) })
	if rank := pick - pickSecond + 1; rank < len(list) {
		return list[rank].c
	}
	return nil
}

// switchTo is switch_target_by_attacker_indicator: 100 hate on the picked attacker.
func switchTo(pick int) scriptCast {
	return scriptCast{act: func(s *Server, o *object, _ creature) {
		if c := s.picked(o, pick); c != nil {
			s.addHate(o, c, 100)
		}
	}}
}

func (sc *npcScript) fleeing() bool { return sc != nil && time.Now().Before(sc.fleeUntil) }

// startScript begins a fight: the opening casts, and its timers first fire after first0 and first1.
func (s *Server) startScript(o *object, first0, first1 time.Duration, step func(*Server, *object, *npcScript, creature), opening ...scriptCast) {
	now := time.Now()
	sc := &npcScript{timers: [10]time.Time{now.Add(first0), now.Add(first1)}, used: map[int]bool{}, step: step}
	sc.add(opening...)
	o.script = sc
	sc.task = s.every(0, time.Second, func() { s.scriptStep(o, sc) })
}

func (s *Server) stopScript(o *object) {
	if o.script != nil {
		o.script.task.cancel()
		o.script = nil
	}
}

func (s *Server) scriptStep(o *object, sc *npcScript) {
	target := s.mostHated(o)
	if o.script != sc || o.dead || target == nil || sc.fleeing() {
		return
	}
	if sc.step != nil {
		sc.step(s, o, sc, target)
	}
	// Scripted actions (spawns, calls) don't wait for a cast; skills and flees do.
	for len(sc.queue) > 0 && sc.queue[0].act != nil && o.script == sc {
		c := sc.queue[0]
		sc.queue = sc.queue[1:]
		c.act(s, o, target)
	}
	if o.script != sc || len(sc.queue) == 0 || o.cast != nil || !s.canAttackOut(o) {
		return
	}
	c := sc.queue[0]
	sc.queue = sc.queue[1:]
	if c.flee > 0 {
		s.fleeFrom(o, sc, target, c.flee)
		return
	}
	if c.wander > 0 {
		s.wander(o, sc, c.wander)
		return
	}
	first := target
	if c.self {
		first = o
	} else if c.pick != pickTarget {
		if first = s.picked(o, c.pick); first == nil {
			return
		}
	}
	s.castNpcSkill(o, c.skill, first)
}

// castNpcSkill has the npc use one of its skills, at the highest level its skill list declares.
func (s *Server) castNpcSkill(o *object, id int32, target creature) {
	tmpl := s.data.Skills[id]
	if tmpl == nil {
		return
	}
	var level int32
	for _, k := range s.data.NpcSkills[o.npc.ID] {
		if k.SkillID == id {
			level = max(level, k.Level)
		}
	}
	if level == 0 {
		level = o.npc.Level
	}
	(&skill{s: s, tmpl: tmpl, effector: o, level: level, first: target}).use()
}

// fleeFrom is flee_from: the npc runs straight away from its target for a while.
func (s *Server) fleeFrom(o *object, sc *npcScript, target creature, d time.Duration) {
	sc.fleeUntil = time.Now().Add(d)
	_, tx, ty, _ := target.loc()
	dx, dy := o.x-tx, o.y-ty
	n := float32(max(dist(o, target), 0.1))
	speed := float32(o.stats.current(data.Speed)) / 1000
	run := speed * float32(d.Seconds())
	o.move.follow = false
	o.move.distance = 0
	o.move.speed = speed
	o.move.setDirection(o.x+dx/n*run, o.y+dy/n*run, o.z)
	if !o.move.scheduled() {
		s.scheduleMove(o)
	}
}

// spawnBurst is an NTrap_A spawn: it wakes at the spot, casts its one skill on
// itself and is gone once the cast has played.
func (s *Server) spawnBurst(o *object, npcID int32, x, y, z float32) {
	t, list := s.data.Npcs[npcID], s.data.NpcSkills[npcID]
	if t == nil || len(list) == 0 || s.data.Skills[list[0].SkillID] == nil {
		return
	}
	life := time.Duration(s.data.Skills[list[0].SkillID].Duration)*time.Millisecond + 2*time.Second
	trap := s.spawnTempNpc(o, t, x, y, z, life)
	s.castNpcSkill(trap, list[0].SkillID, trap)
}

// callForHelp is broadcast_message 1016: the npc's allies within r join in on its target.
func (s *Server) callForHelp(o *object, target creature, r float32) {
	for _, other := range s.npcsNear(o, r) {
		if other.npc.Tribe == o.npc.Tribe || s.data.Tribes.IsSupport(other.npc.Tribe, o.npc.Tribe) {
			s.addHate(other, target, 1)
		}
	}
}

// wander is random_move: the npc runs to a random spot nearby for a while, its chase
// and attack paused as in a flee.
func (s *Server) wander(o *object, sc *npcScript, d time.Duration) {
	sc.fleeUntil = time.Now().Add(d)
	speed := float32(o.stats.current(data.Speed)) / 1000
	run := speed * float32(d.Seconds()) / 2
	angle := rand.Float64() * 2 * math.Pi
	o.move.follow = false
	o.move.distance = 0
	o.move.speed = speed
	o.move.setDirection(o.x+run*float32(math.Cos(angle)), o.y+run*float32(math.Sin(angle)), o.z)
	if !o.move.scheduled() {
		s.scheduleMove(o)
	}
}
