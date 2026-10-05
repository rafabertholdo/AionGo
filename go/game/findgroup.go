package game

import (
	"cmp"
	"slices"
	"time"

	"aionlightning/wire"
)

// Find Group (STR_WINDOW_PARTY_MATCH) is FindGroupService from AL-Game 2.7: the 1.9 Java server has no
// such service. CM_FIND_GROUP and SM_FIND_GROUP keep 2.7's layouts; the 1.9 opcodes and the actions the
// 1.9 client accepts (0, 1, 4, 5) come from its SM_FIND_GROUP handler.

// The 1.9 opcodes are not in AL-Game's tables: CM_FIND_GROUP is from a client capture, and SM_FIND_GROUP
// is the opcode whose client handler parses 2.7's SM_FIND_GROUP (client table index 0xa6).
const (
	cmFindGroup = 0x40 // CM_FIND_GROUP
	smFindGroup = 0xc2 // SM_FIND_GROUP
	// The Find Group window asks about a listed player: 0xb0 is 2.7's CM_CHAT_WINDOW (S name, D 0), whose
	// reply opens the private chat window; 0x9a (S name) asks for the member info popup. Both are answered
	// with SM_CHAT_WINDOW, whose 1.9 handler (client index 0x63) parses the sub types below.
	cmChatWindow     = 0xb0 // CM_CHAT_WINDOW
	cmChatMemberInfo = 0x9a // CM_CHAT_WINDOW's member info request
	smChatWindow     = 0x61 // SM_CHAT_WINDOW
)

// SM_CHAT_WINDOW sub types.
const (
	chatWindowMemberInfo byte = 1 // S name, S legion, C level, C class, C, S
	chatWindowGroup      byte = 2 // S name, D group id, S leader, levels and classes of six members
	chatWindowSolo       byte = 4 // S name, D 0, C class, C level, C
)

func init() {
	handlers[cmFindGroup] = (*conn).findGroup
	handlers[cmChatWindow] = (*conn).chatWindow
	handlers[cmChatMemberInfo] = (*conn).chatMemberInfo
	for op, name := range map[byte]string{cmFindGroup: "CM_FIND_GROUP", cmChatWindow: "CM_CHAT_WINDOW", cmChatMemberInfo: "CM_CHAT_MEMBER_INFO"} {
		clientPacketStates[op] = inGame
		clientNames[op] = name
	}
	serverNames[smFindGroup] = "SM_FIND_GROUP"
	serverNames[smChatWindow] = "SM_CHAT_WINDOW"
}

// chatWindow is CM_CHAT_WINDOW: the player opens a private window with a listed player, who is shown
// alone or with their group (SM_CHAT_WINDOW from AL-Game 2.7).
func (c *conn) chatWindow(r *wire.Reader) {
	name := convertName(r.S())
	r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		if target := s.playerNamed(name); target != nil {
			p.conn.send(chatWindowPacket(target))
		}
	})
}

// chatMemberInfo answers the Find Group member info request with the player's legion, level and class.
func (c *conn) chatMemberInfo(r *wire.Reader) {
	name := convertName(r.S())
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		target := s.playerNamed(name)
		if target == nil {
			return
		}
		w := wire.Packet(smChatWindow)
		w.C(chatWindowMemberInfo)
		w.S(target.Name)
		legion := ""
		if target.legion != nil {
			legion = target.legion.Name
		}
		w.S(legion)
		w.C(byte(target.level))
		w.C(byte(classIDs[target.Class]))
		w.C(0)
		w.S("")
		p.conn.send(w)
	})
}

func chatWindowPacket(target *player) *wire.Writer {
	w := wire.Packet(smChatWindow)
	g := target.group
	if g == nil || len(g.members) < 2 {
		w.C(chatWindowSolo)
		w.S(target.Name)
		w.D(0)
		w.C(byte(classIDs[target.Class]))
		w.C(byte(target.level))
		w.C(0)
		return w
	}
	w.C(chatWindowGroup)
	w.S(target.Name)
	w.D(g.id)
	w.S(g.leader.Name)
	for i := range maxGroupSize {
		if i < len(g.members) {
			w.C(byte(g.members[i].level))
		} else {
			w.C(0)
		}
	}
	for i := range maxGroupSize {
		if i < len(g.members) {
			w.C(byte(classIDs[g.members[i].Class]))
		} else {
			w.C(0)
		}
	}
	return w
}

