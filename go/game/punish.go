package game

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"aionlightning/wire"
)

// Punishments: AL-Game's PunishmentService and PrisonRestrictions, the gag of PlayerRestrictions.canChat, and the
// admin commands that use them (//gag //ungag //sprison //rprison //kick //ban //unban //banip //unbanip).
// Bans are the login server's: the command sends it CM_BAN (SM_BAN) and it kicks the account.

// punishSaver keeps prison sentences; store.Store does.
type punishSaver interface {
	PrisonTimer(playerID int32) (int64, error)
	PunishPlayer(playerID int32, status int, timerMS int64) error
	UnpunishPlayer(playerID int32) error
	SavePunishment(playerID int32, status int, timerMS int64) error
	AccountIDByName(string) (int32, error)
}

// The prison map and where teleportToPrison puts a player.
const (
	prisonMap                         = 510010000
	prisonX, prisonY, prisonZ float32 = 256, 256, 49
	banTypeAccount, banTypeIP         = 1, 2
	banTypeFull                       = 3
)

// punishment is what a player is under: a gag, and a prison sentence that runs while it is online.
type punishment struct {
	gagged      bool
	gagTask     *task
	prisonTimer time.Duration // what is left; a player is in prison while it isn't 0, as Player.isInPrison
	prisonStart time.Time
	prisonTask  *task
}

func (p *player) inPrison() bool { return p.punish.prisonTimer != 0 }

// prisoner is PrisonRestrictions.isInPrison.
func (p *player) prisoner() bool { return p.inPrison() || p.WorldID == prisonMap }

// restrictedInPrison tells a player in prison it can't do what, and says whether it is in prison.
func (s *Server) restrictedInPrison(p *player, what string) bool {
	if !p.prisoner() {
		return false
	}
	s.tell(p, "You cannot "+what+" in prison!")
	return true
}

// canChat is RestrictionsManager.canChat: neither in prison nor gagged.
func (s *Server) canChat(p *player) bool {
	return !s.restrictedInPrison(p, "chat") && !p.punish.gagged
}

// setIsInPrison is PunishmentService.setIsInPrison.
func (s *Server) setIsInPrison(p *player, state bool, minutes int64) {
	p.punish.prisonTask.cancel()
	if !state {
		s.tell(p, "You removed from prison!")
		p.punish.prisonTimer = 0
		s.moveToBind(p, true, 0)
		if err := s.punishDB.UnpunishPlayer(p.ID); err != nil {
			s.log.Error("lifting a punishment", "character", p.Name, "err", err)
		}
		return
	}
	if minutes > 0 {
		s.schedulePrison(p, time.Duration(minutes)*time.Minute)
		s.tell(p, fmt.Sprintf("You are in prison for %d minutes.\nIf you disconnect, the countdown will be stopped.", minutes))
	}
	p.punish.prisonStart = time.Now()
	s.teleportTo(p, prisonMap, prisonX, prisonY, prisonZ, 0, 0)
	if err := s.punishDB.PunishPlayer(p.ID, 1, p.punish.prisonTimer.Milliseconds()); err != nil {
		s.log.Error("punishing", "character", p.Name, "err", err)
	}
}

func (s *Server) schedulePrison(p *player, left time.Duration) {
	p.punish.prisonTimer = left
	p.punish.prisonTask = s.later(left, func() { s.setIsInPrison(p, false, 0) })
}

// loadPunishment is PlayerPunishmentsDAO.loadPlayerPunishments; a prisoner is put in the prison map before entering the world.
func (s *Server) loadPunishment(p *player) error {
	ms, err := s.punishDB.PrisonTimer(p.ID)
	p.punish.prisonTimer = time.Duration(ms) * time.Millisecond
	if p.inPrison() && p.WorldID != prisonMap {
		p.WorldID, p.instance, p.X, p.Y, p.Z = prisonMap, 0, prisonX, prisonY, prisonZ
	}
	return err
}

// prisonLogin is PunishmentService.updatePrisonStatus, once the player is in the world.
func (s *Server) prisonLogin(p *player) {
	if !p.inPrison() {
		return
	}
	s.schedulePrison(p, p.punish.prisonTimer)
	minutes := max(int(p.punish.prisonTimer.Minutes()+0.5), 1)
	plural := ""
	if minutes > 1 {
		plural = "s"
	}
	s.tell(p, fmt.Sprintf("You are still in prison for %d minute%s.", minutes, plural))
	p.punish.prisonStart = time.Now()
}

// prisonLogout takes the time served off the sentence and stops its clock; saveSentence keeps it.
func (s *Server) prisonLogout(p *player) {
	if p.inPrison() {
		p.punish.prisonTimer = max(p.punish.prisonTimer-time.Since(p.punish.prisonStart), 0)
	}
	p.punish.prisonTask.cancel()
	p.punish.gagTask.cancel()
}

func (s *Server) saveSentence(p *player) error {
	status := 0
	if p.inPrison() {
		status = 1
	}
	return s.punishDB.SavePunishment(p.ID, status, p.punish.prisonTimer.Milliseconds())
}

// gag is Gag: mute the player, for minutes or until ungagged.
func (s *Server) gag(p *player, minutes int) {
	p.punish.gagged = true
	if minutes != 0 {
		p.punish.gagTask.cancel()
		p.punish.gagTask = s.later(time.Duration(minutes)*time.Minute, func() {
			p.punish.gagged = false
			s.tell(p, "You have been ungagged")
		})
	}
}

