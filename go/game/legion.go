package game

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"aionlightning/game/store"
	"aionlightning/wire"
)

// Legions: AL-Game's LegionService, Legion and LegionMember. A legion has members ranked as brigade general,
// centurions and legionaries, an announcement board, and a level.
// The warehouse isn't ported: AL-Game turns it off (legion.warehouse). Uploaded emblem images are ignored as in
// AL-Game, whose LegionService.uploadEmblemInfo only sets a flag nothing reads and whose uploadEmblemData is commented out.

func init() {
	handlers[cmLegion] = (*conn).legionRequest
	handlers[cmLegionModifyEmblem] = (*conn).legionModifyEmblem
	handlers[cmLegionSendEmblem] = (*conn).legionSendEmblem
	handlers[cmLegionTabs] = (*conn).legionTabs
	for _, op := range []byte{cmLegionUploadEmblem, cmLegionUploadInfo} {
		handlers[op] = func(*conn, *wire.Reader) {}
	}
}

// legionSaver keeps legions; store.Store does.
type legionSaver interface {
	LegionByID(int32) (*store.Legion, error)
	LegionNameUsed(string) (bool, error)
	InsertLegion(id int32, name string) error
	UpdateLegion(*store.Legion) error
	DeleteLegion(int32) error
	LegionMembers(legionID int32) ([]store.LegionMemberRow, error)
	LegionMemberOf(playerID int32) (*store.LegionMemberRow, error)
	InsertLegionMember(legionID, playerID int32, rank string) error
	UpdateLegionMember(playerID int32, nickname, rank, selfIntro string) error
	DeleteLegionMember(playerID int32) error
	LegionAnnouncements(legionID int32) ([]store.Announcement, error)
	InsertAnnouncement(legionID int32, text string, at time.Time) error
	LegionHistory(legionID int32) ([]store.HistoryEntry, error)
	InsertHistory(legionID int32, h store.HistoryEntry) error
}

// Legion ranks (LegionRank) by their id, and the names the database has.
const (
	rankBrigadeGeneral = 0
	rankCenturion      = 1
	rankLegionary      = 2
)

var rankNames = []string{"BRIGADE_GENERAL", "CENTURION", "LEGIONARY"}

func rankOf(name string) int {
	if i := slices.Index(rankNames, name); i >= 0 {
		return i
	}
	return rankLegionary
}

// gameplay values of legions.properties.
const (
	legionCreateKinah = 10000
	legionDisbandTime = 86400
)

// By level (from 1): the members that fit, and what the next level asks of kinah, members and contribution.
var (
	legionMaxMembers   = []int{30, 60, 90, 120, 150}
	legionLevelKinah   = []int64{100000, 1000000, 2000000, 6000000}
	legionLevelMembers = []int{10, 20, 30, 40}
	legionLevelPoints  = []int32{0, 20000, 100000, 500000}
)

var (
	legionNamePattern   = regexp.MustCompile(`^[a-zA-Z ]{2,16}$`)
	legionIntroPattern  = regexp.MustCompile(`^[a-zA-Z]{2,25}$`)
	legionNickPattern   = regexp.MustCompile(`^[a-zA-Z]{2,10}$`)
	legionNoticePattern = regexp.MustCompile(`^[a-zA-Z .,]{2,120}$`)
)

// System messages and questions of legions.
const (
	msgLegionCreated       = 1300235
	msgLegionBadName       = 1300228
	msgLegionNameTaken     = 1300233
	msgLegionAlreadyIn     = 1300232
	msgLegionNoKinah       = 1300231
	msgLegionNoUser        = 1300253
	msgLegionRejectedInv   = 1390118
	msgLegionInviteDead    = 1300250
	msgLegionInviteSelf    = 1300254
	msgLegionIsMember      = 1300255
	msgLegionIsOther       = 1300256
	msgLegionIncorrect     = 1300627
	msgLegionJoined        = 1300260
	msgLegionFull          = 1300257
	msgLegionInviteDenied  = 1300259
	msgLegionBusy          = 1300325
	msgLegionInviteSent    = 1300258
	msgLegionAnnounce      = 1400019
	msgLegionKickSelf      = 1300243
	msgLegionKickGeneral   = 1300249
	msgLegionNoticeDone    = 1300277
	msgLegionIntroDone     = 1300282
	msgLegionLogin         = 1400133
	msgLegionRankNoUser    = 1300264
	msgLegionRankNoRight   = 1300262
	msgLegionRankSelf      = 1300263
	msgLegionMasterNoUser  = 1300270
	msgLegionMasterSelf    = 1300271
	msgLegionMasterDecline = 1300332
	msgLegionMasterBusy    = 1300331
	msgLegionMasterOffer   = 1300330
	msgLegionMasterNow     = 1300273
	msgLegionRankUp        = 1300267
	msgLegionRankDown      = 1300268
	msgLegionKicked        = 1300246
	msgLegionKickedMember  = 1300247
	msgLegionLeft          = 1300241
	msgLegionMemberLeft    = 900699
	questionLegionInvite   = 80001
	questionLegionMaster   = 80011
	dialogCreateLegion     = 5
	chatLegion             = 0x08
	legionPermCentBase     = 0x60
	msgLegionMaxLevel      = 1300316
	msgLegionLevelKinah    = 1300319
	msgLegionLevelMembers  = 1300318
	msgLegionLevelPoints   = 1300317
	msgLegionLevelUp       = 900700
	msgLegionOnlyMaster    = 1300300
	msgLegionDisbandAsked  = 1300304
	msgLegionDisbanding    = 1300303
	msgLegionRecreated     = 1300307
	msgLegionMustAppoint   = 1300238
	questionLegionDisband  = 80008
	questionLegionRecreate = 80009
	dialogDisbandLegion    = 6
	dialogRecreateLegion   = 7
)

