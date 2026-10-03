package game

import "aionlightning/wire"

func init() {
	handlers[cmReportPlayer] = (*conn).reportPlayer
	handlers[cmDisconnect] = (*conn).clientDisconnect
	// Java's map-open and questionnaire requests have no gameplay action.
	handlers[cmShowMap] = func(*conn, *wire.Reader) {}
	handlers[cmQuestionnaire] = func(_ *conn, r *wire.Reader) {
		r.D()
		for range 4 {
			r.H()
		}
	}
}

// reportPlayer is CM_REPORT_PLAYER: record /ReportAutoHunting for audit only.
func (c *conn) reportPlayer(r *wire.Reader) {
	r.C() // Unknown byte, ignored by Java.
	reported := r.S()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		s.log.Info("[AUDIT] player report", "reporter", p.Name, "reported", reported)
	})
}

// clientDisconnect is CM_DISCONNECT: zero requests a close without a final packet.
// The read loop's deferred disconnected method owns logout and persistence.
func (c *conn) clientDisconnect(r *wire.Reader) {
	request := r.C()
	if r.Err == nil && request == 0 {
		c.close(nil)
	}
}
