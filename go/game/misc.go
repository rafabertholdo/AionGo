package game

import (
	"slices"
	"strings"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// The small requests of the client: titles, macros, taking off effects, looking at another player, searching for
// players, and a group's loot rules.

func init() {
	handlers[cmTitleSet] = (*conn).titleSet
	handlers[cmMacroCreate] = (*conn).macroCreate
	handlers[cmMacroDelete] = (*conn).macroDelete
	handlers[cmSkillDeactivate] = (*conn).skillDeactivate
	handlers[cmRemoveAlteredState] = (*conn).removeAlteredState
	handlers[cmViewPlayerDetails] = (*conn).viewPlayerDetails
	handlers[cmPlayerSearch] = (*conn).playerSearch
	handlers[cmDistributionSettings] = (*conn).distributionSettings
}

// macroSaver keeps macros; store.Store does.
type macroSaver interface {
	AddMacro(playerID, order int32, text string) error
	DeleteMacro(playerID, order int32) error
}

const (
	msgRejectedWatch = 1390114
	msgSearchLevel   = 1400341
	maxSearchResults = 124 // CM_PLAYER_SEARCH.MAX_RESULTS
	deniedDetails    = 1   // DeniedStatus.VEIW_DETAIL
)

// titleSet is CM_TITLE_SET: the player wears a title, and those who see it are told.
func (c *conn) titleSet(r *wire.Reader) {
	title := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		w := wire.Packet(smTitleSet)
		w.D(title)
		p.conn.send(w)
		u := wire.Packet(smTitleUpdate)
		u.D(p.ID)
		u.D(title)
		p.broadcast(u, false)
		p.TitleID = title
		p.stats = s.playerStats(p)
		p.conn.send(s.statsInfo(p))
	})
}

func macroResult(code byte) *wire.Writer {
	w := wire.Packet(smMacroResult)
	w.C(code)
	return w
}

// macroCreate is CM_MACRO_CREATE.
func (c *conn) macroCreate(r *wire.Reader) {
	order, text := int32(r.C()), r.S()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		if !slices.ContainsFunc(p.macros, func(m store.Macro) bool { return m.Order == order }) {
			p.macros = append(p.macros, store.Macro{Order: order, Text: text})
			if err := s.macroDB.AddMacro(p.ID, order, text); err != nil {
				s.log.Error("saving a macro", "err", err)
			}
		}
		p.conn.send(macroResult(0))
	})
}

// macroDelete is CM_MACRO_DELETE.
func (c *conn) macroDelete(r *wire.Reader) {
	order := int32(r.C())
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		if slices.ContainsFunc(p.macros, func(m store.Macro) bool { return m.Order == order }) {
			p.macros = slices.DeleteFunc(p.macros, func(m store.Macro) bool { return m.Order == order })
			if err := s.macroDB.DeleteMacro(p.ID, order); err != nil {
				s.log.Error("deleting a macro", "err", err)
			}
		}
		p.conn.send(macroResult(1))
	})
}

// skillDeactivate is CM_SKILL_DEACTIVATE: a toggled skill is switched off.
func (c *conn) skillDeactivate(r *wire.Reader) {
	id := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		for stack, e := range p.fx.noshow {
			if e.tmpl.ID == id {
				e.end()
				delete(p.fx.noshow, stack)
			}
		}
	})
}

// removeAlteredState is CM_REMOVE_ALTERED_STATE: the player takes an effect off.
func (c *conn) removeAlteredState(r *wire.Reader) {
	id := int32(r.H())
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) { p.fx.removeEffect(id) })
}