// legion is a Legion with its members, whether they are here or not.
type legion struct {
	store.Legion
	members       []*legionMember
	announcements []store.Announcement
	history       []store.HistoryEntry // newest first
}

// legionMember is a LegionMember, with the player's details LegionMemberEx keeps.
type legionMember struct {
	store.LegionMemberRow
	rank int
}

func (l *legion) member(playerID int32) *legionMember {
	for _, m := range l.members {
		if m.PlayerID == playerID {
			return m
		}
	}
	return nil
}

func (l *legion) memberNamed(name string) *legionMember {
	for _, m := range l.members {
		if strings.EqualFold(m.Name, name) {
			return m
		}
	}
	return nil
}

// hasRights is LegionMember.hasRights, with its falls through the cases.
func (l *legion) hasRights(m *legionMember, kind int) bool {
	if m.rank == rankBrigadeGeneral {
		return true
	}
	if m.rank != rankCenturion {
		return false
	}
	c1 := int(l.CenturionPerm1) - legionPermCentBase
	c2 := int(l.CenturionPerm2)
	in := func(v int, allowed ...int) bool { return slices.Contains(allowed, v) }
	for k := kind; k <= 6; k++ {
		switch k {
		case 1:
			if in(c1, 0x08, 0x0C, 0x14, 0x1C, 0x18) {
				return true
			}
		case 2:
			if in(c1, 0x10, 0x0C, 0x18, 0x1C) {
				return true
			}
		case 3:
			if in(c1, 0x04, 0x14, 0x1C) {
				return true
			}
		case 4:
			if in(c2, 0x02, 0x0A, 0x0C, 0x0E) {
				return true
			}
		case 5:
			if in(c2, 0x04, 0x06, 0x0C, 0x0E) {
				return true
			}
		case 6:
			if in(c2, 0x08, 0x0A, 0x0C, 0x0E) {
				return true
			}
		}
	}
	return false
}

// Kinds of rights (LegionService's INVITE, KICK, ...).
const (
	rightInvite       = 1
	rightKick         = 2
	rightAnnouncement = 4
)

// online is the legion's members who are in the world.
func (s *Server) online(l *legion) []*player {
	var list []*player
	for _, m := range l.members {
		if p := s.spawned[m.PlayerID]; p != nil {
			list = append(list, p)
		}
	}
	return list
}

func (s *Server) tellLegion(l *legion, w *wire.Writer) {
	for _, p := range s.online(l) {
		p.conn.send(w)
	}
}

// legionOf is the legion the id names, loaded once.
func (s *Server) legionOf(id int32) (*legion, error) {
	if l := s.legions[id]; l != nil {
		return l, nil
	}
	row, err := s.legionDB.LegionByID(id)
	if err != nil || row == nil {
		return nil, err
	}
	l := &legion{Legion: *row}
	members, err := s.legionDB.LegionMembers(id)
	if err != nil {
		return nil, err
	}
	for _, m := range members {
		l.members = append(l.members, &legionMember{LegionMemberRow: m, rank: rankOf(m.Rank)})
	}
	if l.announcements, err = s.legionDB.LegionAnnouncements(id); err != nil {
		return nil, err
	}
	if l.history, err = s.legionDB.LegionHistory(id); err != nil {
		return nil, err
	}
	slices.Reverse(l.history)
	if s.legions == nil {
		s.legions = map[int32]*legion{}
	}
	s.legions[id] = l
	return l, nil
}

