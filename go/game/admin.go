package game

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// Admin commands use Java's default access level 3; //configure can override individual levels.

const chatSystemNotice = 0x1a // ChatType.SYSTEM_NOTICE

// towns are the places //goto knows: map, x, y and z.
var towns = map[string][4]float32{
	"poeta": {210010000, 806, 1242, 119}, "verteron": {210030000, 1643, 1500, 119}, "eltnen": {210020000, 343, 2724, 264},
	"theobomos": {210060000, 1398, 1557, 31}, "heiron": {210040000, 2540, 343, 411}, "sanctum": {110010000, 1322, 1511, 568},
	"ishalgen": {220010000, 529, 2449, 281}, "altgard": {220030000, 1748, 1807, 254}, "morheim": {220020000, 308, 2274, 449},
	"brusthonin": {220050000, 2917, 2421, 15}, "beluslan": {220040000, 325, 336, 229}, "pandaemonium": {120010000, 1679, 1400, 195},
	"inggison": {210050000, 1335, 276, 590}, "gelkmaros": {220070000, 795, 1684, 362}, "kaisinel": {110020000, 2155, 1567, 1205},
	"marchutan": {120020000, 1557, 1429, 266},
}

// adminCommand runs a "//command params" message of the game master p.
func (s *Server) adminCommand(p *player, text string) {
	name, rest, _ := strings.Cut(strings.TrimPrefix(text, "//"), " ")
	params := strings.Fields(rest)
	if p.conn.account.accessLevel < s.adminCommandLevel(name) {
		s.tell(p, "You dont have enough rights to execute this command")
		return
	}
	if handler := extendedAdminCommands[name]; handler != nil {
		handler(s, p, params)
		return
	}
	switch name {
	case "add":
		s.adminAdd(p, params)
	case "kinah":
		to, args := s.adminReceiver(p, params)
		n, err := strconv.ParseInt(strings.Join(args, ""), 10, 64)
		if to == nil || err != nil {
			s.tell(p, "syntax //kinah [player] <quantity>")
			return
		}
		s.increaseKinah(to, n)
		s.tell(p, "Kinah given successfully.")
	case "goto":
		town, ok := towns[strings.ToLower(strings.Join(params, ""))]
		if !ok {
			s.tell(p, "syntax //goto <location>")
			return
		}
		s.teleportTo(p, int32(town[0]), town[1], town[2], town[3], 0, 0)
	case "moveto":
		var v [4]float64
		for i := range v {
			if i >= len(params) {
				s.tell(p, "syntax //moveto worldId X Y Z")
				return
			}
			var err error
			if v[i], err = strconv.ParseFloat(params[i], 32); err != nil || math.IsNaN(v[i]) || math.IsInf(v[i], 0) {
				s.tell(p, "All the parameters should be numbers")
				return
			}
		}
		if s.data.WorldMaps[int32(v[0])] == nil {
			s.tell(p, fmt.Sprintf("Illegal WorldId %d", int32(v[0])))
			return
		}
		s.teleportTo(p, int32(v[0]), float32(v[1]), float32(v[2]), float32(v[3]), 0, 0)
	case "heal":
		if target := s.creatureByID(p.targetID); target != nil {
			hp, maxHP := target.hitPoints()
			s.increaseHP(target, statusNaturalHP, maxHP+1-hp)
			s.increaseMP(target, statusNaturalMP, target.gameStats().current(data.MaxMP)+1)
		} else {
			s.tell(p, "No target selected")
		}
	case "kill":
		if target := s.creatureByID(p.targetID); target != nil {
			_, maxHP := target.hitPoints()
			s.gotHit(target, p, 0, statusRegular, maxHP+1)
		} else {
			s.tell(p, "No target selected")
		}
	case "rez":
		s.adminRez(p, params)
	case "set":
		s.adminSet(p, params)
	case "morph":
		s.adminMorph(p, params)
	case "siege":
		s.adminSiege(p, params)
	case "announce":
		who, message, _ := strings.Cut(rest, " ")
		switch {
		case message == "":
			s.tell(p, "Syntax: //announce <anonymous|name> <message>")
		case strings.HasPrefix("anonymous", strings.ToLower(who)):
			s.announce("Announce: " + message)
		case strings.HasPrefix("name", strings.ToLower(who)):
			s.announce(p.Name + ": " + message)
		default:
			s.tell(p, "Syntax: //announce <anonymous|name> <message>")
		}
	case "petition":
		s.adminPetition(p, rest)
	default:
		if !s.adminPunish(p, name, params) {
			s.tell(p, "<There is no such admin command: "+name+">")
		}
	}
}

