package game

import (
	"aionlightning/wire"
)

// Duels: two players of the same race fight without dying (AL-Game's DuelService). Players of the other race are
// enemies without a duel.
// ponytail: killing an enemy player gives no abyss points or divine power yet (the abyss, PORTING.md 11).

func init() {
	handlers[cmDuelRequest] = (*conn).duelRequest
}

const (
	questionDuelAccept        = 0xc36c
	questionDuelConfirm       = 0xc36e
	msgDuelAskedBy            = 1301065
	msgDuelAskedTo            = 1300094
	msgDuelRejectedBy         = 1300097
	msgDuelRejectOf           = 1301064
	msgDuelCancelBy           = 1300134
	msgDuelCancelWith         = 1300135
	msgDuelInvalid            = 1300091
	msgDuelDenied             = 1390120
	msgDuelWon                = 1300098
	msgDuelLost               = 1300099
	deniedDuel          int32 = 32 // DeniedStatus.DUEL
)

// isEnemyPlayer is Player.isEnemyPlayer: another race, or a duel.
func (s *Server) isEnemyPlayer(p, other *player) bool {
	return p.Race != other.Race || s.duels[p.ID] == other.ID
}

// duelRequest is CM_DUEL_REQUEST: the player asks another to a duel, and is asked to confirm.
func (c *conn) duelRequest(r *wire.Reader) {
	id := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		other := s.spawned[id]
		if other == nil {
			if o := p.seen[id]; o != nil && o.npc != nil {
				p.conn.send(systemMessage(msgDuelInvalid, o.npc.Name))
			}
			return
		}
		if other.settings.Deny&deniedDuel != 0 {
			p.conn.send(systemMessage(msgDuelDenied, other.Name))
			return
		}
		if s.isEnemyPlayer(p, other) || s.duels[p.ID] != 0 {
			return
		}
		asked := other.putRequest(questionDuelAccept, func(accepted bool) {
			if !accepted {
				p.conn.send(systemMessage(msgDuelRejectedBy, other.Name))
				other.conn.send(systemMessage(msgDuelRejectOf, p.Name))
				return
			}
			s.startDuel(p, other)
		})
		if asked {
			other.conn.send(questionWindow(questionDuelAccept, 0, p.Name))
			other.conn.send(systemMessage(msgDuelAskedBy, p.Name))
		}
		confirmed := p.putRequest(questionDuelConfirm, func(accepted bool) {
			if accepted {
				// Confirming is cancelling: the asker changed its mind.
				other.conn.send(systemMessage(msgDuelCancelBy, p.Name))
				p.conn.send(systemMessage(msgDuelCancelWith, other.Name))
			}
		})
		if confirmed {
			p.conn.send(questionWindow(questionDuelConfirm, 0, other.Name))
			p.conn.send(systemMessage(msgDuelAskedTo, other.Name))
		}
	})
}

func (s *Server) startDuel(a, b *player) {
	if s.spawned[a.ID] == nil || s.spawned[b.ID] == nil {
		return
	}
	for _, pair := range [][2]*player{{a, b}, {b, a}} {
		w := wire.Packet(smDuel)
		w.C(0)
		w.D(pair[1].ID)
		pair[0].conn.send(w)
	}
	if s.duels == nil {
		s.duels = map[int32]int32{}
	}
	s.duels[a.ID], s.duels[b.ID] = b.ID, a.ID
}

// loseDuel is DuelService.loseDuel: the player has lost to its opponent.
func (s *Server) loseDuel(p *player) {
	other := s.duels[p.ID]
	if other == 0 {
		return
	}
	opponent := s.spawned[other]
	for _, c := range []*player{p, opponent} {
		if c != nil {
			c.fx.removeByTargetSlot("DEBUFF", 1<<30)
			s.cancelSkill(c)
		}
	}
	if opponent != nil {
		result := func(w *wire.Writer, kind byte, msg int32, name string) *wire.Writer {
			w.C(1)
			w.C(kind)
			w.D(msg)
			w.S(name)
			return w
		}
		opponent.conn.send(result(wire.Packet(smDuel), 2, msgDuelWon, p.Name))
		p.conn.send(result(wire.Packet(smDuel), 0, msgDuelLost, opponent.Name))
	}
	delete(s.duels, p.ID)
	delete(s.duels, other)
}
