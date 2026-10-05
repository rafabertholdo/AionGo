package game

import (
	"slices"
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// Groups of up to six players: AL-Game's PlayerGroup and GroupService.
// ponytail: a player who leaves the world leaves its group at once (AL-Game keeps its place for
// playergroup.removetime). Bid distribution remains unported.

func init() {
	handlers[cmInviteToGroup] = (*conn).inviteToGroup
	handlers[cmPlayerStatusInfo] = (*conn).playerStatusInfo
	handlers[cmGroupDistribution] = (*conn).groupDistribution
	handlers[cmGroupResponse] = func(*conn, *wire.Reader) {}
}

const (
	maxGroupSize            = 6
	groupMaxDistance        = 100
	questionGroupInvitation = 60000
	groupInterval           = 2 * time.Second
)

// Loot rules (LootRuleType), and the default ones of a new group (LootGroupRules).
const (
	lootFreeForAll = 0
	lootRoundRobin = 1
	lootLeader     = 2
)

// groupEvent is a GroupEvent; the client is told its wire id, which is the same for three of them.
type groupEvent int

const (
	groupLeave groupEvent = iota
	groupMovement
	groupEnter
	groupUpdate
	groupChangeLeader
)

func (e groupEvent) id() byte {
	switch e {
	case groupLeave:
		return 0
	case groupMovement:
		return 1
	}
	return 13
}

// The message ids of groups.

const (
	msgGroupInvited             = 1300173 // "You invited %0 to join your group."
	msgGroupRejected            = 1300161
	msgGroupLeaderChanged       = 1300155
	msgGroupMemberLeft          = 1300168
	msgGroupYouLeft             = 1300043
	msgGroupDisbanded           = 1300167
	msgGroupFull                = 1300152
	msgGroupOnlyLeader          = 1300160
	msgGroupOffline             = 1300159
	msgGroupOtherRace           = 1300188
	msgGroupSelf                = 1300162
	msgGroupTargetDead          = 1300044
	msgGroupYouDead             = 1300163
	msgGroupInAnother           = 1300169
	msgGroupDenied              = 1390116
	chatGroup                   = 0x05
	chatGroupLeader             = 0x07
	deniedGroup           int32 = 4 // DeniedStatus.GROUP
)

// group is a PlayerGroup.
type group struct {
	id           int32
	leader       *player
	members      []*player // in the order they joined
	rule         int32     // LootRuleType
	robin        int
	distribution int32
	qualityRules *[7]int32 // nil uses Java's default quality rules
}

func (g *group) full() bool { return len(g.members) >= maxGroupSize }

// inviteToGroup is CM_INVITE_TO_GROUP: a group's invitation (type 0), or an alliance's (10).
func (c *conn) inviteToGroup(r *wire.Reader) {
	kind, name := r.C(), convertName(r.S())
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		invited := s.playerNamed(name)
		switch {
		case invited == nil:
			p.conn.send(systemMessage(msgPlayerOffline, name))
		case invited.settings.Deny&deniedGroup != 0:
			p.conn.send(systemMessage(msgGroupDenied, invited.Name))
		case kind == 10:
			s.inviteToAlliance(p, invited)
		case kind == 0:
			s.inviteToGroup(p, invited)
		}
	})
}

// inviteToGroup is GroupService.invitePlayerToGroup, with PlayerRestrictions.canInviteToGroup.
func (s *Server) inviteToGroup(p, invited *player) {
	if s.restrictedInPrison(p, "invite members to group") {
		return
	}
	switch g := p.group; {
	case g != nil && g.full():
		p.conn.send(systemMessage(msgGroupFull))
		return
	case g != nil && g.leader != p:
		p.conn.send(systemMessage(msgGroupOnlyLeader))
		return
	case invited.Race != p.Race:
		p.conn.send(systemMessage(msgGroupOtherRace))
		return
	case invited == p:
		p.conn.send(systemMessage(msgGroupSelf))
		return
	case invited.dead:
		p.conn.send(systemMessage(msgGroupTargetDead))
		return
	case p.dead:
		p.conn.send(systemMessage(msgGroupYouDead))
		return
	case invited.group != nil:
		p.conn.send(systemMessage(msgGroupInAnother, invited.Name))
		return
	}
	asked := invited.putRequest(questionGroupInvitation, func(accepted bool) {
		if !accepted {
			p.conn.send(systemMessage(msgGroupRejected, invited.Name))
			return
		}
		if s.spawned[p.ID] == nil || s.spawned[invited.ID] == nil || invited.group != nil || (p.group != nil && p.group.full()) {
			return
		}
		p.conn.send(systemMessage(msgGroupInvited, invited.Name))
		if p.group == nil {
			s.newGroup(p)
		}
		s.addToGroup(p.group, invited)
	})
	if asked {
		invited.conn.send(questionWindow(questionGroupInvitation, 0, p.Name))
	}
}