// adminPunish runs the punishment commands; it says whether name was one.
func (s *Server) adminPunish(p *player, name string, params []string) bool {
	switch name {
	case "gag":
		s.adminGag(p, params)
	case "ungag":
		if target := s.gagTarget(p, params, "ungag <player>"); target != nil {
			target.punish.gagged = false
			target.punish.gagTask.cancel()
			s.tell(target, "You have been ungagged")
			s.tell(p, "Player "+target.Name+" ungagged")
		}
	case "sprison", "rprison":
		s.adminPrison(p, name == "sprison", params)
	case "kick":
		if len(params) < 1 {
			s.tell(p, "syntax //kick <character_name>")
		} else if target := s.onlineNamed(convertName(params[0])); target == nil {
			s.tell(p, "The specified player is not online.")
		} else {
			w := wire.Packet(smQuitResponse)
			w.D(1)
			w.C(0)
			target.conn.close(w)
			s.tell(p, "Kicked player : "+target.Name)
		}
	case "ban", "unban":
		s.adminBan(p, name == "ban", params)
	case "banip", "unbanip":
		usage := "Syntax: //banip <mask> [time in minutes]"
		if name == "unbanip" {
			usage = "Syntax: //unbanip <mask>"
		}
		minutes, ok := int32(0), len(params) > 0
		if name == "unbanip" {
			minutes = -1
		} else if len(params) > 1 {
			n, err := strconv.Atoi(params[1])
			minutes, ok = int32(n), ok && err == nil
		}
		if !ok {
			s.tell(p, usage)
			return true
		}
		s.sendBan(banTypeIP, 0, params[0], minutes, p.ID)
	default:
		return false
	}
	return true
}

// onlineNamed is World.findPlayer: a player in the world, even one whose map is still loading.
func (s *Server) onlineNamed(name string) *player {
	if p := s.playerNamed(name); p != nil {
		return p
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.players {
		if p := c.player; p != nil && strings.EqualFold(p.Name, name) {
			return p
		}
	}
	return nil
}

// gagTarget is the online player a gag command names, or nil, having told the admin.
func (s *Server) gagTarget(p *player, params []string, syntax string) *player {
	if len(params) < 1 {
		s.tell(p, "Syntax: //"+syntax)
		return nil
	}
	name := convertName(params[0])
	target := s.onlineNamed(name)
	if target == nil {
		s.tell(p, "Player "+name+" was not found!")
		s.tell(p, "Syntax: //"+syntax)
	}
	return target
}

func (s *Server) adminGag(p *player, params []string) {
	const syntax = "gag <player> [time in minutes]"
	target := s.gagTarget(p, params, syntax)
	if target == nil {
		return
	}
	minutes := 0
	if len(params) > 1 {
		var err error
		if minutes, err = strconv.Atoi(params[1]); err != nil {
			s.tell(p, "Syntax: //"+syntax)
			return
		}
	}
	s.gag(target, minutes)
	forTime := ""
	if minutes != 0 {
		forTime = fmt.Sprintf(" for %d minutes", minutes)
	}
	s.tell(target, "You have been gagged"+forTime)
	s.tell(p, "Player "+target.Name+" gagged"+forTime)
}

func (s *Server) adminPrison(p *player, send bool, params []string) {
	syntax := "syntax //rprison <player>"
	if send {
		syntax = "syntax //sprison <player> <delay>"
	}
	if len(params) == 0 || len(params) > 2 || send && len(params) != 2 {
		s.tell(p, syntax)
		return
	}
	minutes, err := strconv.Atoi(firstOf(params[1:]))
	if send && err != nil {
		s.tell(p, "Usage: "+strings.TrimPrefix(syntax, "syntax "))
		return
	}
	target := s.onlineNamed(convertName(params[0]))
	switch {
	case target == nil:
	case send:
		s.setIsInPrison(target, true, int64(minutes))
		s.tell(p, fmt.Sprintf("Player %s sent to prison for %d.", target.Name, minutes))
	default:
		s.setIsInPrison(target, false, 0)
		s.tell(p, "Player "+target.Name+" removed from prison.")
	}
}

// adminBan is //ban <player> [account|ip|full] [minutes] and //unban <player> [account|ip|full].
func (s *Server) adminBan(p *player, ban bool, params []string) {
	syntax := "Syntax: //unban <player> [account|ip|full]"
	if ban {
		syntax = "Syntax: //ban <player> [account|ip|full] [time in minutes]"
	}
	if len(params) < 1 {
		s.tell(p, syntax)
		return
	}
	name := convertName(params[0])
	var accountID int32
	if target := s.onlineNamed(name); ban && target != nil {
		accountID = target.conn.account.id
	} else if id, err := s.punishDB.AccountIDByName(name); err == nil {
		accountID = id
	}
	if accountID == 0 {
		s.tell(p, "Player "+name+" was not found!")
		s.tell(p, syntax)
		return
	}
	kind := byte(banTypeFull)
	if len(params) > 1 {
		kindName := strings.ToLower(params[1])
		switch {
		case strings.HasPrefix("account", kindName):
			kind = banTypeAccount
		case strings.HasPrefix("ip", kindName):
			kind = banTypeIP
		case strings.HasPrefix("full", kindName):
			kind = banTypeFull
		default:
			s.tell(p, syntax)
			return
		}
	}
	minutes := int32(-1) // negative lifts the ban
	if ban {
		minutes = 0 // for ever
		if len(params) > 2 {
			n, err := strconv.Atoi(params[2])
			if err != nil {
				s.tell(p, syntax)
				return
			}
			minutes = int32(n)
		}
	}
	s.sendBan(kind, accountID, "", minutes, p.ID)
}

// sendBan is LoginServer.sendBanPacket.
func (s *Server) sendBan(kind byte, accountID int32, ip string, minutes, adminID int32) {
	w := wire.Packet(0x06)
	w.C(kind)
	w.D(accountID)
	w.S(ip)
	w.D(minutes)
	w.D(adminID)
	s.login.send(w)
}
