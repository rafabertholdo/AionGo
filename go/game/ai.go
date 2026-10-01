package game

import (
	"math"
	"math/rand/v2"
	"time"

	"aionlightning/game/data"
)

// The npcs' behaviour is AL-Game's AI: events happen to an npc, they move it between states, and each
// state has it want things (desires) that it does once a second while it is scheduled.

type aiEvent int

const (
	evAttacked aiEvent = iota
	evTiredAttacking
	evMostHatedChanged
	evNothingToDo
	evBackHome
	evRestoredHealth
	evSeePlayer
	evNotSeePlayer
	evTalk
	evRespawned
	evDied
	evDespawn
)

type aiState int

const (
	aiNone aiState = iota
	aiThinking
	aiTalking
	aiActive
	aiAttacking
	aiResting
	aiMovingToHome
)

// priority is AIState's: how strongly the state's desires want to run.
func (st aiState) priority() int {
	switch st {
	case aiThinking:
		return 5
	case aiTalking:
		return 4
	case aiActive:
		return 3
	case aiAttacking:
		return 2
	case aiResting, aiMovingToHome:
		return 1
	}
	return 0
}

// aiKind is which of AL-Game's AIs an npc has: they add handlers to each other's.
type aiKind int

const (
	aiPlain      aiKind = iota // NpcAi
	aiMonster                  // MonsterAi: fights back
	aiAggressive               // AggressiveAi: attacks on sight
)

type npcAI struct {
	s        *Server
	o        *object
	kind     aiKind
	state    aiState
	changed  bool
	desires  []desire
	task     *task
	talkTask *task
}

func newNpcAI(s *Server, o *object) *npcAI {
	a := &npcAI{s: s, o: o, kind: aiPlain}
	switch {
	case s.isAggressive(o):
		a.kind = aiAggressive
	case isMonster(o):
		a.kind = aiMonster
	}
	return a
}

func (a *npcAI) setState(st aiState) {
	if a.state != st {
		a.state = st
		a.changed = true
	}
}

func (a *npcAI) scheduled() bool { return a.task != nil && !a.task.cancelled }

// schedule runs the AI every second.
func (a *npcAI) schedule() {
	if !a.scheduled() {
		a.task = a.s.every(time.Second, time.Second, a.run)
	}
}

func (a *npcAI) stop() {
	if a.task != nil {
		a.task.cancel()
		a.task = nil
	}
}

// has is whether the npc wants a desire of the kind d is.
func (a *npcAI) has(d desire) bool {
	for _, other := range a.desires {
		if d.same(other) {
			return true
		}
	}
	return false
}

func (a *npcAI) clearDesires() {
	for _, d := range a.desires {
		d.clear()
	}
	a.desires = nil
}

// addDesire keeps one of each kind of desire, and the strongest first.
func (a *npcAI) addDesire(d desire) {
	for i, other := range a.desires {
		if d.same(other) {
			a.desires = append(a.desires[:i], a.desires[i+1:]...)
			break
		}
	}
	at := len(a.desires)
	for i, other := range a.desires {
		if other.power() < d.power() {
			at = i
			break
		}
	}
	a.desires = append(a.desires[:at], append([]desire{d}, a.desires[at:]...)...)
}

// run is AI.run: the desires that are due are done, and the state is looked at again if there is nothing left to want.
func (a *npcAI) run() {
	for i := 0; i < len(a.desires); {
		d := a.desires[i]
		if d.ready() && !d.handle(a) {
			d.clear()
			// The desire may have cleared the list itself.
			for j, other := range a.desires {
				if other == d {
					a.desires = append(a.desires[:j], a.desires[j+1:]...)
					i--
					break
				}
			}
		}
		i++
	}
	if len(a.desires) == 0 || a.changed {
		a.analyzeState()
	}
}

