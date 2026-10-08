package game

import (
	"time"

	"aionlightning/wire"
)

func init() {
	// A portal answers a dialog request with three seconds of use, and then the player goes through.
	quests := handlers[cmShowDialog]
	handlers[cmShowDialog] = func(c *conn, r *wire.Reader) {
		peek := wire.NewReader(append([]byte(nil), r.Data...))
		id := peek.D()
		if peek.Err == nil && c.portalDialog(id) {
			return
		}
		quests(c, r)
	}
}

// portalDialog is PortalController.onDialogRequest; it says whether the object was a portal.
func (c *conn) portalDialog(id int32) bool {
	p := c.player
	if p == nil {
		return false
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if p.dead || !p.spawned {
		return false
	}
	o := p.seen[id]
	if o == nil || o.npc == nil || o.dead || o.worldID != p.WorldID || o.instance != p.instance || distance3D(p.X, p.Y, p.Z, o.x, o.y, o.z) > 10 {
		return false
	}
	if o.worldID == 300030000 && o.npc.ID == 700437 {
		return c.nochsanaArtifact(p, o)
	}
	// The 1.9 client assigns ReturnToEntrance to the Fire Temple exit.
	returnToEntrance := o.worldID == 300030000 && o.npc.ID == 700438 ||
		o.worldID == fireTempleWorld && o.npc.ID == 730048
	if !returnToEntrance && (o.npc.Type != "PORTAL" || s.data.Portals[o.npc.ID] == nil) {
		return false
	}
	portal := s.data.Portals[o.npc.ID]
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
	s.later(3*time.Second, func() {
		if p.conn != c || !p.spawned || p.dead || p.seen[id] != o || s.byID[id] != o || o.dead || o.worldID != p.WorldID || o.instance != p.instance || distance3D(p.X, p.Y, p.Z, o.x, o.y, o.z) > 10 {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(s.playerEmotionTo(p, emoteEndQuestLoot, 0, o.id, 0, 0, 0, 0), true)
		if returnToEntrance {
			s.leaveInstance(p)
		} else if !s.darkPoetaExit(p, o) {
			s.portalUse(p, portal)
		}
	})
	return true
}
