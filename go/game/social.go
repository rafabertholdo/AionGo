package game

import (
	"strings"

	"aionlightning/game/store"
	"aionlightning/wire"
)

// Friends, blocks and whispers: AL-Game's SocialService, FriendList and BlockList.

// socialSaver keeps friendships and blocks; store.Store does.
type socialSaver interface {
	AddFriends(a, b int32) error
	DelFriends(a, b int32) error
	AddBlock(player, blocked int32, reason string) error
	DelBlock(player, blocked int32) error
	SetBlockReason(player, blocked int32, reason string) error
}

func init() {
	handlers[cmChatMessageWhisper] = (*conn).whisper
	handlers[cmShowFriendlist] = func(c *conn, _ *wire.Reader) {
		c.withPlayer(func(s *Server, p *player) { p.conn.send(s.friendListPacket(p)) })
	}
	handlers[cmShowBlocklist] = func(c *conn, _ *wire.Reader) {
		c.withPlayer(func(s *Server, p *player) { p.conn.send(blockListPacket(p)) })
	}
	handlers[cmFriendStatus] = (*conn).friendStatus
	handlers[cmFriendAdd] = (*conn).friendAdd
	handlers[cmFriendDel] = (*conn).friendDel
	handlers[cmBlockAdd] = (*conn).blockAdd
	handlers[cmBlockDel] = (*conn).blockDel
	handlers[cmBlockSetReason] = (*conn).blockSetReason
	handlers[cmSetNote] = (*conn).setNote
}

// withPlayer runs fn with the world locked, for the connection's player if it is in the world.
func (c *conn) withPlayer(fn func(s *Server, p *player)) {
	p := c.player
	if p == nil {
		return
	}
	c.s.visMu.Lock()
	defer c.s.visMu.Unlock()
	fn(c.s, p)
}

const (
	maxFriends = 10
	maxBlocks  = 10
)

// System messages of the social lists.
const (
	msgBuddyBusy         = 900847
	msgPlayerOffline     = 1300627 // "%0 is not playing the game."
	msgBlockedBy         = 1300628 // "%0 has blocked you."
	msgBuddyListFull     = 1300887
	msgBuddyNotInList    = 1300889
	msgBlockNoBuddy      = 1300891
	msgBlockAlready      = 1300894
	msgBlockNotBlocked   = 1300897
	msgWhisperLevel      = 1310004 // "You must be level %0 to whisper."
	msgRejectedFriend    = 1390119
	questionBuddyRequest = 0x0DBEE9
)

// SM_FRIEND_RESPONSE codes.
const (
	friendAdded          = 0
	friendOffline        = 1
	friendAlready        = 2
	friendDenied         = 4
	friendListFull       = 5
	friendRemoved        = 6
	friendBlocked        = 8
	deniedFriend   int32 = 16 // DeniedStatus.FRIEND
)

// Friend statuses (FriendList.Status) and SM_FRIEND_NOTIFY codes.
const (
	friendStatusOffline = 0
	friendStatusOnline  = 1
	notifyLogin         = 0
	notifyLogout        = 1
	notifyDeleted       = 2
	chatWhisper         = 0x04
)

// playerNamed finds a player in the world by name, ignoring case.
func (s *Server) playerNamed(name string) *player {
	for _, p := range s.spawned {
		if strings.EqualFold(p.Name, name) {
			return p
		}
	}
	return nil
}

func (p *player) friend(id int32) *store.Character {
	for _, f := range p.friends {
		if f.ID == id {
			return f
		}
	}
	return nil
}

func (p *player) friendNamed(name string) *store.Character {
	for _, f := range p.friends {
		if strings.EqualFold(f.Name, name) {
			return f
		}
	}
	return nil
}

func (p *player) blocked(id int32) bool {
	for _, b := range p.blocks {
		if b.ID == id {
			return true
		}
	}
	return false
}

func (p *player) blockNamed(name string) *store.Block {
	for i := range p.blocks {
		if strings.EqualFold(p.blocks[i].Name, name) {
			return &p.blocks[i]
		}
	}
	return nil
}

// friendEntry writes what SM_FRIEND_LIST and SM_FRIEND_UPDATE tell about a friend, the same but for the online byte.
func (s *Server) friendEntry(w *wire.Writer, f *store.Character, listed bool) {
	live := s.spawned[f.ID]
	level, world, status, last := int32(s.data.Level(f.Exp)), f.WorldID, byte(friendStatusOffline), int32(0)
	note, class := f.Note, f.Class
	if live != nil {
		level, world, status, note, class = int32(live.level), live.WorldID, live.friendStatus, live.Note, live.Class
	} else if !f.LastOnline.IsZero() {
		last = int32(f.LastOnline.Unix())
	}
	w.S(f.Name)
	w.D(level)
	w.D(classIDs[class])
	if listed {
		w.C(1)
	} else {
		w.Bool(live != nil)
	}
	w.D(world)
	w.D(last)
	w.S(note)
	w.C(status)
}

