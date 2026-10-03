package game

import (
	"math/rand/v2"

	"aionlightning/wire"
)

func init() {
	handlers[cmClientCommandRoll] = (*conn).clientCommandRoll
}

// clientCommandRoll is CM_CLIENT_COMMAND_ROLL: roll from 1 through the requested maximum.
func (c *conn) clientCommandRoll(r *wire.Reader) {
	maxRoll := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		roll := int32(1)
		if maxRoll > 1 {
			// Use int64 for the range because maxRoll may be MaxInt32.
			roll += int32(rand.Int64N(int64(maxRoll)))
		}
		p.conn.send(systemMessage(1400126, roll, maxRoll))
		p.broadcast(systemMessage(1400127, p.Name, roll, maxRoll), false)
	})
}
