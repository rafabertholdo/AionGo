package game

import (
	"fmt"
	"strconv"
	"strings"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// Sieges: AL-Game's SiegeService, SiegeLocation, Influence and the //siege command. AL-Game never got as far as
// the fights: what exists is who holds each fortress, artifact and boss raid, whether it is vulnerable, the captures
// (kept in siege_locations) and the influence each race gets from them, sent to everyone on every change.
// ponytail: no siege timer or battle (AL-Game's getSiegeTime is a TODO returning 0), so a location changes hands
// only through //siege capture; captured fortresses' spawns aren't converted to the new race (a TODO there too).

const (
	influenceFortress = 10 // siege.influence.fortress
	influenceArtifact = 1  // siege.influence.artifact
	siegeReset        = 0  // SM_SIEGE_LOCATION_INFO's info type: all locations
	siegeChange       = 1  // one location
	siegeVulnerable   = 2  // the packet's vulnerable value
)

// siegeLoc is a siege location's state.
type siegeLoc struct {
	race       string // ELYOS, ASMODIANS, BALAUR
	legion     int32
	vulnerable bool
	nextState  byte // 0 invulnerable, 1 vulnerable
}

// siegeInfluence is a location's worth to its owner: only fortresses and artifacts count.
func siegeInfluence(kind string) float32 {
	switch kind {
	case "FORTRESS":
		return influenceFortress
	case "ARTIFACT":
		return influenceArtifact
	}
	return 0
}

// siegeState is the state of a location; artifacts are always vulnerable, the rest start invulnerable. Callers hold siegeMu.
func (s *Server) siegeState(l data.SiegeLocation) *siegeLoc {
	if st := s.sieges[l.ID]; st != nil {
		return st
	}
	if s.sieges == nil {
		s.sieges = map[int32]*siegeLoc{}
	}
	st := &siegeLoc{race: "BALAUR", vulnerable: l.Type == "ARTIFACT"}
	if st.vulnerable {
		st.nextState = 1
	}
	if o, ok := s.siegeOwners[l.ID]; ok {
		st.race, st.legion = o.Race, o.Legion
	}
	s.sieges[l.ID] = st
	return st
}

// siegeRaceID is SiegeRace's id.
func siegeRaceID(race string) byte {
	switch race {
	case "ELYOS":
		return 0
	case "ASMODIANS":
		return 1
	}
	return 2
}

// siegeLocations is SM_SIEGE_LOCATION_INFO for every location.
func (s *Server) siegeLocations() *wire.Writer { return s.siegeInfo(siegeReset, s.data.Sieges) }

// siegeInfo is SM_SIEGE_LOCATION_INFO: locations with their owner and state.
func (s *Server) siegeInfo(kind byte, locations []data.SiegeLocation) *wire.Writer {
	s.siegeMu.Lock()
	defer s.siegeMu.Unlock()
	w := wire.Packet(smSiegeLocationInfo)
	w.C(kind)
	w.H(uint16(len(locations)))
	for _, l := range locations {
		st := s.siegeState(l)
		w.D(l.ID)
		w.D(st.legion)
		w.D(0)
		w.D(0)
		w.C(siegeRaceID(st.race))
		if st.vulnerable {
			w.C(siegeVulnerable)
		} else {
			w.C(0)
		}
		w.C(1) // the faction can teleport
		w.C(st.nextState)
		w.D(0)
		w.D(0)
	}
	return w
}

// influenceRatio is SM_INFLUENCE_RATIO: each race's share of the influence of the locations it holds.
func (s *Server) influenceRatio() *wire.Writer {
	s.siegeMu.Lock()
	var total float32
	share := map[string]float32{}
	for _, l := range s.data.Sieges {
		v := siegeInfluence(l.Type)
		total += v
		share[s.siegeState(l).race] += v
	}
	s.siegeMu.Unlock()
	if total > 0 { // AL-Game divides by zero here and sends NaN
		for race := range share {
			share[race] /= total
		}
	}
	w := wire.Packet(smInfluenceRatio)
	w.D(0) // the siege time
	triple := func() {
		w.F(share["ELYOS"])
		w.F(share["ASMODIANS"])
		w.F(share["BALAUR"])
	}
	triple()
	w.H(1) // 1.9 has 3 here, with balauria's values
	w.D(400010000)
	triple()
	return w
}

// siegeBroadcast sends p to everyone in the world; the caller holds visMu.
func (s *Server) siegeBroadcast(packets ...*wire.Writer) {
	for _, p := range s.spawned {
		for _, w := range packets {
			p.conn.send(w)
		}
	}
}

// siegeCapture is SiegeService.capture: the location goes to race and legion, fortresses turn invulnerable, and
// everyone hears of it and of the new influence. The caller holds visMu.
func (s *Server) siegeCapture(l data.SiegeLocation, race string, legion int32) {
	s.siegeMu.Lock()
	st := s.siegeState(l)
	st.race, st.legion = race, legion
	if l.Type == "FORTRESS" {
		st.vulnerable = false
	}
	s.siegeMu.Unlock()
	if s.siegeDB != nil {
		if err := s.siegeDB.SaveSiegeOwner(l.ID, store.SiegeOwner{Race: race, Legion: legion}); err != nil {
			s.log.Error("saving siege location", "id", l.ID, "err", err)
		}
	}
	s.siegeBroadcast(s.siegeInfo(siegeChange, []data.SiegeLocation{l}), s.influenceRatio())
}

// siegeSet is //siege set: the current and next state, sent to everyone. The caller holds visMu.
func (s *Server) siegeSet(l data.SiegeLocation, vulnerable bool, next int) {
	s.siegeMu.Lock()
	st := s.siegeState(l)
	st.vulnerable = vulnerable
	if next >= 0 {
		st.nextState = byte(next)
	}
	s.siegeMu.Unlock()
	s.siegeBroadcast(s.siegeInfo(siegeChange, []data.SiegeLocation{l}))
}

func (s *Server) siegeByID(id int32) (data.SiegeLocation, bool) {
	for _, l := range s.data.Sieges {
		if l.ID == id {
			return l, true
		}
	}
	return data.SiegeLocation{}, false
}

// matches is AL-Game's smart matching: a prefix of the word.
func matches(word, given string) bool { return strings.HasPrefix(word, strings.ToLower(given)) }

// stateValue reads a state as a number or a prefix of "invulnerable" or "vulnerable".
func stateValue(text string, vulnerable int) (int, bool) {
	if n, err := strconv.Atoi(text); err == nil {
		return n, true
	}
	switch {
	case matches("invulnerable", text):
		return 0, true
	case matches("vulnerable", text):
		return vulnerable, true
	}
	return 0, false
}

const siegeHelp = "[Help: Siege Command]\n  Use //siege help <capture|set|list> for more details on the command.\n" +
	"  Notice: This command uses smart matching. You may abbreviate most commands.\n" +
	"  For example: (//siege cap 1011 ely) will match to (//siege capture 1011 elyos)\n"

// adminSiege is //siege capture <location id> <race> [legion id], set <location id> <current state> [next state], list, help.
func (s *Server) adminSiege(p *player, params []string) {
	if len(params) == 0 {
		s.tell(p, "No parameters detected.\nPlease use //siege help")
		return
	}
	switch cmd := params[0]; {
	case matches("help", cmd):
		s.tell(p, siegeHelp)
	case matches("capture", cmd):
		s.adminSiegeCapture(p, params)
	case matches("set", cmd):
		s.adminSiegeSet(p, params)
	case matches("list", cmd):
		s.adminSiegeList(p)
	default:
		s.tell(p, "Sub Command does not exist.\nPlease use //siege help")
	}
}

func (s *Server) adminSiegeCapture(p *player, params []string) {
	if len(params) < 3 || len(params) > 4 {
		s.tell(p, "Incorrect parameter count.\nPlease use //siege help capture")
		return
	}
	id, err := strconv.Atoi(params[1])
	if err != nil {
		s.tell(p, "Location ID must be an integer.")
		return
	}
	var race string
	switch given := params[2]; {
	case matches("elyos", given):
		race = "ELYOS"
	case matches("asmos", given):
		race = "ASMODIANS"
	case matches("balaur", given):
		race = "BALAUR"
	default:
		s.tell(p, "Race must be: Elyos, Asmos, or Balaur.\nPlease use //siege help capture")
		return
	}
	var legion int
	if len(params) == 4 {
		if legion, err = strconv.Atoi(params[3]); err != nil {
			s.tell(p, "Legion ID must be an integer.")
			return
		}
	}
	l, ok := s.siegeByID(int32(id))
	if !ok {
		s.tell(p, fmt.Sprintf("Location does not exist: %d", id))
		return
	}
	s.tell(p, fmt.Sprintf("[Admin Capture]\n - Location ID: %d\n - Race: %s\n - Legion ID: %d\n", id, race, legion))
	s.siegeCapture(l, race, int32(legion))
}

func (s *Server) adminSiegeSet(p *player, params []string) {
	if len(params) < 3 || len(params) > 4 {
		s.tell(p, "Incorrect parameter count.\nPlease use //siege help set")
		return
	}
	id, err := strconv.Atoi(params[1])
	if err != nil {
		s.tell(p, "Location ID must be an integer.")
		return
	}
	current, ok := stateValue(params[2], siegeVulnerable)
	if !ok {
		s.tell(p, "Current State must be an integer.")
		return
	}
	next := -1
	if len(params) == 4 {
		if next, ok = stateValue(params[3], 1); !ok {
			s.tell(p, "Next State must be an integer.")
			return
		}
	}
	if current != 0 && current != siegeVulnerable || len(params) == 4 && next != 0 && next != 1 {
		s.tell(p, "Incorrect state value.\nPlease use //siege help set")
		return
	}
	l, found := s.siegeByID(int32(id))
	if !found {
		s.tell(p, fmt.Sprintf("Location does not exist: %d", id))
		return
	}
	s.tell(p, fmt.Sprintf("[Admin Set State]\n - Location ID: %d\n - New Current State: %d\n", id, current))
	s.siegeSet(l, current == siegeVulnerable, next)
}

func (s *Server) adminSiegeList(p *player) {
	msg := "[Siege Locations]\n"
	for _, l := range s.data.Sieges {
		s.siegeMu.Lock()
		st := *s.siegeState(l)
		s.siegeMu.Unlock()
		line := fmt.Sprintf(" - %s %d (%s)", strings.ToUpper(l.Type[:1])+strings.ToLower(l.Type[1:]), l.ID, st.race)
		if st.legion != 0 {
			line += fmt.Sprintf(" %d", st.legion)
		}
		msg += line + "\n"
		if len(msg) > 500 { // gets long
			s.tell(p, msg)
			msg = ""
		}
	}
	s.tell(p, msg)
}