// tell is PacketSendUtility.sendMessage.
func (s *Server) tell(p *player, text string) { p.conn.send(message(chatAnnouncement, text)) }

func (s *Server) announce(text string) {
	for _, p := range s.spawned {
		p.conn.send(message(chatSystemNotice, text))
	}
}

// adminReceiver takes the player a command names first, if it does: the rest are its arguments.
func (s *Server) adminReceiver(p *player, params []string) (*player, []string) {
	if _, err := strconv.Atoi(firstOf(params)); len(params) > 1 && err != nil {
		to := s.playerNamed(convertName(params[0]))
		if to == nil {
			s.tell(p, "Could not find a player by that name.")
		}
		return to, params[1:]
	}
	return p, params
}

func (s *Server) adminAdd(p *player, params []string) {
	to, args := s.adminReceiver(p, params)
	if to == nil {
		return
	}
	id, err := strconv.Atoi(firstOf(args))
	count := int64(1)
	if len(args) > 1 {
		count, _ = strconv.ParseInt(args[1], 10, 64)
	}
	if err != nil || s.data.Items[int32(id)] == nil {
		s.tell(p, "syntax //add <player> <item ID> <quantity>")
		return
	}
	if s.addItem(to, int32(id), count) {
		s.tell(p, "Item(s) added successfully")
	}
}

func firstOf(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

func (s *Server) adminRez(p *player, params []string) {
	target, ok := s.creatureByID(p.targetID).(*player)
	if !ok || !target.dead {
		s.tell(p, "You can only resurrect dead players.")
		return
	}
	if len(params) > 0 && strings.HasPrefix("instant", params[0]) {
		s.reviveWithEmotion(target, 10)
		return
	}
	w := wire.Packet(smResurrect)
	w.S(p.Name)
	w.H(0)
	w.D(0)
	target.conn.send(w)
}

// adminSet is //set level|exp <value>, on the selected player or the admin.
func (s *Server) adminSet(p *player, params []string) {
	target := p
	if other, ok := s.creatureByID(p.targetID).(*player); ok {
		target = other
	}
	if len(params) < 2 {
		s.tell(p, "syntax //set <class|exp|ap|level|title>")
		return
	}
	n, err := strconv.ParseInt(params[1], 10, 64)
	if err != nil {
		s.tell(p, "You should enter valid second params!")
		return
	}
	if s.adminSetMore(p, params, n) {
		return
	}
	switch params[0] {
	case "exp":
		s.setExp(target, n)
	case "level":
		if n >= 1 && n <= 51 {
			s.setExp(target, s.data.ExpStart(int(n)))
		}
	default:
		s.tell(p, "syntax //set <class|exp|ap|level|title>")
	}
}

func (s *Server) adminMorph(p *player, params []string) {
	target := p
	if other, ok := s.creatureByID(p.targetID).(*player); ok {
		target = other
	}
	var model int64
	if len(params) == 1 && !strings.HasPrefix("cancel", strings.ToLower(params[0])) {
		var err error
		if model, err = strconv.ParseInt(params[0], 10, 32); err != nil || model < 200000 || model > 298021 {
			s.tell(p, "Something wrong with the NPC Id!")
			return
		}
	} else if len(params) != 1 {
		s.tell(p, "syntax //morph <NPC Id | cancel>")
		return
	}
	setModel(target, int32(model))
	target.broadcast(transformPacket(target), true)
}
