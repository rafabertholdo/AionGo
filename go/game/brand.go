package game

import "aionlightning/wire"

func init() {
	handlers[cmShowBrand] = (*conn).showBrand
}

// showBrand is CM_SHOW_BRAND and GroupService/AllianceService.showBrand.
// Java permits any member to broadcast and does not retain brand state.
func (c *conn) showBrand(r *wire.Reader) {
	brandID, targetID := r.D(), r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		w := showBrand(brandID, targetID)
		if p.group != nil {
			for _, m := range p.group.members {
				m.conn.send(w)
			}
		}
		if p.alliance != nil {
			for _, m := range p.alliance.members {
				if m.conn != nil {
					m.conn.send(w)
				}
			}
		}
	})
}

// showBrand is SM_SHOW_BRAND, including the zero/zero reset on alliance join.
func showBrand(brandID, targetID int32) *wire.Writer {
	w := wire.Packet(smShowBrand)
	w.H(1)
	w.D(brandID)
	w.D(targetID)
	return w
}