const (
	findGroupRecruit byte = 0 // the recruit list; its post action is 2, its delete 1
	findGroupApply   byte = 4 // the apply list; its post action is 6, its delete 5

	msgFindGroupRecruitPosted = 1400392
	msgFindGroupApplyPosted   = 1400393

	findGroupPlayerUnk = 65557 // FindGroup.getUnk for a player; teams send 0
	findGroupMaxAge    = time.Hour
	findGroupPerPacket = 200 // entries per SM_FIND_GROUP; each is under 200 bytes
)

type findGroupKey struct {
	race int32
	list byte
	id   int32
}

// findGroupPost is FindGroup: a player's post, or that of their group or alliance.
type findGroupPost struct {
	player    *player
	group     *group
	alliance  *alliance
	message   string
	groupType byte
	updated   time.Time
}

func (f *findGroupPost) id() int32 {
	switch {
	case f.alliance != nil:
		return f.alliance.id
	case f.group != nil:
		return f.group.id
	}
	return f.player.ID
}

func (f *findGroupPost) unk() int32 {
	if f.group == nil && f.alliance == nil {
		return findGroupPlayerUnk
	}
	return 0
}

func (f *findGroupPost) members() []*player {
	switch {
	case f.alliance != nil:
		return f.alliance.members
	case f.group != nil:
		return f.group.members
	}
	return []*player{f.player}
}

func (f *findGroupPost) name() string {
	switch {
	case f.alliance != nil:
		return f.alliance.captain.Name
	case f.group != nil:
		return f.group.leader.Name
	}
	return f.player.Name
}

func (f *findGroupPost) levels() (lowest, highest int) {
	members := f.members()
	lowest, highest = members[0].level, members[0].level
	for _, m := range members {
		lowest, highest = min(lowest, m.level), max(highest, m.level)
	}
	return lowest, highest
}

// findGroup is CM_FIND_GROUP.
func (c *conn) findGroup(r *wire.Reader) {
	action := r.C()
	var id, unk int32
	var message string
	var groupType byte
	switch action {
	case 1:
		id, unk = r.D(), r.D()
	case 2:
		r.D()
		message, groupType = r.S(), r.C()
	case 3:
		id, unk = r.D(), r.D()
		message, groupType = r.S(), r.C()
	case 5:
		id = r.D()
	case 6:
		r.D()
		message, groupType = r.S(), r.C()
		r.C() // class and level: the list shows the player's own
		r.C()
	}
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		race, _ := raceGender(p.Character)
		s.log.Debug("find group", "player", p.Name, "action", action, "race", race, "posts", len(s.findGroups))
		switch action {
		case findGroupRecruit, findGroupApply:
			s.sendFindGroups(p, action)
		case 1, 5:
			list := action - 1
			if post := s.findGroups[findGroupKey{race, list, id}]; post != nil && s.ownsFindGroup(p, post) && (list == findGroupApply || post.unk() == unk) {
				s.removeFindGroup(race, list, id)
			}
		case 2, 6:
			s.postFindGroup(p, action-2, message, groupType)
		case 3:
			if post := s.findGroups[findGroupKey{race, findGroupRecruit, id}]; post != nil && s.ownsFindGroup(p, post) && post.unk() == unk {
				post.message, post.groupType, post.updated = clampRunes(message, findGroupMessageLength), groupType, time.Now()
				s.sendFindGroups(p, findGroupRecruit)
			}
		}
	})
}

// findGroupMessageLength is the most characters the client keeps of a post's message.
const findGroupMessageLength = 50

func clampRunes(text string, n int) string {
	if r := []rune(text); len(r) > n {
		return string(r[:n])
	}
	return text
}

// ownsFindGroup reports whether the player may change the post: their own, or their team's as its leader.
func (s *Server) ownsFindGroup(p *player, post *findGroupPost) bool {
	switch {
	case post.alliance != nil:
		return post.alliance == p.alliance && post.alliance.captain == p
	case post.group != nil:
		return post.group == p.group && post.group.leader == p
	}
	return post.player == p
}