// handleEvent is NpcAi.handleEvent, with the handlers each kind of AI has.
func (a *npcAI) handleEvent(ev aiEvent) {
	if ev != evDied && a.o.dead {
		return
	}
	monster := a.kind != aiPlain
	switch ev {
	case evNothingToDo, evDied:
		a.setState(aiNone)
	case evRespawned:
		a.setState(aiActive)
		a.analyzeState()
	case evDespawn:
		a.handleEvent(evNothingToDo)
	case evTalk:
		if a.s.hasWalkRoutes(a.o) {
			a.setState(aiTalking)
		}
	case evAttacked:
		if monster {
			a.setState(aiAttacking)
			if !a.scheduled() {
				a.analyzeState()
			}
		}
	case evTiredAttacking, evMostHatedChanged, evRestoredHealth:
		if monster {
			a.setState(aiThinking)
		}
	case evBackHome:
		if monster {
			a.setState(aiResting)
		}
	case evSeePlayer:
		if a.kind != aiAggressive || a.state == aiAttacking {
			return // a fight isn't forgotten because someone else comes into view
		}
		busy := a.state == aiActive && a.scheduled()
		a.setState(aiActive)
		switch {
		case !a.scheduled():
			a.analyzeState()
		case busy && !a.has(&aggressionDesire{}):
			// AL-Game doesn't look again here, so a monster that is already walking never notices anyone.
			a.changed = true
		}
	case evNotSeePlayer:
		if a.kind == aiAggressive && len(a.o.watchers) == 0 {
			a.setState(aiThinking)
		}
	}
}

// analyzeState is AI.analyzeState: the state's handler.
func (a *npcAI) analyzeState() {
	a.changed = false
	s, o := a.s, a.o
	switch a.state {
	case aiActive:
		a.clearDesires()
		aggressive := false
		if a.kind == aiAggressive {
			for _, p := range o.watchers {
				if s.aggressiveTo(o, p) {
					aggressive = true
					break
				}
			}
		}
		switch {
		case aggressive:
			a.addDesire(&aggressionDesire{})
		case s.hasWalkRoutes(o):
			a.addDesire(newWalkDesire(s, o))
		}
		if len(a.desires) == 0 {
			a.handleEvent(evNothingToDo)
		} else {
			a.schedule()
		}
	case aiTalking:
		a.clearDesires()
		a.stop()
		a.talkTask.cancel()
		a.talkTask = s.later(time.Minute, func() { a.setState(aiThinking) })
	case aiNone:
		if a.kind == aiPlain {
			return
		}
		a.clearDesires()
		clear(o.aggro)
		a.stop()
	case aiAttacking:
		if a.kind == aiPlain {
			return
		}
		a.clearDesires()
		target := s.mostHated(o)
		if target == nil {
			return
		}
		o.targetID = target.cid()
		o.broadcast(s.lookAt(o), true)
		o.state |= stateWeapon
		o.broadcast(s.emote(o, emoteStartEmote2, target.cid()), true)
		o.broadcast(s.emote(o, emoteAttackMode, target.cid()), true)
		o.move.speed = float32(o.stats.current(data.Speed)) / 1000
		o.move.distance = float32(o.stats.current(data.AttackRange)) / 1000
		if len(s.data.NpcSkills[o.npc.ID]) != 0 {
			a.addDesire(&skillUseDesire{})
		}
		a.addDesire(&attackDesire{target: target.cid(), counter: 1})
		if o.stats.current(data.Speed) != 0 {
			a.addDesire(&moveToTargetDesire{target: target.cid()})
		}
		a.schedule()
	case aiThinking:
		if a.kind == aiPlain {
			return
		}
		a.clearDesires()
		switch {
		case s.mostHated(o) != nil:
			a.setState(aiAttacking)
		case !o.atHome():
			a.setState(aiMovingToHome)
		case o.hp != o.maxHP:
			a.setState(aiResting)
		default:
			a.setState(aiActive)
		}
	case aiResting:
		if a.kind == aiPlain {
			return
		}
		a.addDesire(&restoreHealthDesire{restore: o.maxHP / 5})
	case aiMovingToHome:
		if a.kind == aiPlain {
			return
		}
		a.clearDesires()
		o.targetID = 0
		o.broadcast(s.lookAt(o), true)
		clear(o.aggro)
		o.broadcast(s.emote(o, emoteStartEmote2, 0), true)
		o.broadcast(s.emote(o, emoteNeutralMode, 0), true)
		a.addDesire(&moveToHomeDesire{})
		a.schedule()
	}
}

