package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

// adminSaver contains persistent administrative operations, separate from player gameplay.
type adminSaver interface {
	AddTitle(int32, int32) error
	AddDrop(int32, store.Drop) error
	Bookmarks(int32) ([]store.Bookmark, error)
	AddBookmark(int32, store.Bookmark) error
	DeleteBookmark(int32, string) error
	SaveAnnouncement(store.AutoAnnouncement) error
	DeleteAnnouncement(int32) error
	AutoAnnouncements() ([]store.AutoAnnouncement, error)
	LegionIDByName(string) (int32, error)
}

var extendedAdminCommands map[string]func(*Server, *player, []string)

func init() {
	extendedAdminCommands = map[string]func(*Server, *player, []string){
		"notice": (*Server).adminNotice, "announcefaction": (*Server).adminFaction,
		"movetoplayer": (*Server).adminMoveToPlayer, "movetome": (*Server).adminMoveToMe, "moveplayertoplayer": (*Server).adminMovePlayers, "movetonpc": (*Server).adminMoveToNPC,
		"invis": (*Server).adminInvisible, "invul": (*Server).adminInvulnerable, "dps": (*Server).adminDPS, "speed": (*Server).adminSpeed,
		"addskill": (*Server).adminSkill, "givemissingskills": (*Server).adminMissingSkills, "addtitle": (*Server).adminTitle, "addset": (*Server).adminItemSet, "remove": (*Server).adminRemove,
		"info": (*Server).adminInfo, "playerinfo": (*Server).adminPlayerInfo, "zone": (*Server).adminZone, "weather": (*Server).adminWeather,
		"quest": (*Server).adminQuest, "legion": (*Server).adminLegion, "ai": (*Server).adminAI,
		"spawn": (*Server).adminSpawn, "delete": (*Server).adminDeleteSpawn, "reload_spawn": (*Server).adminReloadSpawns, "save_spawn": (*Server).adminSaveSpawns,
		"bk": (*Server).adminBookmark, "adddrop": (*Server).adminDrop, "announcements": (*Server).adminAnnouncements,
		"dye": (*Server).adminDye, "appearance": (*Server).adminAppearance, "unstuck": (*Server).adminUnstuck,
		"promote": (*Server).adminPromote, "revoke": (*Server).adminRevoke,
		"reload": (*Server).adminReload, "configure": (*Server).adminConfigure, "sys": (*Server).adminSystem, "html": (*Server).adminHTML,
		"fsc": (*Server).adminFakePacket, "raw": (*Server).adminRawPacket, "send": (*Server).adminMappedPackets,
	}
}