// loadLegion is LegionService.getLegionMember for a player who logs in: it is put back in its legion.
func (s *Server) loadLegion(p *player) error {
	row, err := s.legionDB.LegionMemberOf(p.ID)
	if err != nil || row == nil {
		return err
	}
	l, err := s.legionOf(row.LegionID)
	if err != nil || l == nil {
		return err
	}
	p.legion, p.member = l, l.member(p.ID)
	p.legionID = l.ID
	if l.DisbandTime != 0 && int32(time.Now().Unix()) > l.DisbandTime {
		s.disbandLegion(l)
	}
	return nil
}

// legionLogin is LegionService.onLogin.
func (s *Server) legionLogin(p *player) {
	l, m := p.legion, p.member
	if l == nil || m == nil {
		return
	}
	m.Exp, m.WorldID = p.Exp, p.WorldID
	p.conn.send(s.legionAddMember(p, false, 0, ""))
	p.conn.send(legionInfo(l))
	p.conn.send(legionEmblem(l))
	s.tellLegion(l, s.legionUpdateMember(p, 0, ""))
	for _, other := range s.online(l) {
		if other != p {
			other.conn.send(systemMessage(msgLegionLogin, p.Name))
		}
	}
	p.conn.send(s.legionAddMember(p, true, 0, ""))
	p.conn.send(s.legionMemberList(l))
	s.legionAnnouncementTo(p, l)
	if l.DisbandTime != 0 {
		p.conn.send(legionEditTime(0x06, l.DisbandTime))
	}
}

// legionLogout is LegionService.onLogout.
func (s *Server) legionLogout(p *player) {
	l, m := p.legion, p.member
	if l == nil || m == nil {
		return
	}
	m.LastOnline, m.Exp, m.WorldID, m.Class = time.Now(), p.Exp, p.WorldID, p.Class
	s.tellLegion(l, s.legionUpdateMemberOffline(p))
	if err := s.legionDB.UpdateLegionMember(p.ID, m.Nickname, rankNames[m.rank], m.SelfIntro); err != nil {
		s.log.Error("saving a legion member", "err", err)
	}
	s.saveLegion(l)
}

func (s *Server) saveLegion(l *legion) {
	if err := s.legionDB.UpdateLegion(&l.Legion); err != nil {
		s.log.Error("saving a legion", "legion", l.ID, "err", err)
	}
}

// ---- packets

func legionInfo(l *legion) *wire.Writer {
	w := wire.Packet(smLegionInfo)
	w.S(l.Name)
	w.C(byte(l.Level))
	w.D(0) // ranking
	w.C(byte(l.CenturionPerm1))
	w.C(byte(l.CenturionPerm2))
	w.C(0x40)
	w.C(byte(l.LegionarPerm2))
	w.D(l.Contribution)
	w.D(0)
	w.D(0)
	w.D(0)
	for i := 0; i < len(l.announcements) && i < 7; i++ {
		a := l.announcements[len(l.announcements)-1-i]
		w.S(a.Text)
		w.D(int32(a.At.Unix()))
	}
	w.H(0)
	return w
}

func legionEdit(kind byte) *wire.Writer {
	w := wire.Packet(smLegionEdit)
	w.C(kind)
	return w
}

func legionEditTime(kind byte, t int32) *wire.Writer {
	w := legionEdit(kind)
	w.D(t)
	return w
}

func (s *Server) legionMemberList(l *legion) *wire.Writer {
	w := wire.Packet(smLegionMemberlist)
	w.C(1)
	w.H(uint16(65536 - len(l.members)))
	for _, m := range l.members {
		online := s.spawned[m.PlayerID] != nil
		last := int32(0)
		if !online && !m.LastOnline.IsZero() {
			last = int32(m.LastOnline.Unix())
		}
		w.D(m.PlayerID)
		w.S(m.Name)
		w.C(byte(classIDs[m.Class]))
		w.D(int32(s.data.Level(m.Exp)))
		w.C(byte(m.rank))
		w.D(m.WorldID)
		w.Bool(online)
		w.S(m.SelfIntro)
		w.S(m.Nickname)
		w.D(last)
	}
	return w
}

func (s *Server) legionAddMember(p *player, isMember bool, msg int32, text string) *wire.Writer {
	w := wire.Packet(smLegionAddMember)
	w.D(p.ID)
	w.S(p.Name)
	w.C(byte(p.member.rank))
	w.Bool(isMember)
	w.C(byte(classIDs[p.Class]))
	w.C(byte(p.level))
	w.D(p.WorldID)
	w.D(msg)
	w.S(text)
	return w
}

func (s *Server) legionUpdateMember(p *player, msg int32, text string) *wire.Writer {
	w := wire.Packet(smLegionUpdateMember)
	w.D(p.ID)
	w.C(byte(p.member.rank))
	w.C(byte(classIDs[p.Class]))
	w.C(byte(p.level))
	w.D(p.WorldID)
	w.C(1)
	w.D(0)
	w.D(msg)
	w.S(text)
	return w
}

