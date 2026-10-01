package game

import (
	"aionlightning/wire"
)

func init() {
	handlers[cmQuestionResponse] = (*conn).questionResponse
}

// Question codes of SM_QUESTION_WINDOW (the client's string ids) the server asks with.
const (
	questionSoulHealing = 160011
	questionSoulBound   = 95006
)

// request is what the player answers a question window with: it runs with whether the answer was yes.
type request func(accepted bool)

// putRequest is ResponseRequester.putRequest: one question of a kind at a time.
func (p *player) putRequest(code int32, handler request) bool {
	if p.requests == nil {
		p.requests = map[int32]request{}
	}
	if _, asked := p.requests[code]; asked {
		return false
	}
	p.requests[code] = handler
	return true
}

// questionWindow is SM_QUESTION_WINDOW: a yes or no question, with its parameters.
func questionWindow(code, sender int32, params ...any) *wire.Writer {
	w := wire.Packet(smQuestionWindow)
	w.D(code)
	for _, param := range params {
		if id, ok := param.(descriptionID); ok {
			w.H(0x24)
			w.D(int32(id))
			w.H(0)
		} else {
			w.S(param.(string))
		}
	}
	w.D(0)
	w.H(0)
	w.C(1)
	w.D(sender)
	w.D(6)
	return w
}

// questionResponse is CM_QUESTION_RESPONSE.
func (c *conn) questionResponse(r *wire.Reader) {
	p := c.player
	code, answer := r.D(), r.C()
	if p == nil || r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if handler, ok := p.requests[code]; ok {
		delete(p.requests, code)
		handler(answer != 0)
	}
}
