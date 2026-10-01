package game

import (
	"time"

	"aionlightning/wire"
)

// Kisks: AL-Game's Kisk, KiskController, KiskService and ToyPetSpawnAction. A player sets one up with an item; those
// allowed to bind to it come back to life at it, a limited number of times, until it is destroyed or two hours pass.
// ponytail: the use mask 0 (and 1) is "the owner's race": AL-Game's canBind has the race test the wrong way round.

func init() {
	// A kisk answers a dialog request with the question to bind to it.
	next := handlers[cmShowDialog]
	handlers[cmShowDialog] = func(c *conn, r *wire.Reader) {
		peek := wire.NewReader(append([]byte(nil), r.Data...))
		id := peek.D()
		if peek.Err == nil && c.kiskDialog(id) {
			return
		}
		next(c, r)
	}
}

const (
	kiskLifetime                   = 2 * time.Hour
	questionBindToKisk             = 160018
	msgKiskNoAuthority             = 1300799
	msgKiskFarFromNpc              = 1300800
	msgKiskDestroyed               = 1300802
	msgKiskRemoved                 = 1300803
	msgKiskFlying                  = 1300806
	msgKiskRegistered              = 1390159
	msgKiskAlreadyRegistered       = 1390161
	msgKiskAttacked                = 1390166
	msgKiskFull                    = 1400247
	levelUpdateBind                = 2
	kiskDecay                      = 3 * time.Second
	kiskHeadingTurn          uint8 = 60
)

// kisk is what a Kisk has besides being an npc.
type kisk struct {
	ownerID        int32
	ownerName      string
	ownerRace      string
	useMask        int32
	maxMembers     int32
	resurrects     int32
	maxResurrects  int32
	spawned        time.Time
	members        map[int32]bool // player ids
	despawnTimeout *task
}

func (k *kisk) remaining() int32 {
	return max(0, int32((kiskLifetime-time.Since(k.spawned))/time.Second))
}

// spawnKisk is SpawnEngine.spawnKisk and ToyPetSpawnAction.act: the kisk stands where the owner is, and goes in two hours.
func (s *Server) spawnKisk(owner *player, npcID int32) *object {
	t := s.data.Npcs[npcID]
	if t == nil || t.Kisk == nil {
		return nil
	}
	o := s.spawnOwnedAt(owner, npcID, 0, byte((uint8(owner.Heading)+kiskHeadingTurn)%120))
	o.kisk = &kisk{ownerID: owner.ID, ownerName: owner.Name, ownerRace: owner.Race, useMask: t.Kisk.UseMask,
		maxMembers: t.Kisk.Members, resurrects: t.Kisk.Resurrects, maxResurrects: t.Kisk.Resurrects, spawned: time.Now(),
		members: map[int32]bool{}}
	o.timers = []*task{s.later(kiskLifetime, func() { s.kiskRemove(o, msgKiskRemoved) })}
	return o
}

// canBind is Kisk.canBind.
func (s *Server) canBind(o *object, p *player) bool {
	k := o.kisk
	if p.ID != k.ownerID {
		owner := s.spawned[k.ownerID]
		switch k.useMask {
		case 0, 1:
			if p.Race != k.ownerRace {
				return false
			}
		case 2:
			if owner == nil || owner.legion == nil || p.legion != owner.legion {
				return false
			}
		case 4:
			if owner == nil || !(p.group != nil && p.group == owner.group || p.alliance != nil && p.alliance == owner.alliance &&
				p.alliance.slot(p) == p.alliance.slot(owner)) {
				return false
			}
		case 5:
			if owner == nil || p.alliance == nil || p.alliance != owner.alliance {
				return false
			}
		default: // 3: the owner alone
			return false
		}
	}
	return int32(len(k.members)) < k.maxMembers
}