func (s *Server) legionUpdateMemberOffline(p *player) *wire.Writer {
	w := wire.Packet(smLegionUpdateMember)
	w.D(p.ID)
	w.C(byte(p.member.rank))
	w.C(byte(classIDs[p.Class]))
	w.C(byte(p.level))
	w.D(p.WorldID)
	w.C(0)
	w.D(int32(time.Now().Unix()))
	w.D(0)
	w.S("")
	return w
}

func legionTitle(p *player, id int32, name string, rank byte) *wire.Writer {
	w := wire.Packet(smLegionUpdateTitle)
	w.D(p.ID)
	w.D(id)
	w.S(name)
	w.C(rank)
	return w
}

func legionLeave(msg, player int32, name, name1 string) *wire.Writer {
	w := wire.Packet(smLegionLeaveMember)
	w.D(player)
	w.C(0)
	w.D(0)
	w.D(msg)
	w.S(name)
	w.S(name1)
	return w
}

func (s *Server) legionAnnouncementTo(p *player, l *legion) {
	if n := len(l.announcements); n > 0 {
		a := l.announcements[n-1]
		p.conn.send(systemMessage(msgLegionAnnounce, a.Text, int32(a.At.Unix()), 2))
	}
}

// ---- requests

// legionRequest is CM_LEGION: its first byte says what the player asks for.
func (c *conn) legionRequest(r *wire.Reader) {
	op := r.C()
	var name, text string
	var rank int32
	var perms [4]byte
	switch op {
	case 0x00, 0x01, 0x04, 0x05, 0x07:
		r.D()
		name = r.S()
	case 0x02, 0x0E:
		r.D()
		r.H()
	case 0x06:
		rank = r.D()
		name = r.S()
	case 0x09, 0x0A:
		r.D()
		text = r.S()
	case 0x0D:
		perms = [4]byte{r.C(), r.C(), r.C(), r.C()}
	case 0x0F:
		name = r.S()
		text = r.S()
	}
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		if p.legion == nil {
			if op == 0x00 {
				s.createLegion(p, name)
			}
			return
		}
		l := p.legion
		switch op {
		case 0x01:
			s.legionInvite(p, name)
		case 0x02:
			s.legionLeaveOwn(p)
		case 0x04:
			s.legionKick(p, name)
		case 0x05:
			s.legionAppointGeneral(p, name)
		case 0x06:
			s.legionAppointRank(p, name, int(rank))
		case 0x08:
			p.conn.send(legionInfo(l))
		case 0x09:
			s.legionAnnounce(p, text)
		case 0x0A:
			s.legionIntro(p, text)
		case 0x0D:
			if p.member.rank == rankBrigadeGeneral {
				s.legionPermissions(l, int32(perms[3]), int32(perms[0]), int32(perms[1]))
			}
		case 0x0E:
			s.legionLevelUp(p)
		case 0x0F:
			s.legionNickname(p, name, text)
		}
	})
}

// createLegion is LegionService.createLegion.
func (s *Server) createLegion(p *player, name string) {
	used, _ := s.legionDB.LegionNameUsed(name)
	switch {
	case !legionNamePattern.MatchString(name):
		p.conn.send(systemMessage(msgLegionBadName))
		return
	case used:
		p.conn.send(systemMessage(msgLegionNameTaken))
		return
	case p.legion != nil:
		p.conn.send(systemMessage(msgLegionAlreadyIn))
		return
	case p.kinah.Count < legionCreateKinah:
		p.conn.send(systemMessage(msgLegionNoKinah))
		return
	}
	id := s.ids.nextID()
	if err := s.legionDB.InsertLegion(id, name); err != nil {
		s.log.Error("creating a legion", "err", err)
		s.ids.release(id)
		return
	}
	l := &legion{Legion: store.Legion{ID: id, Name: name, Level: 1, LegionarPerm2: 0, CenturionPerm1: legionPermCentBase, CenturionPerm2: 0}}
	if s.legions == nil {
		s.legions = map[int32]*legion{}
	}
	s.legions[id] = l
	s.decreaseKinah(p, legionCreateKinah)
	s.addHistory(l, "", "CREATE")
	s.addLegionMember(l, p, rankBrigadeGeneral)
	p.conn.send(systemMessage(msgLegionCreated, name))
}

