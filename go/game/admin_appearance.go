package game

import (
	"aionlightning/wire"
	"slices"
	"strconv"
	"strings"
)

var adminDyeColors = map[string]string{"turquoise": "198d94", "blue": "1f87f5", "brown": "66250e", "purple": "c38df5", "true red": "c22626", "true white": "ffffff", "white": "ffffff", "black": "000000", "true black": "000000", "hot orange": "e36b00", "rich purple": "440b9a", "hot pink": "d60b7e", "mustard": "fcd251", "green tea": "61bb4f", "olive green": "5f730e", "deep blue": "14398b", "romantic purple": "80185d", "wiki": "85e831", "omblic": "ff5151", "meon": "afaf26", "ormea": "ffaa11", "tange": "bd5fff", "ervio": "3bb7fe", "lunime": "c7af27", "vinna": "052775", "kirka": "ca84ff", "brommel": "c7af27", "pressa": "ff9d29", "merone": "8df598", "kukar": "ffff96", "leopis": "31dfff"}

func (s *Server) adminDye(p *player, args []string) {
	if len(args) < 1 || len(args) > 2 {
		s.adminUsage(p, "dye <color|hex color|no>")
		return
	}
	color := strings.ToLower(strings.Join(args, " "))
	if len(args) == 2 && strings.EqualFold(args[1], "petal") {
		color = strings.ToLower(args[0])
	}
	if mapped, ok := adminDyeColors[color]; ok {
		color = mapped
	}
	var bgra int32
	if color != "no" {
		rgb, err := strconv.ParseUint(strings.TrimPrefix(color, "#"), 16, 24)
		if err != nil {
			s.tell(p, "Unknown dye color.")
			return
		}
		bgra = int32(0xff | ((rgb & 0xff) << 24) | ((rgb & 0xff00) << 8) | ((rgb & 0xff0000) >> 8))
	}
	q := s.adminTarget(p)
	for _, item := range q.equipment {
		if s.template(item) == nil || s.template(item).IsStigma() {
			continue
		}
		item.Color = bgra
		s.saveItem(item)
		q.conn.send(s.updateItemPacket(q, item))
	}
	q.broadcast(s.appearancePacket(q), true)
	s.tell(p, "Player dyed successfully.")
}
func (s *Server) adminAppearance(p *player, args []string) {
	usage := "appearance <size|voice|hair|face|deco|head_size|tattoo|reset> [value]"
	if len(args) < 1 || len(args) > 2 {
		s.adminUsage(p, usage)
		return
	}
	q := s.adminTarget(p)
	if args[0] == "reset" && len(args) == 1 {
		if q.adminAppearance == nil {
			s.tell(p, "The player already has the normal appearance.")
			return
		}
		*q.appearance = *q.adminAppearance
		q.adminAppearance = nil
		s.teleportToInstance(q, q.WorldID, q.instance, q.X, q.Y, q.Z, byte(q.Heading), 0)
		return
	}
	if len(args) != 2 {
		s.adminUsage(p, usage)
		return
	}
	next := *q.appearance
	if args[0] == "size" {
		value, err := finiteAdminFloat(args[1])
		if err != nil || value < 0 || value > 50 {
			s.tell(p, "Size must be between 0 and 50.")
			return
		}
		next.Height = value
	} else {
		n, err := strconv.ParseInt(args[1], 10, 32)
		if err != nil || n < 0 {
			s.tell(p, "Invalid appearance value.")
			return
		}
		limits := map[string]int64{"voice": 3, "hair": 43, "face": 24, "deco": 18, "head_size": 100, "tattoo": 13}
		limit, ok := limits[args[0]]
		if !ok || n > limit || n == 0 && (args[0] == "hair" || args[0] == "face" || args[0] == "deco" || args[0] == "tattoo") {
			s.adminUsage(p, usage)
			return
		}
		switch args[0] {
		case "voice":
			next.Voice = int32(n)
		case "hair":
			next.Hair = int32(n)
		case "face":
			next.Face = int32(n)
		case "deco":
			next.Deco = int32(n)
		case "head_size":
			next.HeadSize = int32(n + 200)
		case "tattoo":
			next.Tattoo = int32(n)
		}
	}
	if q.adminAppearance == nil {
		saved := *q.appearance
		q.adminAppearance = &saved
	}
	*q.appearance = next
	s.teleportToInstance(q, q.WorldID, q.instance, q.X, q.Y, q.Z, byte(q.Heading), 0)
}
func (s *Server) adminSetMore(p *player, args []string, n int64) bool {
	q := s.adminTarget(p)
	switch args[0] {
	case "ap":
		if n < 0 || n > 2147483647 {
			s.tell(p, "AP must be between 0 and 2147483647.")
			return true
		}
		s.addAP(q, int32(n)-q.abyss.AP)
	case "class":
		old := slices.Index(classNames, q.Class)
		if q.level < 9 || old < 0 || old%3 != 0 || n != int64(old+1) && n != int64(old+2) {
			s.tell(p, "Invalid class switch; select an advanced class for a level 9 base class.")
			return true
		}
		s.setClass(q, classNames[n])
	case "title":
		if n < 0 || n > 106 || n != 0 && s.data.Titles[int32(n)] == nil {
			s.tell(p, "Invalid title ID.")
			return true
		}
		q.TitleID = int32(n)
		w := wire.Packet(smTitleSet)
		w.D(q.TitleID)
		q.conn.send(w)
		u := wire.Packet(smTitleUpdate)
		u.D(q.ID)
		u.D(q.TitleID)
		q.broadcast(u, false)
		q.stats = s.playerStats(q)
		q.conn.send(s.statsInfo(q))
	default:
		return false
	}
	return true
}
