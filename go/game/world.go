package game

import (
	"context"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

func init() {
	handlers[cmEnterWorld] = (*conn).enterWorld
	handlers[cmLevelReady] = (*conn).levelReady
	handlers[cmMove] = (*conn).move
	handlers[cmChatMessagePublic] = (*conn).chat
	handlers[cmUiSettings] = (*conn).saveUISettings
	handlers[cmCustomSettings] = (*conn).customSettings
	handlers[cmMacAddress2] = func(*conn, *wire.Reader) {}
}

// Delays AL-Game waits after entering the world before loading the mailbox and
// logging the player in to the chat server, and how long the login protection lasts.
const (
	mailDelay       = 5 * time.Second
	chatDelay       = 10 * time.Second
	protectionDelay = time.Minute
)

// enterWorld is CM_ENTER_WORLD: the chosen character's data, before the client loads its map.
func (c *conn) enterWorld(r *wire.Reader) {
	ch := c.findCharacter(r.D())
	if ch == nil {
		return
	}
	s := c.s
	p, err := s.loadPlayer(ch)
	if err != nil {
		s.log.Error("loading player", "character", ch.Name, "err", err)
		return
	}
	s.mu.Lock()
	if other := s.players[p.ID]; other != nil && other != c {
		s.mu.Unlock()
		return
	}
	s.players[p.ID] = c
	s.mu.Unlock()
	p.conn = c
	c.player = p
	c.state = inGame
	s.visMu.Lock()
	s.restoreEffects(p, p.savedEffects, time.Now())
	p.savedEffects = nil
	s.instanceLogin(p)
	s.kiskLogin(p, false)
	s.visMu.Unlock()
	if err := s.store.SetOnline(p.ID, true); err != nil {
		s.log.Error("marking player online", "character", p.Name, "err", err)
	}

	s.grantStarterSkills(p)
	c.send(skillList(p))
	if p.cooldowns != nil {
		c.send(skillCooldowns(p.cooldowns, time.Now()))
	}
	if len(p.itemCooldowns) > 0 {
		c.send(itemCooldowns(p.itemCooldowns, time.Now()))
	}
	c.send(questList(p))
	c.send(recipeList(p))
	c.send(enterWorldCheck())
	if p.settings.UI != nil {
		c.send(uiSettings(0, p.settings.UI))
	}
	if p.settings.Shortcuts != nil {
		c.send(uiSettings(1, p.settings.Shortcuts))
	}
	if len(p.equipment) > 0 {
		c.send(s.inventoryInfo(p, p.equipment))
	}
	carried := append([]*store.Item{p.kinah}, p.cube...)
	for chunk := range slices.Chunk(carried, 10) {
		c.send(s.inventoryInfo(p, chunk))
	}
	c.send(s.inventoryInfo(p, nil))
	s.log.Info("player entered the world", "account", c.account.name, "character", p.Name)
	c.send(s.statsInfo(p))
	c.send(cubeUpdate(p))
	c.send(s.bindPoint(p))
	c.send(playerID(p))
	c.send(macroList(p))
	c.send(s.gameTimePacket())
	c.send(titleList(p))
	c.send(s.channelInfo(p))
	c.send(playerSpawn(p))
	c.send(emotionList())
	c.send(s.influenceRatio())
	c.send(s.siegeLocations())
	c.send(prices())
	c.send(abyssRank(p.abyss))
	c.send(message(chatAnnouncement, "Welcome to "+s.currentConfig().Name+", on Aion Lightning ported to Go."))
	s.visMu.Lock()
	s.prisonLogin(p)
	s.petitionLogin(p)
	s.classChangeDialog(p)
	if s.currentConfig().HTMLWelcome {
		s.showHTML(p, s.data.Welcome)
	}
	s.visMu.Unlock()
	c.send(statUpdate(smStatupdateMp, p.life.MP, p.stats.current(data.MaxMP)))
	c.send(s.nearbyQuests(p))
	c.send(statUpdate(smFlyTime, p.life.FP, p.stats.current(data.FlyTime)))

	c.after(mailDelay, p, func() {
		s.visMu.Lock()
		defer s.visMu.Unlock()
		if err := s.loadMailbox(p); err != nil {
			s.log.Error("loading the mailbox", "character", p.Name, "err", err)
		}
		c.send(s.mailLetters(p))
		if p.haveUnread() {
			c.send(mailState(true, true))
		}
	})
	c.after(chatDelay, p, func() { s.chat.playerLogin(p.ID, c.account.name) })
}

// levelReady is CM_LEVEL_READY: the map is loaded, so the player appears in it.
func (c *conn) levelReady(*wire.Reader) {
	p := c.player
	if p == nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	c.send(s.playerInfo(p, false))
	s.startProtection(p)
	s.spawnLocked(p)
	s.setFriendStatus(p, friendStatusOnline)
	s.legionLogin(p)
	s.brokerLogin(p)
	if p.kisk != nil {
		c.send(kiskPacket(p.kisk))
	}
	s.riftLogin(p)
	c.send(weather(s.weather(p.WorldID)))
	s.refreshZone(p)
	c.requestOfTheElimEnterWorld()
	c.ascensionEnterWorld()
	c.startPrologue()
	c.sealingAbyssGateEnterWorld()
	c.creatingMonsterEnterWorld()
	c.javaEnterWorld()
	c.send(systemMessage(msgChannelEntered, 1))
	// The 1.9 client takes current HP only once its level is loaded.
	c.send(statUpdate(smStatupdateHp, p.life.HP, p.stats.current(data.MaxHP)))
	c.send(statUpdate(smStatupdateMp, p.life.MP, p.stats.current(data.MaxMP)))
	c.send(s.abnormalStatePacket(p))
	c.send(s.nearbyQuests(p))
	if p.life.HP != p.stats.current(data.MaxHP) || p.life.MP != p.stats.current(data.MaxMP) {
		s.triggerRestore(p)
	}
}

// startProtection is PlayerController.startProtectionActiveTask: after entering or changing map the
// player blinks, and can't be hurt, until it acts or a minute passes.
func (s *Server) startProtection(p *player) {
	p.visualState = visualBlinking
	p.broadcast(playerState(p), true)
	p.protection.cancel()
	p.protection = s.later(protectionDelay, func() { s.stopProtection(p) })
}

// stopProtection is PlayerController.stopProtectionActiveTask.
func (s *Server) stopProtection(p *player) {
	p.protection.cancel()
	p.protection = nil
	if p.spawned && p.visualState == visualBlinking {
		p.visualState = 0
		p.broadcast(playerState(p), true)
	}
}

// endProtection is stopProtection for a client's packet.
func (c *conn) endProtection() {
	if c.player == nil {
		return
	}
	c.s.visMu.Lock()
	defer c.s.visMu.Unlock()
	c.s.stopProtection(c.player)
}

// endProtectionLocked is endProtection for a caller that holds the world lock.
func (c *conn) endProtectionLocked() {
	if c.player != nil {
		c.s.stopProtection(c.player)
	}
}

// Movement types, CM_MOVE's MovementType.
const (
	moveStartMouse       = 0xe0
	moveStartKeyboard    = 0xc0
	moveValidateMouse    = 0xa0
	moveValidateKeyboard = 0x80
	moveGlideUp          = 0x84
	moveGlideDown        = 0xc4
	moveGlideStartMouse  = 0xe4
	moveValidateGlide    = 0xa4
	moveStop             = 0x00
	moveMovingElevator   = 0xf0
	moveOnElevator       = 0x10
	moveStayingElevator  = 0x18
)

// move is CM_MOVE: where the player is now, and where it is heading. Those who
// see the player are told, and the players it comes near or leaves behind
// start or stop seeing it.
// ponytail: fall damage is not ported (PORTING.md 11).
func (c *conn) move(r *wire.Reader) {
	p := c.player
	if p == nil {
		return
	}
	x, y, z := r.F(), r.F(), r.F()
	heading := r.C()
	kind := r.C()
	var x2, y2, z2 float32
	var glide byte
	switch kind {
	case moveStartMouse, moveStartKeyboard:
		x2, y2, z2 = r.F(), r.F(), r.F()
	case moveGlideDown, moveGlideStartMouse:
		x2, y2, z2 = r.F(), r.F(), r.F()
		glide = r.C()
	case moveGlideUp, moveValidateGlide:
		glide = r.C()
	}
	if r.Err != nil || !finitePosition(x, y, z) || !finitePosition(x2, y2, z2) {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if !p.spawned {
		return
	}
	at := func() { s.updatePosition(p, x, y, z, heading) }
	defer func() {
		switch kind {
		case moveGlideStartMouse, moveGlideDown, moveGlideUp, moveValidateGlide:
			s.switchToGliding(p)
		case moveStartMouse, moveStartKeyboard, moveMovingElevator, moveOnElevator, moveStayingElevator,
			moveValidateMouse, moveValidateKeyboard, moveStop:
			s.stopGliding(p)
		}
	}()
	switch kind {
	case moveStartMouse, moveStartKeyboard, moveMovingElevator, moveOnElevator, moveStayingElevator:
		if in := p.interaction; in != nil && in.abortOnMove && in.inProgress() {
			in.abort()
		}
		at()
		p.target = [3]float32{x2, y2, z2}
		p.moves++
		p.broadcast(movePacket(p.ID, x, y, z, heading, kind, &[3]float32{x2, y2, z2}, nil), false)
	case moveGlideStartMouse, moveGlideDown:
		if kind == moveGlideStartMouse {
			p.target = [3]float32{x2, y2, z2}
		}
		at()
		p.broadcast(movePacket(p.ID, x, y, z, heading, kind, &[3]float32{x2, y2, z2}, &glide), false)
	case moveGlideUp:
		at()
		p.broadcast(movePacket(p.ID, x, y, z, heading, kind, nil, &glide), false)
	case moveValidateGlide:
		at()
		speed := float64(p.stats.current(data.Speed))
		angle := float64(heading) * 3 * math.Pi / 180
		dir := [3]float32{float32(speed * math.Cos(angle)), float32(speed * math.Sin(angle)), 0}
		p.broadcast(movePacket(p.ID, x, y, z, heading, moveGlideDown, &dir, &glide), false)
	case moveValidateMouse, moveValidateKeyboard:
		at()
		start := byte(moveStartKeyboard)
		if kind == moveValidateMouse {
			start = moveStartMouse
		}
		p.broadcast(movePacket(p.ID, x, y, z, heading, start, &p.target, nil), false)
	case moveStop:
		p.broadcast(movePacket(p.ID, x, y, z, heading, kind, nil, nil), false)
		at()
	}
	if kind != moveStop {
		c.endProtectionLocked()
	}
}

func finitePosition(x, y, z float32) bool {
	for _, v := range [...]float32{x, y, z} {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return false
		}
	}
	return true
}

// updatePosition is World.updatePosition: the player is there now, and sees and is seen from there.
// The caller holds visMu.
func (s *Server) updatePosition(p *player, x, y, z float32, heading byte) {
	p.X, p.Y, p.Z, p.Heading = x, y, z, int32(heading)
	if cellAt(p.WorldID, p.instance, x, y) != p.cell {
		s.removePlayerCell(p)
		s.addPlayerCell(p)
	}
	s.updateKnown(p)
	s.updateZone(p)
}

// movePacket is SM_MOVE: a player moving, with where it is heading if it is
// setting out, and the glide flag if it is gliding.
func movePacket(id int32, x, y, z float32, heading, kind byte, to *[3]float32, glide *byte) *wire.Writer {
	w := wire.Packet(smMove)
	w.D(id)
	w.F(x)
	w.F(y)
	w.F(z)
	w.C(heading)
	w.C(kind)
	if to != nil {
		w.F(to[0])
		w.F(to[1])
		w.F(to[2])
	}
	if glide != nil {
		w.C(*glide)
	}
	return w
}

// Chat types, ChatType.
const (
	chatNormal = 0x00
	chatShout  = 0x03
)

// chat is CM_CHAT_MESSAGE_PUBLIC: what the player says is heard by those who see it.
// ponytail: group, alliance and legion chat wait for those (PORTING.md 10), and
// so do the chat handlers (admin commands, PORTING.md 11).
func (c *conn) chat(r *wire.Reader) {
	p := c.player
	kind, text := r.C(), r.S()
	if p == nil || r.Err != nil {
		return
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if strings.HasPrefix(text, "//") {
		s.adminCommand(p, text)
		return
	}
	if !s.canChat(p) {
		return
	}
	switch kind {
	case chatNormal, chatShout:
		p.conn.send(s.chatMessage(p, p.conn, text, kind))
		for _, other := range p.known {
			if !other.blocked(p.ID) {
				other.conn.send(s.chatMessage(p, other.conn, text, kind))
			}
		}
	case chatGroup, chatGroupLeader:
		s.sayToGroup(p, text, kind)
	case chatAlliance:
		s.sayToAlliance(p, text)
	case chatLegion:
		s.sayToLegion(p, text)
	}
}

// chatMessage is SM_MESSAGE for player p's words, as listener hears them: one
// of another race reads gibberish, unless either is a game master (gameserver
// factions.speaking.mode 0).
func (s *Server) chatMessage(p *player, listener *conn, text string, kind byte) *wire.Writer {
	race, _ := raceGender(p.Character)
	var unreadable byte
	if c := p.conn.account; c.accessLevel == 0 && listener.account.accessLevel == 0 {
		unreadable = byte(race) + 1
	}
	w := wire.Packet(smMessage)
	w.C(kind)
	w.C(unreadable)
	w.D(p.ID)
	if kind == chatShout {
		w.S(p.Name)
		w.S(text)
		w.F(p.X)
		w.F(p.Y)
		w.F(p.Z)
	} else if kind == chatWhisper || kind == chatGroup || kind == chatGroupLeader || kind == chatLegion || kind == chatAlliance {
		w.S(p.Name)
		w.S(text)
	} else {
		w.H(0)
		w.S(text)
	}
	return w
}

// saveUISettings is CM_UI_SETTINGS: the client's layout or shortcuts, kept for logout.
func (c *conn) saveUISettings(r *wire.Reader) {
	p := c.player
	if p == nil {
		return
	}
	kind := r.C()
	r.H()
	r.H()
	blob := r.B(r.Remaining())
	switch kind {
	case 0:
		p.settings.UI = blob
	case 1:
		p.settings.Shortcuts = blob
	}
}

// leaveWorld is PlayerService.playerLoggedOut: the player leaves the world and is saved.
func (c *conn) leaveWorld() {
	p := c.player
	if p == nil {
		return
	}
	c.player = nil
	s := c.s
	s.visMu.Lock()
	p.LastOnline = time.Now()
	effects := savedEffects(p, p.LastOnline)
	cooldowns := savedItemCooldowns(p.itemCooldowns, p.LastOnline)
	p.fx.stopEffects()
	if p.summon != nil {
		s.releaseSummon(p.summon, unsummonLogout)
	}
	s.legionLogout(p)
	s.leaveGroup(p)
	s.leaveAlliance(p, allianceLeaving)
	s.loseDuel(p)
	if in := p.interaction; in != nil && in.inProgress() {
		in.stop()
	}
	p.itemUse.cancel()
	if e := s.exchangeOf(p); e != nil {
		partner := e.partner
		s.cleanupExchange(p, partner)
		partner.conn.send(exchangeConfirmation(1))
	}
	s.despawnLocked(p)
	s.setFriendStatus(p, friendStatusOffline)
	s.prisonLogout(p)
	p.restore.cancel()
	p.fpTask.cancel()
	p.protection.cancel()
	if p.dead {
		s.moveToBind(p, false, 0)
	}
	s.visMu.Unlock()
	s.mu.Lock()
	if s.players[p.ID] == c {
		delete(s.players, p.ID)
	}
	s.mu.Unlock()
	s.chat.playerLogout(p.ID)
	for _, err := range []error{
		s.store.SaveLifeStats(p.ID, p.life),
		s.store.SaveCharacter(p.Character),
		s.store.SetOnline(p.ID, false),
		s.saveSentence(p),
		s.saveSettings(p),
		s.store.SaveEffects(context.Background(), p.ID, effects),
		s.store.SaveItemCooldowns(context.Background(), p.ID, cooldowns),
		s.saveGameTime(),
	} {
		if err != nil {
			s.log.Error("saving player", "character", p.Name, "err", err)
		}
	}
	s.log.Info("player left the world", "character", p.Name)
}

// saveSettings is PlayerSettingsDAO.saveSettings: the layout and shortcuts the client sent,
// then the display and deny flags, which are kept as their text.
func (s *Server) saveSettings(p *player) error {
	if p.settings.UI != nil {
		if err := s.store.SaveSetting(p.ID, 0, p.settings.UI); err != nil {
			return err
		}
	}
	if p.settings.Shortcuts != nil {
		if err := s.store.SaveSetting(p.ID, 1, p.settings.Shortcuts); err != nil {
			return err
		}
	}
	if err := s.store.SaveSetting(p.ID, 2, strconv.AppendInt(nil, int64(p.settings.Display), 10)); err != nil {
		return err
	}
	return s.store.SaveSetting(p.ID, 3, strconv.AppendInt(nil, int64(p.settings.Deny), 10))
}

// customSettings is CM_CUSTOM_SETTINGS: the player's display flags (mantle, helmet …) and the requests it denies,
// shown to it and to those who see it.
func (c *conn) customSettings(r *wire.Reader) {
	display, deny := r.H(), r.H()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		p.settings.Display, p.settings.Deny = int32(display), int32(deny)
		p.broadcast(customSettingsPacket(p), true)
	})
}