// friendListPacket is SM_FRIEND_LIST.
func (s *Server) friendListPacket(p *player) *wire.Writer {
	w := wire.Packet(smFriendList)
	w.H(uint16(-len(p.friends)))
	w.C(0)
	for _, f := range p.friends {
		s.friendEntry(w, f, true)
	}
	return w
}

// blockListPacket is SM_BLOCK_LIST.
func blockListPacket(p *player) *wire.Writer {
	w := wire.Packet(smBlockList)
	w.H(uint16(len(p.blocks)))
	w.C(0)
	for _, b := range p.blocks {
		w.S(b.Name)
		w.S(b.Reason)
	}
	return w
}

func friendResponse(name string, code byte) *wire.Writer {
	w := wire.Packet(smFriendResponse)
	w.S(name)
	w.C(code)
	return w
}

func blockResponse(code int32, name string) *wire.Writer {
	w := wire.Packet(smBlockResponse)
	w.S(name)
	w.D(code)
	return w
}

func friendNotify(code byte, name string) *wire.Writer {
	w := wire.Packet(smFriendNotify)
	w.S(name)
	w.C(code)
	return w
}

// friendUpdate is SM_FRIEND_UPDATE: how a friend is now.
func (s *Server) friendUpdate(f *store.Character) *wire.Writer {
	w := wire.Packet(smFriendUpdate)
	s.friendEntry(w, f, false)
	return w
}

// setFriendStatus is FriendList.setStatus: the player's friends who are in the world are told how it is now.
func (s *Server) setFriendStatus(p *player, status byte) {
	previous := p.friendStatus
	p.friendStatus = status
	for _, f := range p.friends {
		other := s.spawned[f.ID]
		if other == nil {
			continue
		}
		mine := other.friend(p.ID)
		if mine == nil {
			continue
		}
		other.conn.send(s.friendUpdate(mine))
		switch {
		case previous == friendStatusOffline:
			other.conn.send(friendNotify(notifyLogin, p.Name))
		case status == friendStatusOffline:
			other.conn.send(friendNotify(notifyLogout, p.Name))
		}
	}
}

func (c *conn) friendStatus(r *wire.Reader) {
	status := r.C()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		if status > 2 {
			status = friendStatusOnline
		}
		s.setFriendStatus(p, status)
	})
}

// friendAdd is CM_FRIEND_ADD: the other player is asked, and the two are friends if it agrees.
func (c *conn) friendAdd(r *wire.Reader) {
	name := r.S()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		target := s.playerNamed(name)
		switch {
		case strings.EqualFold(name, p.Name):
		case target == nil:
			p.conn.send(friendResponse(name, friendOffline))
		case p.friend(target.ID) != nil:
			p.conn.send(friendResponse(target.Name, friendAlready))
		case len(p.friends) >= maxFriends:
			p.conn.send(systemMessage(msgBuddyListFull))
		case len(target.friends) >= maxFriends:
			p.conn.send(friendResponse(target.Name, friendListFull))
		case p.blocked(target.ID):
			p.conn.send(friendResponse(target.Name, friendBlocked))
		case target.blocked(p.ID):
			p.conn.send(systemMessage(msgBlockedBy, name))
		default:
			asked := target.putRequest(questionBuddyRequest, func(accepted bool) {
				if !accepted {
					p.conn.send(friendResponse(name, friendDenied))
					return
				}
				if s.spawned[target.ID] == nil || s.spawned[p.ID] == nil {
					p.conn.send(friendResponse(name, friendOffline))
					return
				}
				if len(p.friends) < maxFriends && len(target.friends) < maxFriends {
					s.makeFriends(p, target)
				}
			})
			if !asked {
				p.conn.send(systemMessage(msgBuddyBusy))
				return
			}
			if target.settings.Deny&deniedFriend != 0 {
				p.conn.send(systemMessage(msgRejectedFriend, target.Name))
				return
			}
			target.conn.send(questionWindow(questionBuddyRequest, p.ID, p.Name))
		}
	})
}

// makeFriends is SocialService.makeFriends.
func (s *Server) makeFriends(a, b *player) {
	if err := s.social.AddFriends(a.ID, b.ID); err != nil {
		s.log.Error("making friends", "err", err)
		return
	}
	a.friends = append(a.friends, b.Character)
	b.friends = append(b.friends, a.Character)
	for _, pair := range [][2]*player{{a, b}, {b, a}} {
		pair[0].conn.send(s.friendListPacket(pair[0]))
		pair[0].conn.send(friendResponse(pair[1].Name, friendAdded))
	}
}