// addLegionMember is LegionService.addLegionMember: the player joins the legion, and everyone is told.
func (s *Server) addLegionMember(l *legion, p *player, rank int) {
	if err := s.legionDB.InsertLegionMember(l.ID, p.ID, rankNames[rank]); err != nil {
		s.log.Error("adding a legion member", "err", err)
		return
	}
	m := &legionMember{LegionMemberRow: store.LegionMemberRow{PlayerID: p.ID, LegionID: l.ID, Name: p.Name, Exp: p.Exp, Class: p.Class,
		WorldID: p.WorldID}, rank: rank}
	l.members = append(l.members, m)
	p.legion, p.member, p.legionID = l, m, l.ID
	s.addHistory(l, p.Name, "JOIN")
	p.conn.send(legionInfo(l))
	s.tellLegion(l, s.legionAddMember(p, false, 0, ""))
	p.conn.send(s.legionMemberList(l))
	p.broadcast(legionTitle(p, l.ID, l.Name, byte(rank)), true)
	p.conn.send(legionEdit(0x08))
}

// legionInvite is LegionService.invitePlayerToLegion, with the restrictions.
func (s *Server) legionInvite(p *player, name string) {
	l := p.legion
	target := s.playerNamed(convertName(name))
	switch {
	case target == nil:
		p.conn.send(systemMessage(msgLegionNoUser))
		return
	case target.settings.Deny&deniedGuild != 0:
		p.conn.send(systemMessage(msgLegionRejectedInv, target.Name))
		return
	case p.dead:
		p.conn.send(systemMessage(msgLegionInviteDead))
		return
	case target == p:
		p.conn.send(systemMessage(msgLegionInviteSelf))
		return
	case target.legion != nil && target.legion == l:
		p.conn.send(systemMessage(msgLegionIsMember, target.Name))
		return
	case target.legion != nil:
		p.conn.send(systemMessage(msgLegionIsOther, target.Name))
		return
	case !l.hasRights(p.member, rightInvite) || target.Race != p.Race:
		return
	}
	asked := target.putRequest(questionLegionInvite, func(accepted bool) {
		if !accepted {
			p.conn.send(systemMessage(msgLegionInviteDenied, target.Name))
			return
		}
		if s.spawned[target.ID] == nil {
			p.conn.send(systemMessage(msgLegionIncorrect))
			return
		}
		if len(l.members) >= legionMaxMembers[l.Level-1] || target.legion != nil {
			p.conn.send(systemMessage(msgLegionFull))
			return
		}
		s.addLegionMember(l, target, rankLegionary)
		p.conn.send(systemMessage(msgLegionJoined, target.Name))
		s.legionAnnouncementTo(target, l)
	})
	if !asked {
		p.conn.send(systemMessage(msgLegionBusy))
		return
	}
	p.conn.send(systemMessage(msgLegionInviteSent, target.Name))
	target.conn.send(questionWindow(questionLegionInvite, 0, l.Name, itoa(l.Level), p.Name))
}

const deniedGuild int32 = 8 // DeniedStatus.GUILD

func itoa(n int32) string { return strconv.Itoa(int(n)) }

// removeLegionMember is LegionService.removeLegionMember: the member is out of the legion, and the legion is told.
func (s *Server) removeLegionMember(l *legion, m *legionMember, kick bool, byName string) {
	if err := s.legionDB.DeleteLegionMember(m.PlayerID); err != nil {
		s.log.Error("removing a legion member", "err", err)
		return
	}
	l.members = slices.DeleteFunc(l.members, func(o *legionMember) bool { return o == m })
	if p := s.spawned[m.PlayerID]; p != nil {
		p.broadcast(legionTitle(p, 0, "", 2), true)
	}
	if kick {
		s.addHistory(l, m.Name, "KICK")
		s.tellLegion(l, legionLeave(msgLegionKickedMember, m.PlayerID, byName, m.Name))
	} else {
		s.tellLegion(l, legionLeave(msgLegionMemberLeft, m.PlayerID, m.Name, ""))
	}
}

func (p *player) leftLegion() { p.legion, p.member, p.legionID = nil, nil, 0 }

// legionLeaveOwn is the member's own leaving.
func (s *Server) legionLeaveOwn(p *player) {
	l := p.legion
	// The brigade general can't leave without appointing another (LegionRestrictions.canLeave).
	if p.member.rank == rankBrigadeGeneral {
		p.conn.send(systemMessage(msgLegionMustAppoint))
		return
	}
	s.removeLegionMember(l, p.member, false, "")
	p.conn.send(legionLeave(msgLegionLeft, 0, l.Name, ""))
	p.leftLegion()
}

// legionKick is the kick of a member by name.
func (s *Server) legionKick(p *player, name string) {
	l := p.legion
	target := l.memberNamed(convertName(name))
	switch {
	case target == nil:
		return
	case target.PlayerID == p.ID:
		p.conn.send(systemMessage(msgLegionKickSelf))
		return
	case target.rank == rankBrigadeGeneral:
		p.conn.send(systemMessage(msgLegionKickGeneral))
		return
	case target.rank == p.member.rank || !l.hasRights(p.member, rightKick):
		return
	}
	s.removeLegionMember(l, target, true, p.Name)
	if t := s.spawned[target.PlayerID]; t != nil {
		t.conn.send(legionLeave(msgLegionKicked, 0, l.Name, ""))
		t.leftLegion()
	}
}