func (s *Server) adminTarget(p *player) *player {
	if q, ok := s.creatureByID(p.targetID).(*player); ok {
		return q
	}
	return p
}
func (s *Server) adminUsage(p *player, usage string) { s.tell(p, "Syntax: //"+usage) }
func (s *Server) adminError(p *player, err error) bool {
	if err != nil {
		s.log.Error("admin operation", "character", p.Name, "err", err)
		s.tell(p, "Operation failed; see the server log.")
		return true
	}
	return false
}
func (s *Server) adminNamed(p *player, name string) *player {
	q := s.onlineNamed(convertName(name))
	if q == nil {
		s.tell(p, "The specified player is not online.")
	}
	return q
}
func (s *Server) adminNotice(p *player, args []string) {
	if len(args) == 0 {
		s.adminUsage(p, "notice <message>")
		return
	}
	s.announce("Information : " + strings.Join(args, " "))
}
func (s *Server) adminFaction(p *player, args []string) {
	if len(args) < 2 || args[0] != "ely" && args[0] != "asmo" {
		s.adminUsage(p, "announcefaction <ely|asmo> <message>")
		return
	}
	race, prefix := "ELYOS", "Elyos : "
	if args[0] == "asmo" {
		race, prefix = "ASMODIANS", "Asmodians : "
	}
	for _, q := range s.spawned {
		if q.Race == race {
			q.conn.send(message(chatSystemNotice, prefix+strings.Join(args[1:], " ")))
		}
	}
}
func (s *Server) adminMoveToPlayer(p *player, args []string) { s.adminMove(p, args, 0) }
func (s *Server) adminMoveToMe(p *player, args []string)     { s.adminMove(p, args, 1) }
func (s *Server) adminMovePlayers(p *player, args []string)  { s.adminMove(p, args, 2) }
func (s *Server) adminMove(p *player, args []string, kind int) {
	names := []string{"movetoplayer", "movetome", "moveplayertoplayer"}
	required := 1
	if kind == 2 {
		required = 2
	}
	if len(args) != required {
		s.adminUsage(p, names[kind]+" <player> [destination]")
		return
	}
	q := s.adminNamed(p, args[0])
	if q == nil {
		return
	}
	from, to := p, q
	if kind == 1 {
		from, to = q, p
	}
	if kind == 2 {
		from, to = q, s.adminNamed(p, args[1])
	}
	if to == nil {
		return
	}
	if from == to {
		s.tell(p, "Cannot move a player to their own position.")
		return
	}
	s.teleportToInstance(from, to.WorldID, to.instance, to.X, to.Y, to.Z, byte(to.Heading), 0)
	s.tell(p, "Teleported "+from.Name+" to "+to.Name+".")
}
func (s *Server) adminMoveToNPC(p *player, args []string) {
	id, err := strconv.ParseInt(firstOf(args), 10, 32)
	if err != nil || len(args) != 1 {
		s.adminUsage(p, "movetonpc <npc_id>")
		return
	}
	for _, objectID := range sortedKeys(s.byID) {
		o := s.byID[objectID]
		if o.npc != nil && o.npc.ID == int32(id) && !o.dead {
			s.teleportToInstance(p, o.worldID, o.instance, o.x, o.y, o.z, o.heading, 0)
			return
		}
	}
	s.tell(p, "No spawned NPC with that template ID.")
}
func (s *Server) adminInvisible(p *player, args []string) {
	if p.visualState < 3 {
		p.visualState = 3
		p.fx.set(effectInvisible)
		s.tell(p, "You are invisible.")
	} else {
		p.visualState = 0
		p.fx.unset(effectInvisible)
		s.tell(p, "You are visible.")
	}
	p.broadcast(playerState(p), true)
}
func (s *Server) adminInvulnerable(p *player, args []string) {
	p.adminInvulnerable = !p.adminInvulnerable
	if p.adminInvulnerable {
		s.tell(p, "You are now immortal.")
	} else {
		s.tell(p, "You are now mortal.")
	}
}
func (s *Server) adminSpeed(p *player, args []string) {
	n, err := strconv.Atoi(firstOf(args))
	if err != nil || len(args) != 1 || n < 0 || n > 200 {
		s.adminUsage(p, "speed <percent 0-200>")
		return
	}
	p.stats.initStat(data.Speed, int32(6000+6000*n/100))
	p.stats.initStat(data.FlySpeed, int32(9000+9000*n/100))
	p.conn.send(s.statsInfo(p))
	p.broadcast(s.playerEmotion(p, emoteStartEmote2, 0, 0, 0, 0, 0), true)
}
func (s *Server) adminSkill(p *player, args []string) {
	if len(args) != 2 {
		s.adminUsage(p, "addskill <skillId> <skillLevel>")
		return
	}
	id, e1 := strconv.ParseInt(args[0], 10, 32)
	level, e2 := strconv.ParseInt(args[1], 10, 32)
	if e1 != nil || e2 != nil || level < 1 || level > 65535 || s.data.Skills[int32(id)] == nil {
		s.tell(p, "Invalid skill ID or level.")
		return
	}
	s.addSkill(s.adminTarget(p), int32(id), int32(level), true)
}
func (s *Server) adminMissingSkills(p *player, args []string) {
	for level := 0; level <= p.level; level++ {
		s.learnSkillsFor(p, level, p.Class, true)
	}
	if index := slices.Index(classNames, p.Class); index >= 0 && index%3 != 0 {
		for level := 1; level < 10; level++ {
			s.learnSkillsFor(p, level, classNames[index-index%3], true)
		}
		s.switchCraftingSkill(p)
	}
	p.stats = s.playerStats(p)
	p.conn.send(s.statsInfo(p))
}
func (s *Server) adminTitle(p *player, args []string) {
	id, err := strconv.ParseInt(firstOf(args), 10, 32)
	if err != nil || id < 1 || id > 50 || len(args) > 2 {
		s.adminUsage(p, "addtitle <title_id 1-50> [player]")
		return
	}
	q := s.adminTarget(p)
	if len(args) == 2 {
		q = s.adminNamed(p, args[1])
	}
	if q == nil {
		return
	}
	if q.Race == "ASMODIANS" {
		id += 50
	}
	if s.data.Titles[int32(id)] == nil {
		s.tell(p, "Unknown title.")
		return
	}
	if slices.Contains(q.titles, int32(id)) {
		s.tell(p, "Player already owns that title.")
		return
	}
	if s.adminError(p, s.adminDB.AddTitle(q.ID, int32(id))) {
		return
	}
	q.titles = append(q.titles, int32(id))
	q.conn.send(titleList(q))
	s.tell(p, "Title added successfully.")
}
func (s *Server) adminItemSet(p *player, args []string) {
	q, values := s.adminReceiver(p, args)
	if q == nil {
		return
	}
	id, err := strconv.ParseInt(firstOf(values), 10, 32)
	if err != nil || len(values) != 1 {
		s.adminUsage(p, "addset [player] <itemset ID>")
		return
	}
	var set *data.ItemSet
	for _, candidate := range s.data.ItemSets {
		if candidate.ID == int32(id) {
			set = candidate
			break
		}
	}
	if set == nil {
		s.tell(p, "ItemSet does not exist.")
		return
	}
	if q.cubeLimit()-len(q.cube) < len(set.Parts) {
		s.tell(p, "Not enough free inventory slots.")
		return
	}
	for _, part := range set.Parts {
		if !s.addItem(q, part.ItemID, 1) {
			s.tell(p, "Could not add every item.")
			return
		}
	}
	s.tell(p, "Item Set added successfully.")
}
func (s *Server) adminRemove(p *player, args []string) {
	if len(args) < 2 || len(args) > 3 {
		s.adminUsage(p, "remove <player> <item ID> [quantity]")
		return
	}
	q := s.adminNamed(p, args[0])
	if q == nil {
		return
	}
	id, err := strconv.ParseInt(args[1], 10, 32)
	count := int64(1)
	if len(args) == 3 {
		var e error
		count, e = strconv.ParseInt(args[2], 10, 64)
		if e != nil {
			err = e
		}
	}
	if err != nil || count < 1 {
		s.tell(p, "Invalid item ID or quantity.")
		return
	}
	for _, item := range slices.Clone(q.cube) {
		if item.ItemID == int32(id) {
			s.decreaseItemCount(q, item, count)
			s.tell(p, "Item(s) removed successfully.")
			return
		}
	}
	s.tell(p, "Items with that ID are not in the player's bag.")
}
func (s *Server) adminInfo(p *player, args []string) {
	if o := s.byID[p.targetID]; o != nil {
		id := int32(0)
		if o.npc != nil {
			id = o.npc.ID
		} else if o.gatherable != nil {
			id = o.gatherable.ID
		}
		s.tell(p, fmt.Sprintf("Template: %d ObjectId: %d Map: %d X: %.3f Y: %.3f Z: %.3f Heading: %d", id, o.id, o.worldID, o.x, o.y, o.z, o.heading))
		return
	}
	s.adminDescribePlayer(p, s.adminTarget(p), "loc")
}
func (s *Server) adminPlayerInfo(p *player, args []string) {
	if len(args) < 1 || len(args) > 2 {
		s.adminUsage(p, "playerinfo <player> [loc|item|group|skill|legion]")
		return
	}
	q := s.adminNamed(p, args[0])
	if q == nil {
		return
	}
	kind := ""
	if len(args) == 2 {
		kind = args[1]
	}
	s.adminDescribePlayer(p, q, kind)
}
func (s *Server) adminDescribePlayer(p, q *player, kind string) {
	s.tell(p, fmt.Sprintf("%s: level %d %s %s; account %s; IP %s", q.Name, q.level, q.Race, q.Class, q.conn.account.name, q.conn.ip))
	switch kind {
	case "":
	case "loc":
		s.tell(p, fmt.Sprintf("Map %d Instance %d X %.3f Y %.3f Z %.3f Heading %d", q.WorldID, q.instance, q.X, q.Y, q.Z, q.Heading))
	case "item":
		for _, item := range append(append(slices.Clone(q.cube), q.equipment...), q.warehouse...) {
			s.tell(p, fmt.Sprintf("Item %d count %d location %d equipped %t", item.ItemID, item.Count, item.Location, item.Equipped))
		}
	case "skill":
		for _, skill := range q.skills {
			s.tell(p, fmt.Sprintf("Skill %d level %d", skill.ID, skill.Level))
		}
	case "group":
		if q.group == nil {
			s.tell(p, "No group.")
		} else {
			for _, m := range q.group.members {
				s.tell(p, m.Name)
			}
		}
	case "legion":
		if q.legion == nil {
			s.tell(p, "No legion.")
		} else {
			s.tell(p, fmt.Sprintf("Legion %s level %d points %d", q.legion.Name, q.legion.Level, q.legion.Contribution))
			for _, m := range q.legion.members {
				s.tell(p, m.Name+" "+m.Rank)
			}
		}
	default:
		s.adminUsage(p, "playerinfo <player> [loc|item|group|skill|legion]")
	}
}
func (s *Server) adminZone(p *player, args []string) {
	if len(args) > 1 || len(args) == 1 && args[0] != "refresh" {
		s.adminUsage(p, "zone [refresh]")
		return
	}
	s.refreshZone(p)
	if p.zone == nil {
		s.tell(p, "You are out of any zone.")
	} else {
		s.tell(p, "You are in zone: "+p.zone.Name)
	}
}
func (s *Server) adminWeather(p *player, args []string) {
	if len(args) == 1 && args[0] == "reset" {
		s.mu.Lock()
		s.weathers = map[int32]weatherState{}
		s.mu.Unlock()
		for _, q := range s.spawned {
			q.conn.send(weather(s.weather(q.WorldID)))
		}
		return
	}
	if len(args) != 2 {
		s.adminUsage(p, "weather <region> <value 0-8> | reset")
		return
	}
	town, ok := towns[strings.ToLower(args[0])]
	n, err := strconv.Atoi(args[1])
	if !ok || err != nil || n < 0 || n > 8 {
		s.tell(p, "Unknown region or invalid weather (0-8).")
		return
	}
	world := int32(town[0])
	s.mu.Lock()
	s.weathers[world] = weatherState{code: byte(n), since: time.Now()}
	s.mu.Unlock()
	for _, q := range s.spawned {
		if q.WorldID == world {
			q.conn.send(weather(byte(n)))
		}
	}
}
func (s *Server) adminQuest(p *player, args []string) {
	if len(args) != 2 && len(args) != 4 {
		s.adminUsage(p, "quest start <id> | set <id> <status> <var>")
		return
	}
	id, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil || s.data.Quests[int32(id)] == nil {
		s.tell(p, "Unknown quest ID.")
		return
	}
	q := s.adminTarget(p)
	if args[0] == "start" && len(args) == 2 {
		script := s.data.QuestScripts[int32(id)]
		if script == nil {
			s.tell(p, "No quest handler.")
			return
		}
		q.conn.startQuest(script, 0)
		return
	}
	if args[0] != "set" || len(args) != 4 {
		s.adminUsage(p, "quest start <id> | set <id> <status> <var>")
		return
	}
	status := strings.ToUpper(args[2])
	if !slices.Contains([]string{"NONE", "START", "REWARD", "COMPLETE", "LOCKED"}, status) {
		s.tell(p, "Invalid quest status.")
		return
	}
	value, e := strconv.ParseInt(args[3], 10, 32)
	state := q.quest(int32(id))
	if e != nil || state == nil {
		s.tell(p, "Quest must exist and var must be an integer.")
		return
	}
	next := *state
	next.Status, next.Vars = status, int32(value)
	if s.adminError(p, s.quests.SaveQuest(q.ID, next)) {
		return
	}
	*state = next
	q.conn.send(questAccepted(2, next))
}
func (s *Server) adminLegion(p *player, args []string) {
	if len(args) < 2 || len(args) > 3 {
		s.adminUsage(p, "legion <disband|setlevel|setpoints|setname> <legion> [value]")
		return
	}
	var l *legion
	for _, candidate := range s.legions {
		if strings.EqualFold(candidate.Name, args[1]) {
			l = candidate
			break
		}
	}
	if l == nil {
		id, err := s.adminDB.LegionIDByName(args[1])
		if s.adminError(p, err) {
			return
		}
		l, err = s.legionOf(id)
		if s.adminError(p, err) {
			return
		}
	}
	if l == nil {
		s.tell(p, "Legion not found.")
		return
	}
	if args[0] == "disband" && len(args) == 2 {
		s.disbandLegion(l)
		return
	}
	if len(args) != 3 {
		s.adminUsage(p, "legion <setlevel|setpoints|setname> <legion> <value>")
		return
	}
	before := l.Legion
	switch args[0] {
	case "setname":
		if !legionNamePattern.MatchString(args[2]) {
			s.tell(p, "Invalid legion name.")
			return
		}
		used, err := s.legionDB.LegionNameUsed(args[2])
		if s.adminError(p, err) {
			return
		}
		if used {
			s.tell(p, "Legion name already used.")
			return
		}
		l.Name = args[2]
	case "setlevel", "setpoints":
		n, err := strconv.ParseInt(args[2], 10, 32)
		if err != nil || n < 0 || args[0] == "setlevel" && (n < 1 || n > 5) || args[0] == "setpoints" && n > 2000000000 {
			s.tell(p, "Invalid value.")
			return
		}
		if args[0] == "setlevel" {
			l.Level = int32(n)
		} else {
			l.Contribution = int32(n)
		}
	default:
		s.adminUsage(p, "legion <disband|setlevel|setpoints|setname> <legion> [value]")
		return
	}
	if err := s.legionDB.UpdateLegion(&l.Legion); err != nil {
		l.Legion = before
		s.adminError(p, err)
		return
	}
	s.tellLegion(l, legionInfo(l))
	for _, q := range s.online(l) {
		q.broadcast(legionTitle(q, l.ID, l.Name, byte(q.member.rank)), true)
	}
	s.tell(p, "Legion updated.")
}
func (s *Server) adminAI(p *player, args []string) {
	if len(args) != 1 || args[0] != "info" {
		s.adminUsage(p, "ai info")
		return
	}
	o := s.byID[p.targetID]
	if o == nil || o.ai == nil {
		s.tell(p, "Select an NPC first.")
		return
	}
	s.tell(p, fmt.Sprintf("Ai state: %d; desires: %d; scheduled: %t", o.ai.state, len(o.ai.desires), o.ai.task != nil && !o.ai.task.cancelled))
}
func (s *Server) adminUnstuck(p *player, args []string) {
	if p.dead {
		s.tell(p, "Cannot unstuck a dead player.")
		return
	}
	s.moveToBind(p, true, 0)
}

// finiteAdminFloat rejects NaN/Inf before coordinates or appearance enter the world.
func finiteAdminFloat(text string) (float32, error) {
	n, err := strconv.ParseFloat(text, 32)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("invalid finite number")
	}
	return float32(n), nil
}