func (s *Server) newGroup(leader *player) {
	g := &group{id: s.ids.nextID(), leader: leader, members: []*player{leader}, rule: lootRoundRobin}
	leader.group = g
	leader.conn.send(groupInfo(g))
	s.repostFindGroup(leader, s.takeFindGroup(leader))
}

// groupInfo is SM_GROUP_INFO: the group's leader and its loot rules.
func groupInfo(g *group) *wire.Writer {
	w := wire.Packet(smGroupInfo)
	w.D(g.id)
	w.D(g.leader.ID)
	w.D(g.rule)
	w.D(g.distribution)
	for _, above := range g.lootQualityRules() {
		w.D(above)
	}
	w.D(0)
	w.H(0)
	w.C(0)
	return w
}

// groupMemberInfo is SM_GROUP_MEMBER_INFO.
func (s *Server) groupMemberInfo(g *group, p *player, event groupEvent) *wire.Writer {
	w := wire.Packet(smGroupMemberInfo)
	w.D(g.id)
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
	w.C(event.id())
	w.H(1) // channel
	if event == groupMovement {
		return w
	}
	w.S(p.Name)
	w.H(0)
	w.H(0)
	list := p.fx.list()
	w.H(uint16(len(list)))
	for _, e := range list {
		w.D(e.effector.cid())
		w.H(uint16(e.tmpl.ID))
		w.C(byte(e.level))
		w.C(byte(e.slot()))
		w.D(e.elapsed())
	}
	w.D(0x25F7)
	return w
}

// addToGroup is PlayerGroup.addPlayerToGroup.
func (s *Server) addToGroup(g *group, p *player) {
	s.removePlayerFindGroups(p)
	g.members = append(g.members, p)
	p.group = g
	p.conn.send(groupInfo(g))
	s.updateGroup(g, p, groupEnter)
	if g.full() {
		race, _ := raceGender(g.leader.Character)
		s.removeFindGroup(race, findGroupRecruit, g.id)
	}
}

// updateGroup is PlayerGroup.updateGroupUIToEvent.
func (s *Server) updateGroup(g *group, subject *player, event groupEvent) {
	others := func(fn func(m *player)) {
		for _, m := range slices.Clone(g.members) {
			if m != subject {
				fn(m)
			}
		}
	}
	switch event {
	case groupLeave:
		changed := false
		if subject == g.leader && len(g.members) > 0 {
			g.leader = g.members[0]
			changed = true
		}
		for _, m := range g.members {
			if changed {
				m.conn.send(groupInfo(g))
				m.conn.send(systemMessage(msgGroupLeaderChanged))
			}
			if m != subject {
				m.conn.send(s.groupMemberInfo(g, subject, event))
			}
			if len(g.members) > 1 {
				m.conn.send(systemMessage(msgGroupMemberLeft, subject.Name))
			}
		}
		s.tellSubject(g, subject, event)
		subject.conn.send(systemMessage(msgGroupYouLeft))
	case groupEnter:
		s.tellSubject(g, subject, event)
		others(func(m *player) { m.conn.send(s.groupMemberInfo(g, subject, event)) })
	default:
		others(func(m *player) { m.conn.send(s.groupMemberInfo(g, subject, event)) })
	}
}

// tellSubject is PlayerGroup.eventToSubjective: the player is told of each of the others.
func (s *Server) tellSubject(g *group, subject *player, event groupEvent) {
	for _, m := range g.members {
		if m != subject {
			subject.conn.send(s.groupMemberInfo(g, m, event))
		}
	}
}

// updateGroupOf tells the group of a change to one of its members, if it is in one.
func (s *Server) updateGroupOf(p *player, event groupEvent) {
	if p.group != nil {
		s.updateGroup(p.group, p, event)
	}
}

// leaveGroup is GroupService.removePlayerFromGroup: the group ends when one is left.
func (s *Server) leaveGroup(p *player) {
	g := p.group
	if g == nil {
		return
	}
	g.members = slices.DeleteFunc(g.members, func(m *player) bool { return m == p })
	p.group = nil
	s.leaveLootRolls(p)
	s.updateGroup(g, p, groupLeave)
	p.conn.send(wire.Packet(smLeaveGroupMember))
	if len(g.members) < 2 {
		s.disbandGroup(g)
	}
}

