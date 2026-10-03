package game

import "aionlightning/wire"

func init() {
	handlers[cmObjectSearch] = (*conn).objectSearch
}

// objectSearch is CM_OBJECT_SEARCH: locate the first spawn of a template on the map.
func (c *conn) objectSearch(r *wire.Reader) {
	npcID := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		for _, group := range s.data.SpawnsByNPC[npcID] {
			if len(group.Spots) == 0 {
				continue
			}
			spot := group.Spots[0]
			w := wire.Packet(smShowNpcOnMap)
			w.D(npcID)
			w.D(group.Map)
			w.D(group.Map)
			w.F(spot.X)
			w.F(spot.Y)
			w.F(spot.Z)
			c.send(w)
			return
		}
	})
}