// atHome is Npc.isAtSpawnLocation.
func (o *object) atHome() bool {
	return distance3D(o.homeX, o.homeY, o.homeZ, o.x, o.y, o.z) < 3
}

// desire is something an npc wants, done once its turn comes.
type desire interface {
	power() int
	// ready is true when it is the desire's turn: it counts the calls, and acts every interval-th.
	ready() bool
	// handle does it and returns false once the npc no longer wants it.
	handle(a *npcAI) bool
	clear()
	same(other desire) bool
}

// paced is what desires share: their strength and how often they act.
type paced struct {
	strength int
	counter  int
	interval int
}

func (p *paced) power() int { return p.strength }
func (p *paced) ready() bool {
	ready := p.counter%max(p.interval, 1) == 0
	p.counter++
	return ready
}
func (p *paced) clear() {}

// aggressionDesire is AggressionDesire: look for a player to attack among those it sees.
type aggressionDesire struct{ paced }

func (d *aggressionDesire) same(o desire) bool { _, ok := o.(*aggressionDesire); return ok }

func (d *aggressionDesire) handle(a *npcAI) bool {
	s, o := a.s, a.o
	d.interval, d.strength = 2, aiActive.priority()
	for _, p := range o.watchers {
		if p.dead || !inRange3D(o, p, float32(o.npc.SRange)) || !s.canSee(o, p) || !s.aggressiveTo(o, p) {
			continue
		}
		a.setState(aiNone)
		o.broadcast(attackPacket(o, p, 0, 633, 0, []attackResult{{status: statusNormalHit}}), true)
		id := p.ID
		s.later(time.Second, func() {
			if p := s.spawned[id]; p != nil {
				s.addHate(o, p, 1)
			}
		})
		break
	}
	return true
}

// canSee is Npc.canSeePlayer: an npc doesn't notice a player who isn't standing up, or who is hiding from it.
func (s *Server) canSee(o *object, p *player) bool {
	return p.state&stateActive != 0 && !(p.visualState == 1 && o.npc.Rank == "NORMAL")
}

// attackDesire is AttackDesire: hit the target when it is near, give up when it isn't.
type attackDesire struct {
	paced
	target      int32
	notPossible int
	counter     int
	o           *object
}

func (d *attackDesire) same(o desire) bool {
	other, ok := o.(*attackDesire)
	return ok && other.target == d.target
}

func (d *attackDesire) handle(a *npcAI) bool {
	s, o := a.s, a.o
	d.o, d.interval, d.strength = o, 2, aiAttacking.priority()
	target := s.creatureByID(d.target)
	if target == nil || target.isDead() {
		s.stopHating(o, d.target)
		o.ai.handleEvent(evTiredAttacking)
		return false
	}
	distance := dist(o, target)
	if distance > 50 {
		s.stopHating(o, d.target)
		o.ai.handleEvent(evTiredAttacking)
		return false
	}
	d.counter++
	if d.counter%2 == 0 && s.mostHated(o) != target {
		o.ai.handleEvent(evTiredAttacking)
		return false
	}
	if distance*1000 <= float64(o.stats.current(data.AttackRange)) {
		s.npcAttack(o, target)
		d.notPossible = 0
	} else {
		d.notPossible++
	}
	if d.notPossible > 10 {
		s.stopHating(o, d.target)
		o.ai.handleEvent(evTiredAttacking)
		return false
	}
	return true
}

