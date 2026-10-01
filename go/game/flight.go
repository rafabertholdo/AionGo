package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// Flying and gliding: AL-Game's FlyController, and the fly time (FP) that runs out while a player is in the air and
// comes back on the ground (FpReduceTask and FpRestoreTask).
// Zones that forbid flying are in zones.go. Flight teleporters are at the bottom: the paths are in the client.

const (
	stateGliding   = 1 << 9
	fpReduceEvery  = time.Second
	fpRestoreEvery = 2 * time.Second
	fpFirstDelay   = 2 * time.Second
)

// inState is Creature.isInState: all the state's bits are set.
func (p *player) inState(mask uint16) bool { return p.state&mask == mask }

// startFly is FlyController.startFly.
func (s *Server) startFly(p *player) {
	p.state |= stateFlying
	p.flyState = 1
	s.triggerFpReduce(p)
	p.broadcast(s.playerEmotion(p, emoteStartEmote2, 0, 0, 0, 0, 0), true)
	p.conn.send(s.statsInfo(p))
}

// endFly is FlyController.endFly: the player lands.
func (s *Server) endFly(p *player) {
	if !p.inState(stateFlying) && !p.inState(stateGliding) {
		return
	}
	p.broadcast(s.playerEmotion(p, emoteLand, 0, 0, 0, 0, 0), true)
	p.state &^= stateFlying | stateGliding
	p.flyState = 0
	// This is probably what changes the fly speed back into the speed.
	p.broadcast(s.playerEmotion(p, emoteStartEmote2, 0, 0, 0, 0, 0), true)
	p.conn.send(s.statsInfo(p))
	s.triggerFpRestore(p)
}

// switchToGliding is FlyController.switchToGliding.
func (s *Server) switchToGliding(p *player) {
	if p.inState(stateGliding) {
		return
	}
	p.state |= stateGliding
	if p.flyState == 0 {
		s.triggerFpReduce(p)
	}
	p.flyState = 2
	p.conn.send(s.statsInfo(p))
}

// stopGliding is FlyController.onStopGliding.
func (s *Server) stopGliding(p *player) {
	if !p.inState(stateGliding) {
		return
	}
	p.state &^= stateGliding
	if p.inState(stateFlying) {
		p.flyState = 1
	} else {
		p.flyState = 0
		s.triggerFpRestore(p)
	}
	p.conn.send(s.statsInfo(p))
}

func (s *Server) cancelFp(p *player) {
	p.fpTask.cancel()
	p.fpTask = nil
}

func (s *Server) sendFp(p *player) {
	p.conn.send(statUpdate(smFlyTime, p.life.FP, p.stats.current(data.FlyTime)))
}

// triggerFpReduce is PlayerLifeStats.triggerFpReduce: the fly time runs out, a point a second.
func (s *Server) triggerFpReduce(p *player) {
	s.cancelFp(p)
	if p.dead || p.life.FP == 0 && p.flyState == 0 {
		return
	}
	p.fpTask = s.every(fpFirstDelay, fpReduceEvery, func() {
		switch {
		case p.dead:
			s.cancelFp(p)
		case p.life.FP == 0:
			if p.flyState > 0 {
				s.endFly(p)
			} else {
				s.triggerFpRestore(p)
			}
		default:
			p.life.FP--
			s.sendFp(p)
		}
	})
}

// triggerFpRestore is PlayerLifeStats.triggerFpRestore: on the ground the fly time comes back, a point in two seconds.
func (s *Server) triggerFpRestore(p *player) {
	s.cancelFp(p)
	if p.dead || p.life.FP >= p.stats.current(data.FlyTime) {
		return
	}
	p.fpTask = s.every(fpFirstDelay, fpRestoreEvery, func() {
		if p.dead || p.life.FP >= p.stats.current(data.FlyTime) {
			s.cancelFp(p)
			return
		}
		p.life.FP++
		s.sendFp(p)
	})
}

func init() {
	handlers[cmFlightTeleport] = (*conn).flightTeleport
}

// usingFlyTeleport is Player.isUsingFlyTeleport.
func (p *player) usingFlyTeleport() bool { return p.inState(stateFlying) && p.flightTeleportID != 0 }

// startFlightTeleport is TeleportService.flightTeleport after the kinah is paid: the player is flying the client's
// path (teleportid of npc_teleporter.xml), and everyone who sees it is told. The server has no path of its own: the
// client sends its positions (CM_FLIGHT_TELEPORT) and lands with LAND_FLYTELEPORT.
func (s *Server) startFlightTeleport(p *player, path int32) {
	p.state |= stateFlying
	p.state &^= stateActive
	p.flightTeleportID = path
	p.broadcast(s.playerEmotion(p, emoteStartFlyTele, uint16(path), 0, 0, 0, 0), true)
}

// endFlightTeleport is PlayerController.onFlyTeleportEnd: the player has arrived.
func (s *Server) endFlightTeleport(p *player) {
	p.state &^= stateFlying
	p.flightTeleportID, p.flightDistance = 0, 0
	p.state |= stateActive
	s.refreshZone(p)
}

// flightTeleport is CM_FLIGHT_TELEPORT: where the player is along its path.
func (c *conn) flightTeleport(r *wire.Reader) {
	r.D() // map id
	x, y, z := r.F(), r.F(), r.F()
	r.C() // location id
	distance := r.D()
	p := c.player
	if p == nil || r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if !p.spawned || !p.inState(stateFlying) || p.flightTeleportID == 0 {
		return
	}
	p.flightDistance = distance
	s.updatePosition(p, x, y, z, 0)
}