// friendDel is CM_FRIEND_DEL.
func (c *conn) friendDel(r *wire.Reader) {
	name := r.S()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		f := p.friendNamed(name)
		if f == nil {
			p.conn.send(systemMessage(msgBuddyNotInList))
			return
		}
		if err := s.social.DelFriends(p.ID, f.ID); err != nil {
			s.log.Error("deleting friend", "err", err)
			return
		}
		p.friends = removeCharacter(p.friends, f.ID)
		p.conn.send(s.friendListPacket(p))
		p.conn.send(friendResponse(f.Name, friendRemoved))
		if other := s.spawned[f.ID]; other != nil {
			other.friends = removeCharacter(other.friends, p.ID)
			other.conn.send(friendNotify(notifyDeleted, p.Name))
			other.conn.send(s.friendListPacket(other))
		}
	})
}

func removeCharacter(list []*store.Character, id int32) []*store.Character {
	kept := list[:0:0]
	for _, f := range list {
		if f.ID != id {
			kept = append(kept, f)
		}
	}
	return kept
}

// SM_BLOCK_RESPONSE codes.
const (
	blockSuccess    = 0
	unblockSuccess  = 1
	blockNotFound   = 2
	blockListFull   = 3
	blockCantBeSelf = 4
)

// blockAdd is CM_BLOCK_ADD.
func (c *conn) blockAdd(r *wire.Reader) {
	name, reason := r.S(), r.S()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		target := s.playerNamed(name)
		switch {
		case strings.EqualFold(p.Name, name):
			p.conn.send(blockResponse(blockCantBeSelf, name))
		case len(p.blocks) >= maxBlocks:
			p.conn.send(blockResponse(blockListFull, name))
		case target == nil:
			p.conn.send(blockResponse(blockNotFound, name))
		case p.friend(target.ID) != nil:
			p.conn.send(systemMessage(msgBlockNoBuddy))
		case p.blocked(target.ID):
			p.conn.send(systemMessage(msgBlockAlready))
		default:
			if err := s.social.AddBlock(p.ID, target.ID, reason); err != nil {
				s.log.Error("blocking", "err", err)
				return
			}
			p.blocks = append(p.blocks, store.Block{ID: target.ID, Name: target.Name, Reason: reason})
			p.conn.send(blockResponse(blockSuccess, target.Name))
			p.conn.send(blockListPacket(p))
		}
	})
}

// blockDel is CM_BLOCK_DEL.
func (c *conn) blockDel(r *wire.Reader) {
	name := r.S()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		b := p.blockNamed(name)
		if b == nil {
			p.conn.send(systemMessage(msgBuddyNotInList))
			return
		}
		id, blockedName := b.ID, b.Name
		if err := s.social.DelBlock(p.ID, id); err != nil {
			s.log.Error("unblocking", "err", err)
			return
		}
		kept := p.blocks[:0:0]
		for _, other := range p.blocks {
			if other.ID != id {
				kept = append(kept, other)
			}
		}
		p.blocks = kept
		p.conn.send(blockResponse(unblockSuccess, blockedName))
		p.conn.send(blockListPacket(p))
	})
}

// blockSetReason is CM_BLOCK_SET_REASON.
func (c *conn) blockSetReason(r *wire.Reader) {
	name, reason := r.S(), r.S()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		b := p.blockNamed(name)
		if b == nil {
			p.conn.send(systemMessage(msgBlockNotBlocked))
			return
		}
		if b.Reason == reason {
			return
		}
		if err := s.social.SetBlockReason(p.ID, b.ID, reason); err != nil {
			s.log.Error("setting the reason of a block", "err", err)
			return
		}
		b.Reason = reason
		p.conn.send(blockListPacket(p))
	})
}

// setNote is CM_SET_NOTE: the friends who are in the world get the new list.
func (c *conn) setNote(r *wire.Reader) {
	note := r.S()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		if note == p.Note {
			return
		}
		p.Note = note
		for _, f := range p.friends {
			if other := s.spawned[f.ID]; other != nil {
				other.conn.send(s.friendListPacket(other))
			}
		}
	})
}

// whisper is CM_CHAT_MESSAGE_WHISPER.
func (c *conn) whisper(r *wire.Reader) {
	name, text := r.S(), r.S()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		name = convertName(name)
		receiver := s.playerNamed(name)
		switch {
		case receiver == nil:
			p.conn.send(systemMessage(msgPlayerOffline, name))
		case p.level < 10:
			p.conn.send(systemMessage(msgWhisperLevel, "10"))
		case receiver.blocked(p.ID):
			p.conn.send(systemMessage(msgBlockedBy, receiver.Name))
		case s.canChat(p):
			receiver.conn.send(s.chatMessage(p, receiver.conn, text, chatWhisper))
		}
	})
}

// loadSocial reads a player's friends and blocks.
func (s *Server) loadSocial(p *player) (err error) {
	if p.friends, err = s.store.Friends(p.ID); err != nil {
		return err
	}
	p.blocks, err = s.store.Blocks(p.ID)
	return err
}