// clear is AttackDesire.onClear: the npc puts its weapon away.
func (d *attackDesire) clear() {
	if d.o != nil {
		d.o.state &^= stateWeapon
	}
}

func (s *Server) stopHating(o *object, id int32) {
	if info := o.aggro[id]; info != nil {
		info.hate = 0
	}
}

// moveToTargetDesire is MoveToTargetDesire: follow the target.
type moveToTargetDesire struct {
	paced
	target int32
	o      *object
}

func (d *moveToTargetDesire) same(o desire) bool {
	other, ok := o.(*moveToTargetDesire)
	return ok && other.target == d.target
}

func (d *moveToTargetDesire) handle(a *npcAI) bool {
	s, o := a.s, a.o
	d.o, d.interval, d.strength = o, 1, aiAttacking.priority()
	target := s.creatureByID(d.target)
	if o.dead || target == nil || target.isDead() {
		return false
	}
	o.move.follow = true
	if !o.move.scheduled() {
		s.scheduleMove(o)
	}
	_, x, y, z := target.loc()
	return distance3D(o.x, o.y, o.z, x, y, z) <= 150
}

// clear is MoveToTargetDesire.onClear: the npc stops following.
func (d *moveToTargetDesire) clear() {
	if d.o != nil {
		d.o.move.stop()
	}
}

// moveToHomeDesire is MoveToHomeDesire: go back to where it spawned.
type moveToHomeDesire struct {
	paced
	o *object
}

func (d *moveToHomeDesire) same(o desire) bool { _, ok := o.(*moveToHomeDesire); return ok }

func (d *moveToHomeDesire) handle(a *npcAI) bool {
	s, o := a.s, a.o
	d.o, d.interval, d.strength = o, 1, aiMovingToHome.priority()
	if o.dead {
		return false
	}
	o.move.setDirection(o.homeX, o.homeY, o.homeZ)
	o.move.follow = false
	if !o.move.scheduled() {
		s.scheduleMove(o)
	}
	if distance3D(o.x, o.y, o.z, o.homeX, o.homeY, o.homeZ) < 2 {
		a.handleEvent(evBackHome)
		return false
	}
	return true
}

func (d *moveToHomeDesire) clear() {
	if d.o != nil {
		d.o.move.stop()
	}
}

// restoreHealthDesire is RestoreHealthDesire: heal a fifth of its life a second.
type restoreHealthDesire struct {
	paced
	restore int32
}

func (d *restoreHealthDesire) same(o desire) bool { _, ok := o.(*restoreHealthDesire); return ok }

func (d *restoreHealthDesire) handle(a *npcAI) bool {
	d.interval, d.strength = 1, aiResting.priority()
	o := a.o
	if o.dead {
		return false
	}
	a.s.increaseNpcHP(o, statusNaturalHP, d.restore)
	if o.hp == o.maxHP {
		a.handleEvent(evRestoredHealth)
		return false
	}
	return true
}

// walkDesire is WalkDesire: walk a route, or wander around the spawn point, resting between the steps.
type walkDesire struct {
	paced
	s             *Server
	o             *object
	route         data.Route
	walkingToNext bool
	target        int
	nextMove      time.Time
	random        bool
	point         [3]float32
	walkArea      float32
	halfArea      float32
	minRandomDist float32
}