// kiskDialog is KiskController.onDialogRequest; it says whether the object was a kisk.
func (c *conn) kiskDialog(id int32) bool {
	p := c.player
	if p == nil {
		return false
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	o := p.seen[id]
	if o == nil || o.kisk == nil {
		return false
	}
	switch {
	case p.kisk == o:
		p.conn.send(systemMessage(msgKiskAlreadyRegistered))
	case s.canBind(o, p):
		asked := p.putRequest(questionBindToKisk, func(accepted bool) {
			if !accepted || o.dead {
				return
			}
			if !s.canBind(o, p) {
				p.conn.send(systemMessage(msgKiskNoAuthority))
				return
			}
			s.bindToKisk(o, p)
		})
		if asked {
			p.conn.send(questionWindow(questionBindToKisk, p.ID))
		}
	case int32(len(o.kisk.members)) >= o.kisk.maxMembers:
		p.conn.send(systemMessage(msgKiskFull))
	default:
		p.conn.send(systemMessage(msgKiskNoAuthority))
	}
	return true
}

// bindToKisk is KiskService.onBind: the player leaves the kisk it was bound to and is bound to this one.
func (s *Server) bindToKisk(o *object, p *player) {
	if old := p.kisk; old != nil {
		delete(old.kisk.members, p.ID)
		s.kiskUpdate(old)
	}
	o.kisk.members[p.ID] = true
	if s.kisks == nil {
		s.kisks = map[int32]*object{}
	}
	s.kisks[p.ID] = o
	p.kisk = o
	s.kiskUpdate(o)
	p.conn.send(s.bindPoint(p))
	p.conn.send(systemMessage(msgKiskRegistered))
	update := wire.Packet(smLevelUpdate)
	update.D(p.ID)
	update.H(levelUpdateBind)
	update.H(uint16(p.level))
	update.H(0)
	p.broadcast(update, true)
}

// kiskPacket is SM_KISK_UPDATE.
func kiskPacket(o *object) *wire.Writer {
	k := o.kisk
	w := wire.Packet(smKiskUpdate)
	w.D(o.id)
	w.D(k.useMask)
	w.D(int32(len(k.members)))
	w.D(k.maxMembers)
	w.D(k.resurrects)
	w.D(k.maxResurrects)
	w.D(k.remaining())
	return w
}

// kiskUpdate is Kisk.broadcastKiskUpdate: the members that don't see it, and those of its race that do, are told how it is.
func (s *Server) kiskUpdate(o *object) {
	for id := range o.kisk.members {
		if m := s.spawned[id]; m != nil && o.watchers[id] == nil {
			m.conn.send(kiskPacket(o))
		}
	}
	for _, p := range o.watchers {
		if p.Race == o.kisk.ownerRace {
			p.conn.send(kiskPacket(o))
		}
	}
}

// kiskLogin is KiskService.onLogin: a player who comes back is still bound to its kisk.
func (s *Server) kiskLogin(p *player, tell bool) {
	o := s.kisks[p.ID]
	if o == nil || o.dead {
		delete(s.kisks, p.ID)
		return
	}
	p.kisk = o
	if tell {
		p.conn.send(kiskPacket(o))
	}
}

// kiskTell sends the members a message.
func (s *Server) kiskTell(o *object, w *wire.Writer) {
	for id := range o.kisk.members {
		if m := s.spawned[id]; m != nil {
			m.conn.send(w)
		}
	}
}

// unbindKisk is KiskService.removeKisk: nobody is bound to it any more, and those who were dead have nowhere to rise.
func (s *Server) unbindKisk(o *object) {
	for id := range o.kisk.members {
		delete(s.kisks, id)
		m := s.spawned[id]
		if m == nil {
			continue
		}
		m.kisk = nil
		m.conn.send(s.bindPoint(m))
		if m.dead {
			die := wire.Packet(smDie)
			die.Bool(false)
			die.Bool(false)
			die.D(0)
			m.conn.send(die)
		}
	}
	o.kisk.members = map[int32]bool{}
}

// kiskRemove is KiskController.onDespawn: the kisk goes at once.
func (s *Server) kiskRemove(o *object, msg int32) {
	if o.kisk.despawnTimeout != nil {
		return
	}
	s.kiskTell(o, systemMessage(msg))
	s.unbindKisk(o)
	s.despawnOwned(o)
}

// kiskDied is KiskController.onDie: it falls, and is gone in three seconds.
func (s *Server) kiskDied(o *object, attacker creature) {
	o.state |= stateDead
	var by int32
	if attacker != nil {
		by = attacker.cid()
	}
	o.broadcast(emotionPacket(o.id, emoteDie, o.state, 0, 0, by, 0, 0, 0, 0, 0, 0), true)
	s.kiskTell(o, systemMessage(msgKiskDestroyed))
	s.unbindKisk(o)
	o.kisk.despawnTimeout = s.later(kiskDecay, func() {
		o.dead = false // despawnOwned skips the dead
		s.despawnOwned(o)
	})
}

// kiskRevive is ReviveController.kiskRevive: the player rises at its kisk, which one resurrection is used up of.
func (s *Server) kiskRevive(p *player) {
	o := p.kisk
	if o == nil || o.dead {
		return
	}
	s.reviveAt(p, 25, 25)
	p.conn.send(systemMessage(msgRevive))
	p.conn.send(s.statsInfo(p))
	p.conn.send(s.playerInfo(p, false))
	s.teleportToInstance(p, o.worldID, o.instance, o.x, o.y, o.z, o.heading, 0)
	o.kisk.resurrects--
	if o.kisk.resurrects <= 0 {
		s.kiskRemove(o, msgKiskRemoved)
	} else {
		s.kiskUpdate(o)
	}
}

// kiskRemaining is what SM_DIE tells a dead player of its kisk: how long it has left.
func kiskRemaining(p *player) int32 {
	if p.kisk == nil {
		return 0
	}
	return p.kisk.kisk.remaining()
}

// canSpawnKisk is ToyPetSpawnAction.canAct.
func (s *Server) canSpawnKisk(p *player) bool {
	if p.inState(stateFlying) {
		p.conn.send(systemMessage(msgKiskFlying))
		return false
	}
	if p.instance != 0 {
		p.conn.send(systemMessage(msgKiskFarFromNpc))
		return false
	}
	return true
}