// legionAppointGeneral is LegionService.appointBrigadeGeneral: the target is asked to take over.
func (s *Server) legionAppointGeneral(p *player, name string) {
	l := p.legion
	target := s.playerNamed(convertName(name))
	switch {
	case p.member.rank != rankBrigadeGeneral:
		p.conn.send(systemMessage(msgLegionRankNoRight))
		return
	case target == nil:
		p.conn.send(systemMessage(msgLegionMasterNoUser))
		return
	case target == p:
		p.conn.send(systemMessage(msgLegionMasterSelf))
		return
	case target.legion != l:
		return
	}
	asked := target.putRequest(questionLegionMaster, func(accepted bool) {
		if !accepted {
			p.conn.send(systemMessage(msgLegionMasterDecline, target.Name))
			return
		}
		if s.spawned[target.ID] == nil || target.member.rank == rankBrigadeGeneral {
			return
		}
		p.member.rank = rankCenturion
		s.tellLegion(l, s.legionUpdateMember(p, 0, ""))
		target.member.rank = rankBrigadeGeneral
		s.tellLegion(l, s.legionUpdateMember(target, msgLegionMasterNow, target.Name))
		s.saveMember(p)
		s.saveMember(target)
		s.addHistory(l, target.Name, "APPOINTED")
	})
	if !asked {
		p.conn.send(systemMessage(msgLegionMasterBusy))
		return
	}
	p.conn.send(systemMessage(msgLegionMasterOffer, target.Name))
	target.conn.send(questionWindow(questionLegionMaster, p.ID))
}

func (s *Server) saveMember(p *player) {
	m := p.member
	if err := s.legionDB.UpdateLegionMember(p.ID, m.Nickname, rankNames[m.rank], m.SelfIntro); err != nil {
		s.log.Error("saving a legion member", "err", err)
	}
}

// legionAppointRank is LegionService.appointRank: a legionary becomes a centurion and a centurion a legionary.
func (s *Server) legionAppointRank(p *player, name string, rank int) {
	l := p.legion
	target := s.playerNamed(convertName(name))
	switch {
	case p.member.rank != rankBrigadeGeneral:
		p.conn.send(systemMessage(msgLegionRankNoRight))
		return
	case target == nil:
		p.conn.send(systemMessage(msgLegionRankNoUser))
		return
	case target == p:
		p.conn.send(systemMessage(msgLegionRankSelf))
		return
	case target.legion != l:
		return
	}
	msg := int32(msgLegionRankDown)
	if rank == rankCenturion && target.member.rank == rankLegionary {
		target.member.rank = rankCenturion
		msg = msgLegionRankUp
	} else {
		target.member.rank = rankLegionary
	}
	s.tellLegion(l, s.legionUpdateMember(target, msg, target.Name))
	s.saveMember(target)
}

// legionAnnounce is LegionService.changeAnnouncement.
func (s *Server) legionAnnounce(p *player, text string) {
	l := p.legion
	if !legionNoticePattern.MatchString(text) || !l.hasRights(p.member, rightAnnouncement) {
		return
	}
	now := time.Now()
	if err := s.legionDB.InsertAnnouncement(l.ID, text, now); err != nil {
		s.log.Error("saving a legion notice", "err", err)
		return
	}
	l.announcements = append(l.announcements, store.Announcement{Text: text, At: now})
	p.conn.send(systemMessage(msgLegionNoticeDone))
	w := wire.Packet(smLegionEdit)
	w.C(0x05)
	w.S(text)
	w.D(int32(now.Unix()))
	s.tellLegion(l, w)
}

// legionIntro is LegionService.changeSelfIntro.
func (s *Server) legionIntro(p *player, text string) {
	if !legionIntroPattern.MatchString(text) {
		return
	}
	p.member.SelfIntro = text
	w := wire.Packet(smLegionUpdateSelfIntro)
	w.D(p.ID)
	w.S(text)
	s.tellLegion(p.legion, w)
	p.conn.send(systemMessage(msgLegionIntroDone))
	s.saveMember(p)
}

// legionNickname is LegionService.changeNickname: a member's nickname, changed by a member with the right.
func (s *Server) legionNickname(p *player, name, nickname string) {
	l := p.legion
	target := s.playerNamed(convertName(name))
	if target == nil || target.legion != l || !legionNickPattern.MatchString(nickname) {
		return
	}
	target.member.Nickname = nickname
	w := wire.Packet(smLegionUpdateNickname)
	w.D(target.ID)
	w.S(nickname)
	s.tellLegion(l, w)
	s.saveMember(target)
}

