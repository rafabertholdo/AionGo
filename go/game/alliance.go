package game

import (
	"slices"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// Alliances of up to 24 players in four groups of six: AL-Game's PlayerAlliance and AllianceService.
// ponytail: a player who leaves the world leaves the alliance at once (AL-Game waits alliance.removetime),
// and there are no loot rules, brands or readiness checks.

const (
	maxAllianceSize           = 24
	allianceGroupSize         = 6
	allianceViceCaptains      = 4
	questionAllianceInvite    = 70004
	chatAlliance              = 0x06
	msgAllianceInvitedGroup   = 1300189
	msgAllianceInvited        = 1301017
	msgAllianceRejected       = 1300190
	msgAllianceAlreadyOurs    = 1300193
	msgAllianceOther          = 1300974
	msgAllianceCantAdd        = 1300196
	msgAllianceNotLeader      = 1300194
	msgAllianceDispersed      = 1300201
	msgAllianceEntered        = 1390263
	msgAllianceSelf           = 1301006
	msgAllianceDead           = 1301007
	msgAllianceNoSlot         = 1300975
	msgAllianceBan            = 1300979
	msgAllianceCaptainChanged = 1300986
	msgAlliancePromoted       = 1300984
	msgAllianceDemoted        = 1300985
)

// The wire ids of a PlayerAllianceEvent that differ from the group's.
const (
	allianceBanned  byte = 2
	allianceEnter   byte = 13
	allianceLeaving byte = 0
)

// alliance is a PlayerAlliance.
type alliance struct {
	id      int32
	captain *player
	vice    []*player
	members []*player // in the order they joined
}

func (a *alliance) has(p *player) bool { return slices.Contains(a.members, p) }

// slot is the alliance group (1000-1003) a member is in: the members fill the groups in order.
func (a *alliance) slot(p *player) int32 {
	return 1000 + int32(slices.Index(a.members, p)/allianceGroupSize)
}

// inviteToAlliance is AllianceService.invitePlayerToAlliance, with RestrictionsManager.canInviteToAlliance.
func (s *Server) inviteToAlliance(p, invited *player) {
	if s.restrictedInPrison(p, "invite members to alliance") {
		return
	}
	a := p.alliance
	var size int
	if a != nil {
		size = len(a.members)
	} else if p.group != nil {
		size = len(p.group.members)
	}
	incoming := 1
	if invited.group != nil {
		incoming = len(invited.group.members)
	}
	switch {
	case invited == p:
		p.conn.send(systemMessage(msgAllianceSelf))
		return
	case p.dead:
		p.conn.send(systemMessage(msgAllianceDead))
		return
	case invited.Race != p.Race:
		p.conn.send(systemMessage(msgGroupOtherRace))
		return
	case a != nil && a.has(invited):
		p.conn.send(systemMessage(msgAllianceAlreadyOurs, invited.Name))
		return
	case invited.alliance != nil:
		p.conn.send(systemMessage(msgAllianceOther, invited.Name))
		return
	case a != nil && a.captain != p && !slices.Contains(a.vice, p):
		p.conn.send(systemMessage(msgAllianceNotLeader, invited.Name))
		return
	case a == nil && p.group != nil && p.group.leader != p:
		p.conn.send(systemMessage(msgGroupOnlyLeader))
		return
	case invited.group != nil && invited.group.leader != invited:
		p.conn.send(systemMessage(msgAllianceNotLeader, invited.Name))
		return
	case size+incoming > maxAllianceSize:
		p.conn.send(systemMessage(msgAllianceNoSlot))
		return
	}
	asked := invited.putRequest(questionAllianceInvite, func(accepted bool) {
		if !accepted {
			p.conn.send(systemMessage(msgAllianceRejected, invited.Name))
			return
		}
		if s.spawned[p.ID] == nil || s.spawned[invited.ID] == nil || invited.alliance != nil {
			return
		}
		s.formAlliance(p, invited)
	})
	if !asked {
		return
	}
	if invited.group != nil {
		p.conn.send(systemMessage(msgAllianceInvitedGroup, invited.Name))
	} else {
		p.conn.send(systemMessage(msgAllianceInvited, invited.Name))
	}
	invited.conn.send(questionWindow(questionAllianceInvite, 0, p.Name))
}

// formAlliance adds the invited player, or their group, to the inviter's alliance, which a group's leader starts.
func (s *Server) formAlliance(p, invited *player) {
	a := p.alliance
	var joining []*player
	if a == nil {
		a = &alliance{id: s.ids.nextID(), captain: p}
		if p.group != nil {
			joining = slices.Clone(p.group.members)
		} else {
			joining = []*player{p}
		}
	}
	if invited.group != nil {
		joining = append(joining, invited.group.members...)
	} else {
		joining = append(joining, invited)
	}
	if len(a.members)+len(joining) > maxAllianceSize {
		p.conn.send(systemMessage(msgAllianceCantAdd))
		return
	}
	for _, m := range joining {
		s.leaveGroup(m)
	}
	for _, m := range joining {
		s.addToAlliance(a, m)
	}
}

// addToAlliance is AllianceService.addMemberToAlliance.
func (s *Server) addToAlliance(a *alliance, p *player) {
	a.members = append(a.members, p)
	p.alliance = a
	p.conn.send(allianceInfo(a))
	w := wire.Packet(smShowBrand)
	w.H(1)
	w.D(0)
	w.D(0)
	p.conn.send(w)
	p.conn.send(systemMessage(msgAllianceEntered))
	s.updateAlliance(a, p, allianceEnter)
	for _, m := range a.members {
		if m != p {
			p.conn.send(s.allianceMemberInfo(a, m, allianceEnter))
		}
	}
}

// allianceInfo is SM_ALLIANCE_INFO.
func allianceInfo(a *alliance) *wire.Writer {
	w := wire.Packet(smAllianceInfo)
	w.H(4)
	w.D(a.id)
	w.D(a.captain.ID)
	for i := range allianceViceCaptains {
		if i < len(a.vice) {
			w.D(a.vice[i].ID)
		} else {
			w.D(0)
		}
	}
	for range 9 { // loot rules
		w.D(0)
	}
	w.C(0)
	for i := range int32(4) {
		w.D(i)
		w.D(1000 + i)
	}
	w.D(0)
	w.H(0)
	return w
}

// allianceMemberInfo is SM_ALLIANCE_MEMBER_INFO.
func (s *Server) allianceMemberInfo(a *alliance, p *player, event byte) *wire.Writer {
	w := wire.Packet(smAllianceMemberInfo)
	w.D(a.slot(p))
	w.D(p.ID)
	hp, maxHP := p.hitPoints()
	w.D(maxHP)
	w.D(hp)
	w.D(p.stats.current(data.MaxMP))
	w.D(p.life.MP)
	w.D(p.stats.current(data.FlyTime))
	w.D(p.life.FP)
	w.D(p.WorldID)
	w.D(p.WorldID)
	w.F(p.X)
	w.F(p.Y)
	w.F(p.Z)
	_, gender := raceGender(p.Character)
	w.C(byte(classIDs[p.Class]))
	w.C(byte(gender))
	w.C(byte(p.level))
	w.C(event)
	w.H(0)
	if event <= 1 { // leaving, or moving
		return w
	}
	w.S(p.Name)
	w.D(0)
	list := p.fx.list()
	w.H(uint16(len(list)))
	for _, e := range list {
		w.D(e.effector.cid())
		w.H(uint16(e.tmpl.ID))
		w.C(byte(e.level))
		w.C(byte(e.slot()))
		w.D(e.elapsed())
	}
	return w
}

// updateAlliance is AllianceService.broadcastAllianceMemberInfo: the other members are told of subject.
func (s *Server) updateAlliance(a *alliance, subject *player, event byte) {
	for _, m := range slices.Clone(a.members) {
		if m != subject {
			m.conn.send(s.allianceMemberInfo(a, subject, event))
		}
	}
}

// broadcastAllianceInfo sends the alliance's captains again to every member.
func (s *Server) broadcastAllianceInfo(a *alliance, msg *wire.Writer) {
	for _, m := range a.members {
		m.conn.send(allianceInfo(a))
		if msg != nil {
			m.conn.send(msg)
		}
	}
}

// allianceStatus is AllianceService.playerStatusInfo.
func (s *Server) allianceStatus(p *player, status byte, id int32) {
	a := p.alliance
	if a == nil {
		return
	}
	target := s.spawned[id]
	leads := a.captain == p
	inAlliance := target != nil && a.has(target)
	switch status {
	case 12:
		s.leaveAlliance(p, allianceLeaving)
	case 14:
		if inAlliance && target != p && (leads || slices.Contains(a.vice, p) && target != a.captain) {
			target.conn.send(systemMessage(msgAllianceBan, p.Name))
			s.leaveAlliance(target, allianceBanned)
		}
	case 15:
		if leads && inAlliance && target != p {
			a.vice = slices.DeleteFunc(a.vice, func(m *player) bool { return m == target })
			old := a.captain
			a.captain = target
			s.broadcastAllianceInfo(a, systemMessage(msgAllianceCaptainChanged, old.Name, target.Name))
		}
	case 23:
		if leads && inAlliance && target != p && !slices.Contains(a.vice, target) && len(a.vice) < allianceViceCaptains {
			a.vice = append(a.vice, target)
			s.broadcastAllianceInfo(a, systemMessage(msgAlliancePromoted, target.Name))
		}
	case 24:
		if leads && inAlliance && slices.Contains(a.vice, target) {
			a.vice = slices.DeleteFunc(a.vice, func(m *player) bool { return m == target })
			s.broadcastAllianceInfo(a, systemMessage(msgAllianceDemoted, target.Name))
		}
	}
}

// leaveAlliance is AllianceService.removeMemberFromAlliance: the alliance ends when one is left.
func (s *Server) leaveAlliance(p *player, event byte) {
	a := p.alliance
	if a == nil {
		return
	}
	s.updateAlliance(a, p, event)
	a.members = slices.DeleteFunc(a.members, func(m *player) bool { return m == p })
	a.vice = slices.DeleteFunc(a.vice, func(m *player) bool { return m == p })
	p.alliance = nil
	p.conn.send(wire.Packet(smLeaveGroupMember))
	if len(a.members) < 2 {
		for _, m := range slices.Clone(a.members) {
			m.alliance = nil
			m.conn.send(wire.Packet(smLeaveGroupMember))
			m.conn.send(systemMessage(msgAllianceDispersed))
		}
		s.ids.release(a.id)
		a.members = nil
		return
	}
	if a.captain == p {
		a.captain = a.members[0]
	}
	s.broadcastAllianceInfo(a, nil)
}

// sayToAlliance is CM_CHAT_MESSAGE_PUBLIC for alliance chat: every member hears it.
func (s *Server) sayToAlliance(p *player, text string) {
	if p.alliance == nil {
		return
	}
	for _, m := range p.alliance.members {
		m.conn.send(s.chatMessage(p, m.conn, text, chatAlliance))
	}
}

// updateAllianceOf tells the alliance of a change to one of its members, if it is in one.
func (s *Server) updateAllianceOf(p *player) {
	if p.alliance != nil {
		s.updateAlliance(p.alliance, p, 1)
	}
}
