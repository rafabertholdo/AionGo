package game

import (
	"math/rand/v2"
	"time"

	"aionlightning/wire"
)

// Rifts: AL-Game's RiftSpawnManager and RiftController. Every 100 minutes one rift of each region's seven opens at
// random in Eltnen, Heiron, Morheim and Beluslan, with its exit in the other race's map; it stays 26 minutes, and
// closes when as many players as its entries say have gone through.
// ponytail: only the first channel of each map; AL-Game leaves a rift standing when its 26 minutes end (its despawn
// test is backwards), here it goes. The level range is only what the client is told, as in AL-Game.

func init() {
	// A rift answers a dialog request with the question to go through it.
	next := handlers[cmShowDialog]
	handlers[cmShowDialog] = func(c *conn, r *wire.Reader) {
		peek := wire.NewReader(append([]byte(nil), r.Data...))
		id := peek.D()
		if peek.Err == nil && c.riftDialog(id) {
			return
		}
		next(c, r)
	}
}

const (
	riftPeriod      = 100 * time.Minute
	riftFirst       = 10 * time.Second
	riftLifetime    = 26 * time.Minute
	riftDecay       = 3 * time.Second
	riftMinLevel    = 25
	riftUnknown     = 6793
	questionUseRift = 160019
)

// riftKind is a RiftEnum: the master rift's anchor and its slave's, how many may go through and up to what level.
type riftKind struct {
	master, slave string
	entries       int32
	maxLevel      int32
	destination   string // the race that may use it
}

// riftKinds lists the 28 RiftEnum values in order: seven per region, four regions.
var riftKinds = func() []riftKind {
	small := [7][2]int32{{12, 28}, {20, 32}, {35, 36}, {35, 37}, {45, 40}, {50, 40}, {50, 45}}
	big := [7][2]int32{{24, 35}, {36, 35}, {48, 46}, {48, 40}, {60, 50}, {60, 50}, {72, 50}}
	var kinds []riftKind
	for _, r := range []struct {
		from, to, race string
		table          [7][2]int32
	}{{"ELTNEN", "MORHEIM", "ASMODIANS", small}, {"HEIRON", "BELUSLAN", "ASMODIANS", big},
		{"MORHEIM", "ELTNEN", "ELYOS", small}, {"BELUSLAN", "HEIRON", "ELYOS", big}} {
		for i, e := range r.table {
			letter := string(rune('A' + i))
			kinds = append(kinds, riftKind{r.from + "_" + letter + "M", r.to + "_" + letter + "S", e[0], e[1], r.race})
		}
	}
	return kinds
}()

// rift is what a RiftController has besides its npc: the master has the slave to go to, and counts who went.
type rift struct {
	kind      riftKind
	master    bool
	slave     *object
	used      int32
	accepting bool
}

// startRifts is RiftSpawnManager.startRiftPool.
func (s *Server) startRifts() {
	s.every(riftFirst, riftPeriod, func() {
		for region := range 4 {
			s.spawnRift(riftKinds[region*7+rand.IntN(7)])
		}
	})
}

// riftSpot finds the spawn of an anchor in the static data.
func (s *Server) riftSpot(anchor string) (npcID, world int32, spot [4]float32, ok bool) {
	for _, groups := range s.data.Spawns {
		for _, g := range groups {
			if g.Handler == "RIFT" && g.Anchor == anchor && len(g.Spots) > 0 {
				sp := g.Spots[0]
				return g.NpcID, g.Map, [4]float32{sp.X, sp.Y, sp.Z, float32(sp.Heading)}, true
			}
		}
	}
	return
}

// spawnRift is RiftSpawnManager.spawnRift: the slave, then the master that leads to it, each with a lifetime.
func (s *Server) spawnRift(kind riftKind) *object {
	masterID, masterWorld, m, ok := s.riftSpot(kind.master)
	slaveID, slaveWorld, sl, ok2 := s.riftSpot(kind.slave)
	if !ok || !ok2 || s.data.Npcs[masterID] == nil || s.data.Npcs[slaveID] == nil {
		return nil
	}
	s.log.Info("spawning rift", "rift", kind.master)
	slave := s.spawnRiftObject(slaveID, slaveWorld, sl, &rift{kind: kind})
	master := s.spawnRiftObject(masterID, masterWorld, m, &rift{kind: kind, master: true, slave: slave, accepting: true})
	for _, p := range s.spawned {
		if p.WorldID == master.worldID {
			p.conn.send(riftAnnounce())
		}
	}
	return master
}

func (s *Server) spawnRiftObject(npcID, world int32, at [4]float32, r *rift) *object {
	o := &object{id: s.ids.nextID(), worldID: world, x: at[0], y: at[1], z: at[2], heading: byte(at[3]),
		npc: s.data.Npcs[npcID], homeX: at[0], homeY: at[1], homeZ: at[2], rift: r}
	s.initNpc(o)
	s.byID[o.id] = o
	s.addObject(o)
	o.timers = []*task{s.later(riftLifetime, func() { s.despawnOwned(o) })}
	return o
}

// riftDialog is RiftController.onDialogRequest; it says whether the object was a rift.
func (c *conn) riftDialog(id int32) bool {
	p := c.player
	if p == nil {
		return false
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	o := p.seen[id]
	if o == nil || o.rift == nil {
		return false
	}
	if !o.rift.master || !o.rift.accepting {
		return true
	}
	if p.putRequest(questionUseRift, func(accepted bool) { s.useRift(o, p, accepted) }) {
		p.conn.send(questionWindow(questionUseRift, 0))
	}
	return true
}

// useRift is the RequestResponseHandler of RiftController: the player goes to the slave rift, and the last of the
// entries closes both.
func (s *Server) useRift(o *object, p *player, accepted bool) {
	r := o.rift
	if !accepted || !r.accepting || o.dead || r.slave == nil {
		return
	}
	s.teleportTo(p, r.slave.worldID, r.slave.x, r.slave.y, r.slave.z, 0, 0)
	r.used++
	if r.used >= r.kind.entries {
		r.accepting = false
		s.later(riftDecay, func() {
			s.despawnOwned(o)
			s.despawnOwned(r.slave)
		})
	}
	o.broadcast(riftStatus(o), true)
}

// riftStatus is SM_RIFT_STATUS.
func riftStatus(o *object) *wire.Writer {
	w := wire.Packet(smRiftStatus)
	w.D(o.id)
	w.D(o.rift.used)
	w.D(o.rift.kind.entries)
	w.D(riftUnknown)
	w.D(riftMinLevel)
	w.D(o.rift.kind.maxLevel)
	return w
}

// riftAnnounce is SM_RIFT_ANNOUNCE.
func riftAnnounce() *wire.Writer {
	w := wire.Packet(smRiftAnnounce)
	w.D(0)
	w.D(1)
	w.D(0)
	return w
}

// riftLogin is RiftSpawnManager.sendRiftStatus: a player who enters a map is told of the rifts open in it.
func (s *Server) riftLogin(p *player) {
	for _, o := range s.byID {
		if o.rift != nil && o.rift.master && !o.dead && o.worldID == p.WorldID {
			p.conn.send(riftAnnounce())
		}
	}
}