// legionPermissions is LegionService.changePermissions.
func (s *Server) legionPermissions(l *legion, legionar2, centurion1, centurion2 int32) {
	if legionar2 < 0 || legionar2 > 0x08 || centurion1 < 0x60 || centurion1 > 0x7C || centurion2 < 0 || centurion2 > 0x0E {
		return
	}
	l.LegionarPerm2, l.CenturionPerm1, l.CenturionPerm2 = legionar2, centurion1, centurion2
	w := legionEdit(0x02)
	w.C(byte(l.CenturionPerm1))
	w.C(byte(l.CenturionPerm2))
	w.C(0x40)
	w.C(byte(l.LegionarPerm2))
	s.tellLegion(l, w)
	s.saveLegion(l)
}

// disbandLegion is LegionService.disbandLegion: the legion and its members' places in it are gone.
func (s *Server) disbandLegion(l *legion) {
	for _, m := range l.members {
		if p := s.spawned[m.PlayerID]; p != nil {
			p.broadcast(legionTitle(p, 0, "", 0), true)
			p.conn.send(legionLeave(1300302, 0, l.Name, ""))
			p.leftLegion()
		}
	}
	delete(s.legions, l.ID)
	if err := s.legionDB.DeleteLegion(l.ID); err != nil {
		s.log.Error("deleting a legion", "err", err)
	}
}

// sayToLegion is legion chat: every member who is in the world hears it.
func (s *Server) sayToLegion(p *player, text string) {
	if p.legion == nil {
		return
	}
	for _, m := range s.online(p.legion) {
		m.conn.send(s.chatMessage(p, m.conn, text, chatLegion))
	}
}

// legionLevelUp is LegionService.requestChangeLevel: the legion rises a level for kinah, when it has the members and points.
func (s *Server) legionLevelUp(p *player) {
	l := p.legion
	i := int(l.Level) - 1
	switch {
	case l.Level >= 5:
		p.conn.send(systemMessage(msgLegionMaxLevel))
		return
	case p.kinah.Count < legionLevelKinah[i]:
		p.conn.send(systemMessage(msgLegionLevelKinah))
		return
	case len(l.members) < legionLevelMembers[i]:
		p.conn.send(systemMessage(msgLegionLevelMembers))
		return
	case l.Contribution < legionLevelPoints[i]:
		p.conn.send(systemMessage(msgLegionLevelPoints))
		return
	}
	s.decreaseKinah(p, legionLevelKinah[i])
	l.Level++
	s.addHistory(l, itoa(l.Level), "LEVEL_UP")
	w := legionEdit(0x00)
	w.C(byte(l.Level))
	s.tellLegion(l, w)
	s.tellLegion(l, systemMessage(msgLegionLevelUp, int32(l.Level)))
	s.saveLegion(l)
}

// disbandDialog is NpcController's dialog 6, and recreateDialog 7: the brigade general asks to disband the legion, which
// happens in a day, or to keep it.
func (s *Server) disbandDialog(p *player) {
	l := p.legion
	if l == nil {
		return
	}
	if p.member.rank != rankBrigadeGeneral {
		p.conn.send(systemMessage(msgLegionOnlyMaster))
		return
	}
	if l.DisbandTime != 0 {
		p.conn.send(systemMessage(msgLegionDisbandAsked))
		return
	}
	if p.putRequest(questionLegionDisband, func(accepted bool) {
		if !accepted {
			return
		}
		l.DisbandTime = int32(time.Now().Unix()) + legionDisbandTime
		s.saveLegion(l)
		for _, m := range s.online(l) {
			m.conn.send(s.legionUpdateMember(m, msgLegionDisbanding, itoa(l.DisbandTime)))
			s.tellLegion(l, legionEditTime(0x06, l.DisbandTime))
		}
	}) {
		p.conn.send(questionWindow(questionLegionDisband, 0))
	}
}

func (s *Server) recreateDialog(p *player) {
	l := p.legion
	if l == nil || l.DisbandTime == 0 {
		return
	}
	if p.member.rank != rankBrigadeGeneral {
		p.conn.send(systemMessage(msgLegionOnlyMaster))
		return
	}
	if p.putRequest(questionLegionRecreate, func(accepted bool) {
		if !accepted {
			return
		}
		l.DisbandTime = 0
		s.saveLegion(l)
		s.tellLegion(l, legionEdit(0x07))
		for _, m := range s.online(l) {
			m.conn.send(s.legionUpdateMember(m, msgLegionRecreated, ""))
		}
	}) {
		p.conn.send(questionWindow(questionLegionRecreate, 0))
	}
}