// postFindGroup is FindGroupService.addFindGroupList: a team member posts for the team.
// The 1.9 client has no reply for actions 2 and 6, so the list is sent again instead.
func (s *Server) postFindGroup(p *player, list byte, message string, groupType byte) {
	race, _ := raceGender(p.Character)
	post := &findGroupPost{player: p, alliance: p.alliance, message: clampRunes(message, findGroupMessageLength), groupType: groupType, updated: time.Now()}
	if post.alliance == nil {
		post.group = p.group
	}
	if s.findGroups == nil {
		s.findGroups = map[findGroupKey]*findGroupPost{}
	}
	s.findGroups[findGroupKey{race, list, post.id()}] = post
	if list == findGroupRecruit {
		p.conn.send(systemMessage(msgFindGroupRecruitPosted))
	} else {
		p.conn.send(systemMessage(msgFindGroupApplyPosted))
	}
	s.sendFindGroups(p, list)
}

// sendFindGroups is FindGroupService.sendFindGroups. A new list id makes the client clear its list;
// the client shows it once the total has arrived, so a long list goes in several packets.
func (s *Server) sendFindGroups(p *player, list byte) {
	race, _ := raceGender(p.Character)
	s.expireFindGroups()
	var posts []*findGroupPost
	for _, key := range sortedFindGroupKeys(s.findGroups) {
		if key.race == race && key.list == list {
			posts = append(posts, s.findGroups[key])
		}
	}
	s.findGroupListID = max(s.findGroupListID+1, int32(time.Now().Unix()))
	for start := 0; start == 0 || start < len(posts); start += findGroupPerPacket {
		page := posts[start:min(start+findGroupPerPacket, len(posts))]
		w := wire.Packet(smFindGroup)
		w.C(list)
		w.H(uint16(len(posts)))
		w.H(uint16(len(page)))
		w.D(s.findGroupListID)
		for _, post := range page {
			lowest, highest := post.levels()
			w.D(post.id())
			if list == findGroupRecruit {
				w.D(post.unk())
				w.C(post.groupType)
				w.S(post.message)
				w.S(post.name())
				w.C(byte(len(post.members())))
				w.C(byte(lowest))
				w.C(byte(highest))
			} else {
				w.C(post.groupType)
				w.S(post.message)
				w.S(post.name())
				w.C(byte(classIDs[post.player.Class]))
				w.C(byte(post.player.level))
			}
			w.D(int32(post.updated.Unix()))
		}
		p.conn.send(w)
	}
}

func sortedFindGroupKeys(m map[findGroupKey]*findGroupPost) []findGroupKey {
	keys := make([]findGroupKey, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b findGroupKey) int {
		return cmp.Or(cmp.Compare(a.race, b.race), cmp.Compare(a.list, b.list), cmp.Compare(a.id, b.id))
	})
	return keys
}

// removeFindGroup is FindGroupService.removeFindGroup: the race's players drop the post from their lists.
func (s *Server) removeFindGroup(race int32, list byte, id int32) *findGroupPost {
	key := findGroupKey{race, list, id}
	post := s.findGroups[key]
	if post == nil {
		return nil
	}
	delete(s.findGroups, key)
	w := wire.Packet(smFindGroup)
	w.C(list + 1)
	w.D(id)
	if list == findGroupRecruit {
		w.D(post.unk())
	}
	for _, o := range s.spawned {
		if r, _ := raceGender(o.Character); r == race {
			o.conn.send(w)
		}
	}
	return post
}

// expireFindGroups is FindGroupService.clean: posts not updated within the hour go.
func (s *Server) expireFindGroups() {
	for _, key := range sortedFindGroupKeys(s.findGroups) {
		if time.Since(s.findGroups[key].updated) > findGroupMaxAge {
			s.removeFindGroup(key.race, key.list, key.id)
		}
	}
}

// removePlayerFindGroups drops the player's own posts: on joining a team (as in 2.7) and on logout.
func (s *Server) removePlayerFindGroups(p *player) {
	race, _ := raceGender(p.Character)
	s.removeFindGroup(race, findGroupRecruit, p.ID)
	s.removeFindGroup(race, findGroupApply, p.ID)
}

// takeFindGroup and repostFindGroup are 2.7's group and alliance create callbacks: the founder's own
// post, recruiting or applying, becomes the new team's recruit post.
func (s *Server) takeFindGroup(p *player) *findGroupPost {
	race, _ := raceGender(p.Character)
	if post := s.removeFindGroup(race, findGroupRecruit, p.ID); post != nil {
		return post
	}
	return s.removeFindGroup(race, findGroupApply, p.ID)
}

func (s *Server) repostFindGroup(p *player, post *findGroupPost) {
	if post != nil {
		s.postFindGroup(p, findGroupRecruit, post.message, post.groupType)
	}
}
