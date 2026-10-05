package game

import "aionlightning/wire"

func init() {
	handlers[cmOpenStaticdoor] = (*conn).openStaticDoor
}

// openStaticDoor is CM_OPEN_STATICDOOR: broadcast the switch-door emotion for the door ID.
func (c *conn) openStaticDoor(r *wire.Reader) {
	doorID := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		p.broadcast(staticDoorEmotion(doorID), true)
		s.darkPoetaDoor(p)
	})
}

func staticDoorEmotion(doorID int32) *wire.Writer {
	w := wire.Packet(smEmotion)
	w.D(doorID)
	w.C(emoteSwitchDoor)
	w.H(9)
	w.D(0)
	return w
}