// customSettingsPacket is SM_CUSTOM_SETTINGS.
func customSettingsPacket(p *player) *wire.Writer {
	w := wire.Packet(smCustomSettings)
	w.D(p.ID)
	w.C(1)
	w.H(uint16(p.settings.Display))
	w.H(uint16(p.settings.Deny))
	return w
}

// after runs fn in d, if the connection still has p in the world then.
func (c *conn) after(d time.Duration, p *player, fn func()) {
	time.AfterFunc(d, func() {
		c.worldMu.Lock()
		defer c.worldMu.Unlock()
		if c.player == p {
			fn()
		}
	})
}

// gameTime is Atreia's clock, in game minutes: one every five seconds.
func (s *Server) gameTime() int32 {
	return s.clockBase + int32(time.Since(s.clockStart)/(5*time.Second))
}

func (s *Server) saveGameTime() error {
	return s.store.SaveGameTime(s.gameTime())
}

// weatherDuration is how long a map keeps its weather.
const weatherDuration = 2 * time.Hour

// weather is WeatherService's: each map has one of nine weathers, drawn at random every two hours.
func (s *Server) weather(worldID int32) byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, ok := s.weathers[worldID]
	if !ok || time.Since(w.since) > weatherDuration {
		w = weatherState{code: byte(rand.IntN(9)), since: time.Now()}
		s.weathers[worldID] = w
	}
	return w.code
}

type weatherState struct {
	code  byte
	since time.Time
}
