package game

import (
	"math"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// creature is a player or an npc: what combat and the npcs' behaviour treat alike (AL-Game's Creature).
// All of it is called with the world locked.
type creature interface {
	cid() int32
	clevel() int32
	loc() (world int32, x, y, z float32)
	cheading() byte
	hitPoints() (hp, maxHP int32)
	isDead() bool
	gameStats() *gameStats
	tribe() string
	// send sends a packet to the creature's client, if it has one.
	send(w *wire.Writer)
	// broadcast sends w to those who see the creature, and to it as well if toSelf.
	broadcast(w *wire.Writer, toSelf bool)
	// targetOf is the id of what the creature has selected, or 0.
	targetOf() int32
	setTarget(id int32)
	// mana is the current mana; only players have any.
	mana() int32
	casting() *skill
	setCasting(*skill)
	// moveCount is how many times the creature has set out to move.
	moveCount() int32
	fxc() *effectController
}

func (p *player) cid() int32    { return p.ID }
func (p *player) clevel() int32 { return int32(p.level) }
func (p *player) loc() (int32, float32, float32, float32) {
	return p.WorldID, p.X, p.Y, p.Z
}
func (p *player) cheading() byte { return byte(p.Heading) }
func (p *player) hitPoints() (int32, int32) {
	return p.life.HP, p.stats.current(data.MaxHP)
}
func (p *player) isDead() bool          { return p.dead }
func (p *player) gameStats() *gameStats { return p.stats }
func (p *player) send(w *wire.Writer)   { p.conn.send(w) }
func (p *player) targetOf() int32       { return p.targetID }
func (p *player) setTarget(id int32)    { p.targetID = id }
func (p *player) tribe() string {
	if race, _ := raceGender(p.Character); race == 1 {
		return "PC_DARK"
	}
	return "PC"
}

func (o *object) cid() int32 { return o.id }
func (o *object) clevel() int32 {
	if o.summonLevel != 0 {
		return o.summonLevel
	}
	if o.owner != nil {
		return int32(o.owner.level) // a trap or a servant is as strong as its maker
	}
	return o.npc.Level
}
func (o *object) loc() (int32, float32, float32, float32) {
	return o.worldID, o.x, o.y, o.z
}
func (o *object) cheading() byte            { return o.heading }
func (o *object) hitPoints() (int32, int32) { return o.hp, o.maxHP }
func (o *object) isDead() bool              { return o.dead }
func (o *object) gameStats() *gameStats     { return o.stats }
func (o *object) tribe() string             { return o.npc.Tribe }
func (o *object) send(*wire.Writer)         {}
func (o *object) targetOf() int32           { return o.targetID }
func (o *object) setTarget(id int32)        { o.targetID = id }
func (o *object) broadcast(w *wire.Writer, _ bool) {
	for _, p := range o.watchers {
		p.conn.send(w)
	}
}

// creatureByID finds the player or npc with the id; the caller holds the world lock.
func (s *Server) creatureByID(id int32) creature {
	if o := s.byID[id]; o != nil && o.npc != nil {
		return o
	}
	if p := s.spawned[id]; p != nil {
		return p
	}
	return nil
}

func distance3D(ax, ay, az, bx, by, bz float32) float64 {
	dx, dy, dz := float64(bx-ax), float64(by-ay), float64(bz-az)
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// dist is MathUtil.getDistance between two creatures.
func dist(a, b creature) float64 {
	_, ax, ay, az := a.loc()
	_, bx, by, bz := b.loc()
	return distance3D(ax, ay, az, bx, by, bz)
}

// inRange3D is MathUtil.isIn3dRange, for creatures in the same instance (or channel) of a map.
func inRange3D(a, b creature, r float32) bool {
	aw, ax, ay, az := a.loc()
	bw, bx, by, bz := b.loc()
	if aw != bw || instanceOf(a) != instanceOf(b) {
		return false
	}
	dx, dy, dz := bx-ax, by-ay, bz-az
	return dx*dx+dy*dy+dz*dz < r*r
}

func (p *player) mana() int32            { return p.life.MP }
func (p *player) casting() *skill        { return p.cast }
func (p *player) setCasting(sk *skill)   { p.cast = sk }
func (p *player) moveCount() int32       { return p.moves }
func (p *player) fxc() *effectController { return &p.fx }

func (o *object) mana() int32            { return o.stats.current(data.MaxMP) }
func (o *object) casting() *skill        { return o.cast }
func (o *object) setCasting(sk *skill)   { o.cast = sk }
func (o *object) moveCount() int32       { return o.moves }
func (o *object) fxc() *effectController { return &o.fx }

// instanceOf is the instance of its map a creature is in.
func instanceOf(c creature) int32 {
	switch x := c.(type) {
	case *player:
		return x.instance
	case *object:
		return x.instance
	}
	return 0
}
