package game

import (
	"slices"

	"aionlightning/wire"
)

func init() {
	handlers[cmAllianceGroupChange] = (*conn).allianceGroupChange
}

// allianceGroupChange is CM_ALLIANCE_GROUP_CHANGE.
func (c *conn) allianceGroupChange(r *wire.Reader) {
	memberID, groupID, secondID := r.D(), r.D(), r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		a := p.alliance
		if a == nil {
			s.tell(p, "You are not in an alliance.")
			return
		}
		if a.captain != p && !slices.Contains(a.vice, p) {
			s.tell(p, "You do not have the authority for that.")
			return
		}
		s.changeAllianceGroup(a, memberID, groupID, secondID)
	})
}

// changeAllianceGroup is AllianceService.handleGroupChange. Validate the
// request before mutation; Java assumes valid members, group IDs and capacity.
func (s *Server) changeAllianceGroup(a *alliance, memberID, groupID, secondID int32) {
	var first, second *player
	for _, m := range a.members {
		if m.ID == memberID {
			first = m
		}
		if m.ID == secondID {
			second = m
		}
	}
	if first == nil {
		return
	}
	if groupID == 0 {
		if second == nil {
			return
		}
		a.initSlots()
		a.slots[first], a.slots[second] = a.slots[second], a.slots[first]
		s.broadcastAllianceGroupChange(a, first)
		s.broadcastAllianceGroupChange(a, second)
		return
	}
	if groupID < 1000 || groupID > 1003 {
		return
	}
	if a.slot(first) != groupID && a.groupSize(groupID) >= allianceGroupSize {
		return
	}
	a.initSlots()
	a.slots[first] = groupID
	s.broadcastAllianceGroupChange(a, first)
}

func (s *Server) broadcastAllianceGroupChange(a *alliance, subject *player) {
	info := s.allianceMemberInfo(a, subject, allianceEnter)
	id := playerID(subject)
	for _, m := range a.members {
		if m.conn != nil {
			m.conn.send(info)
			m.conn.send(id)
		}
	}
}