const (
	legionEmblemKinah     = 10000 // legion.emblemrequiredkinah
	maxLegionEmblem       = 40
	msgLegionChangedEmble = 1390137
)

// legionEmblem is SM_LEGION_UPDATE_EMBLEM.
func legionEmblem(l *legion) *wire.Writer {
	w := wire.Packet(smLegionUpdateEmblem)
	w.D(l.ID)
	w.H(uint16(l.EmblemID))
	w.C(0xFF)
	w.C(byte(l.EmblemR))
	w.C(byte(l.EmblemG))
	w.C(byte(l.EmblemB))
	return w
}

// legionModifyEmblem is CM_LEGION_MODIFY_EMBLEM and LegionService.storeLegionEmblem: a legion of level 2 buys an emblem.
func (c *conn) legionModifyEmblem(r *wire.Reader) {
	id, emblem := r.D(), int32(r.H())
	r.C()
	red, green, blue := r.C(), r.C(), r.C()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		l := p.legion
		if l == nil || l.ID != id || emblem > maxLegionEmblem || l.Level < 2 {
			return
		}
		if p.kinah.Count < legionEmblemKinah {
			p.conn.send(systemMessage(msgNotEnoughKinah, strconv.Itoa(legionEmblemKinah)))
			return
		}
		if l.EmblemID == 0 && l.EmblemR|l.EmblemG|l.EmblemB == 0 {
			s.addHistory(l, "", "EMBLEM_REGISTER")
		} else {
			s.addHistory(l, "", "EMBLEM_MODIFIED")
		}
		s.decreaseKinah(p, legionEmblemKinah)
		l.EmblemID, l.EmblemR, l.EmblemG, l.EmblemB = emblem, int32(red), int32(green), int32(blue)
		s.saveLegion(l)
		s.tellLegion(l, legionEmblem(l))
		p.conn.send(systemMessage(msgLegionChangedEmble))
	})
}

// legionSendEmblem is CM_LEGION_SEND_EMBLEM: the client asks for a legion's emblem and name.
func (c *conn) legionSendEmblem(r *wire.Reader) {
	id := r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		l, err := s.legionOf(id)
		if err != nil || l == nil {
			return
		}
		w := wire.Packet(smLegionSendEmblem)
		w.D(l.ID)
		w.H(uint16(l.EmblemID))
		w.D(0)
		w.C(0xFF)
		w.C(byte(l.EmblemR))
		w.C(byte(l.EmblemG))
		w.C(byte(l.EmblemB))
		w.S(l.Name)
		w.C(1)
		p.conn.send(w)
	})
}

// The history types the client knows by their ids; LEVEL_UP isn't in the database's enum, so it lasts until the legion unloads.
var historyIDs = map[string]byte{"CREATE": 0, "JOIN": 1, "KICK": 2, "LEVEL_UP": 3, "APPOINTED": 4, "EMBLEM_REGISTER": 5, "EMBLEM_MODIFIED": 6}

// addHistory is LegionService.addHistory: the entry is kept and the legion is sent its history.
func (s *Server) addHistory(l *legion, name, kind string) {
	h := store.HistoryEntry{Type: kind, Name: name, At: time.Now()}
	l.history = slices.Insert(l.history, 0, h)
	if kind != "LEVEL_UP" {
		if err := s.legionDB.InsertHistory(l.ID, h); err != nil {
			s.log.Error("saving legion history", "legion", l.ID, "err", err)
		}
	}
	s.tellLegion(l, legionTabs(l.history, 0))
}

// legionTabs is SM_LEGION_TABS: a page of eight entries of the history.
func legionTabs(history []store.HistoryEntry, page int) *wire.Writer {
	w := wire.Packet(smLegionTabs)
	from := min(page*8, len(history))
	entries := history[from:min(from+8, len(history))]
	w.D(0x12)
	w.D(int32(page))
	w.D(int32(len(entries)))
	for _, h := range entries {
		w.D(int32(h.At.Unix()))
		w.C(historyIDs[h.Type])
		w.C(0)
		w.S(h.Name)
		w.B(make([]byte, max(0, 134-(len([]rune(h.Name))*2+2))))
	}
	w.H(0)
	return w
}

// legionTabs is CM_LEGION_TABS: the client asks for a page of the history (tab 0); the rewards tab (1) is empty.
func (c *conn) legionTabs(r *wire.Reader) {
	page, tab := int(r.D()), r.C()
	if r.Err != nil || page > 3 {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		if l := p.legion; l != nil && tab == 0 && len(l.history) >= page*8 && len(l.history) > 0 {
			p.conn.send(legionTabs(l.history, page))
		}
	})
}
