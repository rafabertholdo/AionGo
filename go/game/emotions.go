package game

import (
	"aionlightning/game/data"
	"aionlightning/wire"
)

func init() {
	handlers[cmEmotion] = (*conn).emotion
	handlers[cmTargetSelect] = (*conn).targetSelect
}

// Creature states (CreatureState): bits of the state word players and npcs send.
const (
	stateActive     = 1
	stateFlying     = 1 << 1
	stateResting    = 1 << 2
	stateChair      = 3 << 1
	stateLooting    = 3 << 2
	stateWeapon     = 1 << 5
	stateWalking    = 1 << 6
	statePowershard = 1 << 7
)

// Emotion types (EmotionType).
const (
	emoteSelectTarget   = 0
	emoteJump           = 1
	emoteSit            = 2
	emoteStand          = 3
	emoteChairSit       = 4
	emoteChairUp        = 5
	emoteStartFlyTele   = 6
	emoteLandFlyTele    = 7
	emoteFly            = 11
	emoteLand           = 12
	emoteDie            = 16
	emoteResurrect      = 17
	emoteEmote          = 19
	emoteAttackMode     = 22
	emoteNeutralMode    = 23
	emoteWalk           = 24
	emoteRun            = 25
	emoteSwitchDoor     = 29
	emoteStartEmote     = 30
	emoteOpenShop       = 31
	emoteCloseShop      = 32
	emoteStartEmote2    = 33
	emotePowershardOn   = 34
	emotePowershardOff  = 35
	emoteAttackMode2    = 36
	emoteNeutralMode2   = 37
	emoteStartLoot      = 38
	emoteEndLoot        = 39
	emoteStartQuestLoot = 40
	emoteEndQuestLoot   = 41
)

// System messages the emotions answer with.
const (
	msgNoPowershard         = 1300490
	msgActivatePowershard   = 1300491
	msgDeactivatePowershard = 1300492
)

// emotion is CM_EMOTION: the player sits, stands, draws or puts away its weapon, walks, runs, jumps
// or emotes, and those who see it, and it, are told.
func (c *conn) emotion(r *wire.Reader) {
	p := c.player
	if p == nil {
		return
	}
	kind := r.C()
	var emote int32
	var x, y, z float32
	var heading byte
	switch kind {
	case emoteEmote:
		emote = int32(r.H())
	case emoteChairSit, emoteChairUp:
		x, y, z, heading = r.F(), r.F(), r.F(), r.C()
	}
	if r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	switch kind {
	case emoteSelectTarget:
		return
	case emoteSit:
		p.state |= stateResting
	case emoteStand:
		p.state &^= stateResting
	case emoteChairSit:
		p.state &^= stateActive
		p.state |= stateChair
	case emoteChairUp:
		p.state &^= stateChair
		p.state |= stateActive
	case emoteFly:
		if !p.flightAllowed() {
			p.conn.send(systemMessage(msgFlyingForbiddenHere))
			return
		}
		s.startFly(p)
	case emoteLand:
		s.endFly(p)
	case emoteLandFlyTele:
		s.endFlightTeleport(p)
	case emoteAttackMode, emoteAttackMode2:
		p.state |= stateWeapon
	case emoteNeutralMode, emoteNeutralMode2:
		p.state &^= stateWeapon
	case emoteWalk:
		p.state |= stateWalking
	case emoteRun:
		p.state &^= stateWalking
	case emotePowershardOn:
		if !p.hasPowershard() {
			p.conn.send(systemMessage(msgNoPowershard))
			return
		}
		p.conn.send(systemMessage(msgActivatePowershard))
		p.state |= statePowershard
	case emotePowershardOff:
		p.conn.send(systemMessage(msgDeactivatePowershard))
		p.state &^= statePowershard
	}
	p.broadcast(s.playerEmotion(p, kind, emote, x, y, z, heading), true)
}

// hasPowershard is Equipment.isPowerShardEquipped.
func (p *player) hasPowershard() bool {
	for _, item := range p.equipment {
		if item.Slot&(1<<13|1<<14) != 0 {
			return true
		}
	}
	return false
}

// playerEmotion is SM_EMOTION for a player.
func (s *Server) playerEmotion(p *player, kind byte, emote int32, x, y, z float32, heading byte) *wire.Writer {
	return s.playerEmotionTo(p, kind, emote, p.targetID, x, y, z, heading)
}

