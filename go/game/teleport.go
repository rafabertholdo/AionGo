package game

import (
	"cmp"
	"strconv"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

func init() {
	handlers[cmRevive] = (*conn).revive
}

// teleportDefaultDelay is TeleportService.TELEPORT_DEFAULT_DELAY: how long the teleport animation lasts.
const teleportDefaultDelay = 2200 * time.Millisecond

// itemUsageAnimation is SM_ITEM_USAGE_ANIMATION: the bar that fills while a player teleports or uses an item.
func itemUsageAnimation(player, item, itemID int32, ms int32, end byte, unknown int32) *wire.Writer {
	w := wire.Packet(smItemUsageAnimation)
	w.D(player)
	w.D(player)
	w.D(item)
	w.D(itemID)
	w.D(ms)
	w.C(end)
	w.C(1)
	w.C(0)
	w.D(unknown)
	return w
}

// teleportTo is TeleportService.teleportTo: the player is taken to the place after a delay of animation.
func (s *Server) teleportTo(p *player, world int32, x, y, z float32, heading byte, delay time.Duration) bool {
	inst := int32(0)
	if world == p.WorldID {
		inst = p.instance
	}
	return s.teleportToInstance(p, world, inst, x, y, z, heading, delay)
}

// teleportToInstance is teleportTo into a given instance of the map.
func (s *Server) teleportToInstance(p *player, world, inst int32, x, y, z float32, heading byte, delay time.Duration) bool {
	if p.dead || !p.spawned {
		return false
	}
	if delay == 0 {
		s.changePosition(p, world, inst, x, y, z, heading)
		return true
	}
	p.conn.send(itemUsageAnimation(p.ID, 0, 0, int32(delay/time.Millisecond), 0, 0))
	s.later(delay, func() {
		if p.dead || !p.spawned {
			return
		}
		p.conn.send(itemUsageAnimation(0, 0, 0, 0, 1, 0))
		s.changePosition(p, world, inst, x, y, z, heading)
	})
	return true
}

// changePosition is TeleportService.changePosition: the player leaves where it is and stands in the new place;
// in another map the client is told to load it, and asks to appear in it when it has.
func (s *Server) changePosition(p *player, world, inst int32, x, y, z float32, heading byte) {
	s.despawnLocked(p)
	previous, previousInstance := p.WorldID, p.instance
	p.WorldID, p.instance, p.X, p.Y, p.Z, p.Heading = world, inst, x, y, z, int32(heading)
	if previous == world && previousInstance == inst {
		p.conn.send(s.statsInfo(p))
		p.conn.send(s.playerInfo(p, false))
		s.spawnLocked(p)
		s.refreshZone(p)
	} else {
		s.startProtection(p)
		p.conn.send(s.channelInfo(p))
		p.conn.send(playerSpawn(p))
	}
	s.startProtection(p)
}

// bindLocation is where a player goes when it comes back from death: its bind point, or where its race starts.
func (s *Server) bindLocation(p *player) data.Location {
	if at, ok := s.data.BindPoints[p.BindPoint]; p.BindPoint != 0 && ok {
		return at
	}
	return s.startLocation(p.Race)
}

// moveToBind is TeleportService.moveToBindLocation.
func (s *Server) moveToBind(p *player, useTeleport bool, delay time.Duration) {
	at := s.bindLocation(p)
	if useTeleport {
		s.teleportTo(p, at.MapID, at.X, at.Y, at.Z, byte(p.Heading), delay)
		return
	}
	p.WorldID, p.instance, p.X, p.Y, p.Z = at.MapID, 0, at.X, at.Y, at.Z
}

// Revive types of CM_REVIVE.
const (
	reviveBind    = 0
	reviveRebirth = 1
	reviveItem    = 2
	reviveSkill   = 3
	reviveKisk    = 4
)

const msgRevive = 1300738

// revive is CM_REVIVE: the player comes back to life the way it chose.
// ponytail: rebirth, self resurrection items, skills and kisks wait for their systems (PORTING.md 7, 8, 11).
func (c *conn) revive(r *wire.Reader) {
	p := c.player
	kind := r.C()
	if p == nil || r.Err != nil || !p.dead {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	switch kind {
	case reviveBind:
		s.reviveAt(p, 25, 25)
		p.conn.send(systemMessage(msgRevive))
		p.conn.send(s.statsInfo(p))
		p.conn.send(s.playerInfo(p, false))
		s.moveToBind(p, true, 0)
	case reviveKisk:
		s.kiskRevive(p)
	case reviveSkill:
		s.reviveWithEmotion(p, 10)
	case reviveRebirth:
		s.reviveWithEmotion(p, cmp.Or(p.rebirthPercent, 5))
	case reviveItem:
		stone := s.selfReviveStone(p)
		if stone == nil {
			return
		}
		t := s.template(stone)
		p.setItemCooldown(t)
		p.broadcast(itemUsageAnimation(p.ID, stone.UniqueID, stone.ItemID, 0, 1, 1), true)
		s.decreaseItemCount(p, stone, 1)
		s.reviveWithEmotion(p, 15)
	}
}

// reviveWithEmotion is ReviveController.skillRevive and its likes: the player gets up where it fell.
func (s *Server) reviveWithEmotion(p *player, percent int32) {
	s.reviveAt(p, percent, percent)
	p.broadcast(s.playerEmotion(p, emoteResurrect, 0, 0, 0, 0, 0), true)
	p.conn.send(systemMessage(msgRevive))
	p.conn.send(s.statsInfo(p))
}

// selfReviveStones are the items that bring a player back on its own, in the order they are tried.
var selfReviveStones = []int32{161001001, 161000003, 161000004, 161000001}

// selfReviveStone is ReviveController.getSelfRezStone: the first of the stones in the cube that isn't cooling down.
func (s *Server) selfReviveStone(p *player) *store.Item {
	for _, id := range selfReviveStones {
		for _, item := range p.cube {
			if item.ItemID != id {
				continue
			}
			if t := s.template(item); t != nil && time.Now().Before(p.itemCooldowns[t.UseDelayID]) {
				continue
			}
			return item
		}
	}
	return nil
}

// rebirthPercent is ReviveController.checkForSelfRezEffect: how much life a rebirth effect on the player gives, or 0.
func (s *Server) rebirthPercent(p *player) int32 {
	for _, e := range p.fx.list() {
		for _, t := range e.templates {
			if t.kind == "rebirth" {
				return t.n.Int("resurrect_percent")
			}
		}
	}
	return 0
}

// reviveAt is ReviveController.revive: the player is alive with some of its life and mana.
func (s *Server) reviveAt(p *player, hpPercent, mpPercent int32) {
	p.dead = false
	p.state &^= stateDead
	p.state |= stateActive
	p.life.HP = int32(int64(p.stats.current(data.MaxHP)) * int64(hpPercent) / 100)
	p.life.MP = p.stats.current(data.MaxMP) * mpPercent / 100
	p.dirtyHP, p.dirtyMP = true, true
	s.triggerRestore(p)
	s.triggerFpRestore(p)
	s.startProtection(p)
}

func init() {
	handlers[cmTeleportSelect] = (*conn).teleportSelect
}

// System messages of teleporters.
const (
	msgAirportFlying  = 1300696
	msgAirportWrongNP = 1300692
)

// showTeleportMap is TeleportService.showMap: the client shows the map of the destinations the npc goes to.
func (s *Server) showTeleportMap(p *player, o *object) {
	if p.inState(stateFlying) {
		p.conn.send(systemMessage(msgAirportFlying))
		return
	}
	if race := o.npc.Race; (race == "ELYOS" || race == "ASMODIANS") && race != p.Race {
		p.conn.send(systemMessage(msgAirportWrongNP))
		return
	}
	t := s.data.Teleporters[o.npc.ID]
	if t == nil || t.TeleportID == 0 {
		return
	}
	w := wire.Packet(smTeleportMap)
	w.D(o.id)
	w.H(uint16(t.TeleportID))
	p.conn.send(w)
}

// teleportSelect is CM_TELEPORT_SELECT: the player picks a destination on the map of a teleporter.
func (c *conn) teleportSelect(r *wire.Reader) {
	id, loc := r.D(), r.D()
	p := c.player
	if p == nil || r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	o := c.dialogNpc(id)
	if o == nil || p.dead {
		return
	}
	t := s.data.Teleporters[o.npc.ID]
	if t == nil || t.Type != "REGULAR" && t.Type != "FLIGHT" {
		return
	}
	dest, place := t.Destination(loc), s.data.TeleLocations[loc]
	if dest == nil || place == nil {
		return
	}
	// A price that is 20% cheaper as of 1.9, through the services' price modifiers.
	price := int64(float32(dest.Price) * 0.8)
	price = int64(float64(int64(float64(int64(float64(price)*priceDefault/100))*priceModifier/100)) * priceTaxes / 100)
	if !s.decreaseKinah(p, price) {
		p.conn.send(systemMessage(msgNotEnoughKinah, strconv.FormatInt(price, 10)))
		return
	}
	if t.Type == "FLIGHT" {
		s.startFlightTeleport(p, dest.TeleportID)
		return
	}
	w := wire.Packet(smTeleportLoc)
	w.C(3)
	w.C(0x90)
	w.C(0x9E)
	w.D(place.MapID)
	w.F(place.X)
	w.F(place.Y)
	w.F(place.Z)
	w.C(0)
	p.conn.send(w)
	s.teleportTo(p, place.MapID, place.X, place.Y, place.Z, byte(p.Heading), teleportDefaultDelay)
}
