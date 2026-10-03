package game

import (
	"math"
	"math/rand/v2"
	"strconv"
	"strings"

	"aionlightning/wire"
)

func init() {
	handlers[cmClientCommandRoll] = (*conn).clientCommandRoll
	handlers[cmClientCommandLoc] = (*conn).clientCommandLoc
}

// clientCommandLoc is CM_CLIENT_COMMAND_LOC: report the server's current coordinates.
func (c *conn) clientCommandLoc(_ *wire.Reader) {
	c.withPlayer(func(s *Server, p *player) {
		s.log.Info("[AUDIT] Received \"/loc\" command")
		p.conn.send(systemMessage(
			230038,
			p.WorldID,
			locationCoordinate(p.X),
			locationCoordinate(p.Y),
			locationCoordinate(p.Z),
		))
	})
}

// locationCoordinate retains Java Float.toString's decimal point and exponent style.
func locationCoordinate(value float32) string {
	if math.IsInf(float64(value), 1) {
		return "Infinity"
	}
	if math.IsInf(float64(value), -1) {
		return "-Infinity"
	}
	format := byte('f')
	if abs := math.Abs(float64(value)); abs != 0 && (abs < 0.001 || abs >= 10000000) {
		format = 'e'
	}
	text := strconv.FormatFloat(float64(value), format, -1, 32)
	mantissa, exponent, scientific := strings.Cut(text, "e")
	if scientific && !strings.Contains(mantissa, ".") {
		// Java selects the nearest decimal with at least two scientific digits.
		text = strconv.FormatFloat(float64(value), 'e', 1, 32)
		mantissa, exponent, _ = strings.Cut(text, "e")
	}
	if !strings.Contains(mantissa, ".") && !strings.ContainsAny(mantissa, "NI") {
		mantissa += ".0"
	}
	if scientific {
		n, _ := strconv.Atoi(exponent)
		return mantissa + "E" + strconv.Itoa(n)
	}
	return mantissa
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