// playerEmotionTo is playerEmotion aimed at another target than the player's own, like the corpse it loots.
func (s *Server) playerEmotionTo(p *player, kind byte, emote int32, target int32, x, y, z float32, heading byte) *wire.Writer {
	speed := float32(p.stats.current(data.Speed)) / speedScale
	if p.inState(stateFlying) {
		speed = float32(p.stats.current(data.FlySpeed)) / speedScale
	}
	return emotionPacket(p.ID, kind, p.state, speed, emote, target, x, y, z, heading,
		uint16(p.stats.base(data.AttackSpeed)), uint16(p.stats.current(data.AttackSpeed)))
}

// emotionPacket is SM_EMOTION: who did what, with the extras that kind of emotion carries.
func emotionPacket(id int32, kind byte, state uint16, speed float32, emote int32, target int32,
	x, y, z float32, heading byte, baseAttackSpeed, attackSpeed uint16) *wire.Writer {
	w := wire.Packet(smEmotion)
	w.D(id)
	w.C(kind)
	if kind == emoteSwitchDoor {
		w.H(9)
		w.D(0)
		return w
	}
	w.H(state)
	if kind == emoteWalk {
		speed -= speed * 75 / 100
	}
	w.F(speed)
	switch kind {
	case emoteChairSit, emoteChairUp:
		w.F(x)
		w.F(y)
		w.F(z)
		w.C(heading)
	case emoteStartFlyTele:
		w.D(emote)
	case emoteDie, emoteStartLoot, emoteEndLoot, emoteStartQuestLoot, emoteEndQuestLoot:
		w.D(target)
	case emoteEmote:
		w.D(target)
		w.H(uint16(emote))
		w.C(1)
	case emoteStartEmote, emoteStartEmote2:
		w.H(baseAttackSpeed)
		w.H(attackSpeed)
	case emoteSelectTarget, emoteJump, emoteSit, emoteStand, emoteLandFlyTele, emoteFly, emoteLand, emoteResurrect,
		emoteAttackMode, emoteNeutralMode, emoteRun, emoteOpenShop, emoteCloseShop, emotePowershardOn,
		emotePowershardOff, emoteAttackMode2, emoteNeutralMode2, emoteWalk:
	default:
		if target != 0 {
			w.D(target)
		}
	}
	return w
}

// targetSelect is CM_TARGET_SELECT: the player selects an object, or the object's own target, or none.
func (c *conn) targetSelect(r *wire.Reader) {
	p := c.player
	id, kind := r.D(), r.C()
	if p == nil || r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if _, known := s.lookup(id); known {
		if kind == 1 {
			t, _ := s.lookup(id)
			if t.target == 0 {
				return
			}
			p.targetID = t.target
		} else {
			p.targetID = id
		}
	} else {
		p.targetID = 0
	}
	p.conn.send(s.targetSelected(p))
	target := wire.Packet(smTargetUpdate)
	target.D(p.ID)
	target.D(p.targetID)
	p.broadcast(target, false)
}

// creatureView is what a target's level, life and target look like to a player who selects it.
type creatureView struct {
	level, maxHP, hp int32
	target           int32
}

// lookup finds the player or object with the id. The caller holds visMu.
func (s *Server) lookup(id int32) (creatureView, bool) {
	if o := s.byID[id]; o != nil {
		if o.npc == nil {
			return creatureView{level: 1, maxHP: 1, hp: 1}, true
		}
		return creatureView{level: o.npc.Level, maxHP: o.maxHP, hp: o.hp, target: o.targetID}, true
	}
	if p := s.spawned[id]; p != nil {
		return creatureView{level: int32(p.level), maxHP: p.stats.current(data.MaxHP), hp: p.life.HP, target: p.targetID}, true
	}
	return creatureView{}, false
}

// targetSelected is SM_TARGET_SELECTED: what the player selected, with its level and life.
func (s *Server) targetSelected(p *player) *wire.Writer {
	view, ok := s.lookup(p.targetID)
	if !ok {
		view = creatureView{level: 1, maxHP: 1, hp: 1}
	}
	w := wire.Packet(smTargetSelected)
	w.D(p.targetID)
	w.H(uint16(view.level))
	w.D(view.maxHP)
	w.D(view.hp)
	return w
}