// disbandGroup is GroupService.disbandGroup.
func (s *Server) disbandGroup(g *group) {
	if len(g.members) > 0 {
		race, _ := raceGender(g.members[0].Character)
		s.removeFindGroup(race, findGroupRecruit, g.id)
	}
	s.ids.release(g.id)
	for _, m := range g.members {
		m.group = nil
		s.leaveLootRolls(m)
		m.conn.send(systemMessage(msgGroupDisbanded))
		w := wire.Packet(smLeaveGroupMember)
		w.D(0)
		w.D(0)
		w.H(0)
		w.C(0)
		m.conn.send(w)
	}
	g.members = nil
}

// playerStatusInfo is CM_PLAYER_STATUS_INFO: LFG status, group and alliance actions.
func (c *conn) playerStatusInfo(r *wire.Reader) {
	status, id := r.C(), r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		switch status {
		case 9: // the client's "looking for group" toggle: 2 turns it on
			p.lookingForGroup = id == 2
			return
		case 12, 14, 15, 19, 23, 24:
			s.allianceStatus(p, status, id)
			return
		}
		target := p
		if id != 0 {
			target = s.spawned[id]
		}
		if target == nil || target.group == nil || target.group != p.group {
			return
		}
		g := p.group
		// Only the leader can remove another member or hand over the lead.
		if (target != p || status == 3) && g.leader != p {
			return
		}
		switch status {
		case 2, 6:
			s.leaveGroup(target)
		case 3:
			g.leader = target
			for _, m := range g.members {
				m.conn.send(groupInfo(g))
				if m == target {
					m.conn.send(systemMessage(msgGroupLeaderChanged))
				}
				m.conn.send(s.groupMemberInfo(g, target, groupChangeLeader))
			}
		}
	})
}

// groupDistribution is CM_GROUP_DISTRIBUTION: the player shares kinah equally among the others.
func (c *conn) groupDistribution(r *wire.Reader) {
	amount := int64(r.D())
	if r.Err != nil || amount < 1 {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		g := p.group
		if g == nil || p.kinah.Count < amount || int64(len(g.members)-1) > amount {
			return
		}
		reward := amount / int64(len(g.members)-1)
		for _, m := range g.members {
			if m == p {
				s.decreaseKinah(m, amount)
			} else {
				s.increaseKinah(m, reward)
			}
		}
	})
}

// sayToGroup is CM_CHAT_MESSAGE_PUBLIC for group and leader chat: every member hears it.
func (s *Server) sayToGroup(p *player, text string, kind byte) {
	if p.group == nil {
		return
	}
	for _, m := range p.group.members {
		m.conn.send(s.chatMessage(p, m.conn, text, kind))
	}
}

// groupReward is GroupService.doReward: experience is shared by those near the monster, and the loot goes by the rules.
func (s *Server) groupReward(g *group, o *object) {
	var near []*player
	var levelSum, highest int32
	for _, m := range g.members {
		if s.spawned[m.ID] == nil || m.dead || !inRange3D(m, o, groupMaxDistance) {
			continue
		}
		near = append(near, m)
		levelSum += int32(m.level)
		highest = max(highest, int32(m.level))
	}
	if len(near) == 0 {
		return
	}
	exp := int64(o.npc.Stats.MaxXP) * int64(xpPercent(o.npc.Level-highest)) / 100
	bonus := int64(100 + (len(near)-1)*10)
	for _, m := range near {
		reward := exp * bonus * int64(m.level) / (int64(levelSum) * 100)
		if highest-int32(m.level) >= 10 {
			reward = 0
		}
		s.giveExp(m, reward)
		s.recordQuestKill(o, m)
	}
	s.registerDropFor(o, g.leader, highest, s.lootRecipients(g, o, near))
	o.loot.group = g
	o.loot.eligible = slices.Clone(near)
}

// lootRecipients is GroupService.getMembersToRegistrateByRules: who may take the loot.
func (s *Server) lootRecipients(g *group, o *object, near []*player) []*player {
	switch g.rule {
	case lootRoundRobin:
		g.robin = (g.robin + 1) % len(g.members)
		if m := g.members[g.robin]; inRange3D(m, o, groupMaxDistance) {
			return []*player{m}
		}
	case lootLeader:
		return []*player{g.leader}
	}
	return slices.Clone(g.members)
}