func newWalkDesire(s *Server, o *object) *walkDesire {
	d := &walkDesire{s: s, o: o, random: o.randomWalk != 0, walkArea: 10, halfArea: 5, minRandomDist: 2}
	d.strength, d.interval = aiActive.priority(), 1
	if route, ok := s.data.Walkers[o.walker]; ok {
		d.route = route
		o.move.speed = o.npc.Stats.WalkSpeed
		o.move.walking = true
		o.broadcast(s.emote(o, emoteWalk, 0), true)
	} else if d.random {
		d.walkArea = max(5, float32(o.randomWalk))
		d.halfArea = d.walkArea / 2
		d.minRandomDist = min(d.walkArea/5, 2)
		d.point = [3]float32{o.x, o.y, o.z}
		o.move.speed = o.npc.Stats.WalkSpeed
		o.move.walking = true
		o.move.distance = d.minRandomDist
		o.broadcast(s.emote(o, emoteWalk, 0), true)
	}
	return d
}

func (d *walkDesire) same(o desire) bool { _, ok := o.(*walkDesire); return ok }

func (d *walkDesire) clear() { d.o.move.stop() }

func (d *walkDesire) handle(a *npcAI) bool {
	if d.route == nil && !d.random {
		return false
	}
	if d.walkingToNext {
		d.checkArrived()
	}
	d.walk()
	return true
}

func (d *walkDesire) step() [3]float32 {
	if d.route != nil {
		st := d.route[d.target]
		return [3]float32{st.X, st.Y, st.Z}
	}
	return d.point
}

func (d *walkDesire) checkArrived() {
	step := d.step()
	minDist := d.minRandomDist
	if d.route != nil {
		minDist = 2
	}
	if distance3D(d.o.x, d.o.y, d.o.z, step[0], step[1], step[2]) <= float64(minDist) {
		d.walkingToNext = false
		d.setNextTime()
	}
}

func (d *walkDesire) walk() {
	if d.walkingToNext || time.Now().Before(d.nextMove) {
		return
	}
	d.setNextPosition()
	d.walkingToNext = true
	step := d.step()
	d.o.move.setDirection(step[0], step[1], step[2])
	if !d.o.move.scheduled() {
		d.s.scheduleMove(d.o)
	}
}

func (d *walkDesire) setNextPosition() {
	if d.route == nil {
		d.point = [3]float32{
			d.o.homeX - d.halfArea + rand32()*d.walkArea,
			d.o.homeY - d.halfArea + rand32()*d.walkArea,
			d.o.homeZ,
		}
		return
	}
	switch {
	case d.random:
		d.target = int(rnd(0, int32(len(d.route)-1)))
	case d.target < len(d.route)-1:
		d.target++
	default:
		d.target = 0
	}
}

// AL-Game's npc movement delays, in seconds (npcmovement.properties).
const (
	minWalkDelay = 3
	maxWalkDelay = 15
)

func (d *walkDesire) setNextTime() {
	var delay int32
	switch {
	case d.route == nil:
		delay = rnd(minWalkDelay, maxWalkDelay)
	case d.random:
		delay = rnd(5, 60)
	default:
		delay = d.route[d.target].RestTime
	}
	d.nextMove = time.Now().Add(time.Duration(delay) * time.Second)
}

func rand32() float32 { return float32(rnd(0, math.MaxInt32-1)) / float32(math.MaxInt32) }

// skillUseDesire is SkillUseDesire: once a second, a random skill of the npc's list is used on its target, by
// the skill's own probability, unless it is casting.
type skillUseDesire struct{ paced }

func (d *skillUseDesire) same(o desire) bool { _, ok := o.(*skillUseDesire); return ok }

func (d *skillUseDesire) handle(a *npcAI) bool {
	s, o := a.s, a.o
	d.interval, d.strength = 1, 3 // AIState.USESKILL's priority
	if o.cast != nil {
		return true
	}
	skills := s.data.NpcSkills[o.npc.ID]
	pick := skills[rand.IntN(len(skills))]
	if rand.IntN(101) >= int(pick.Probability) {
		return true
	}
	tmpl := s.data.Skills[pick.SkillID]
	if tmpl == nil {
		return true
	}
	(&skill{s: s, tmpl: tmpl, effector: o, level: pick.Level, first: s.creatureByID(o.targetID)}).use()
	return true
}