// viewPlayerDetails is CM_VIEW_PLAYER_DETAILS: what another player wears.
func (c *conn) viewPlayerDetails(r *wire.Reader) {
	id := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		target := s.spawned[id]
		if target == nil {
			return
		}
		if target.settings.Deny&deniedDetails != 0 {
			p.conn.send(systemMessage(msgRejectedWatch, target.Name))
			return
		}
		var worn []*store.Item
		for _, item := range target.equipment {
			if item.Slot < 1<<19 {
				worn = append(worn, item)
			}
		}
		w := wire.Packet(smViewPlayerDetails)
		w.D(id)
		w.C(11)
		w.C(byte(len(worn)))
		w.C(0)
		w.D(0)
		for _, item := range worn {
			t := s.template(item)
			if t == nil {
				t = &data.ItemTemplate{ID: item.ItemID}
			}
			w.D(t.ID)
			w.H(36)
			w.D(t.NameID)
			w.H(0)
			w.H(36)
			w.C(4)
			w.C(1)
			w.H(0)
			w.H(0)
			w.C(0)
			w.H(0)
			w.C(6)
			w.H(uint16(item.Slot))
			w.H(0)
			w.C(0)
			w.H(62)
			w.H(uint16(item.Count))
			for range 5 {
				w.D(0)
			}
			w.C(0)
		}
		p.conn.send(w)
	})
}

// playerSearch is CM_PLAYER_SEARCH: the players who are in the world and match the filters.
func (c *conn) playerSearch(r *wire.Reader) {
	name := r.S()
	if name != "" {
		r.B(44 - (len(name)*2 + 2))
	} else {
		r.B(42)
	}
	region, classMask := r.D(), r.D()
	minLevel, maxLevel, lfg := r.C(), r.C(), r.C()
	r.C()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		if p.level < 10 {
			p.conn.send(systemMessage(msgSearchLevel, "10"))
			return
		}
		var found []*player
		for _, id := range sortedKeys(s.spawned) {
			o := s.spawned[id]
			switch {
			case o.friendStatus == friendStatusOffline:
			case lfg == 1 && !o.lookingForGroup:
			case name != "" && !strings.Contains(strings.ToLower(o.Name), strings.ToLower(name)):
			case minLevel != 0xFF && o.level < int(minLevel):
			case maxLevel != 0xFF && o.level > int(maxLevel):
			case classMask > 0 && classMaskOf(o.Class)&classMask == 0:
			case region > 0 && o.WorldID != region:
			case o.Race != p.Race:
			default:
				found = append(found, o)
			}
			if len(found) > maxSearchResults {
				break
			}
		}
		w := wire.Packet(smPlayerSearch)
		w.H(uint16(len(found)))
		for _, o := range found {
			_, gender := raceGender(o.Character)
			w.D(o.WorldID)
			w.F(o.X)
			w.F(o.Y)
			w.F(o.Z)
			w.C(byte(classIDs[o.Class]))
			w.C(byte(gender))
			w.C(byte(o.level))
			w.C(jIf[byte](o.lookingForGroup, 2, 0)) // status: 2 = looking for group
			w.S(o.Name)
			w.B(make([]byte, max(44-(len(o.Name)*2+2), 0)))
		}
		p.conn.send(w)
	})
}

// classMaskOf is PlayerClass.getMask: the bit the class stands for, a bit for each of the classes in order.
func classMaskOf(class string) int32 { return 1 << classIDs[class] }

// distributionSettings is CM_DISTRIBUTION_SETTINGS: the group's leader sets how loot goes.
func (c *conn) distributionSettings(r *wire.Reader) {
	rule := r.D()
	distribution := r.D()
	var qualities [7]int32
	for i := range qualities {
		qualities[i] = r.D()
	}
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		g := p.group
		if g == nil || g.leader != p {
			return
		}
		g.rule = lootFreeForAll
		if rule == lootRoundRobin || rule == lootLeader {
			g.rule = rule
		}
		g.distribution = 0
		if distribution == 2 || distribution == 3 {
			g.distribution = distribution
		}
		g.qualityRules = &qualities
		for _, m := range g.members {
			m.conn.send(groupInfo(g))
		}
	})
}
