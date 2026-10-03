package game

import (
	"aionlightning/wire"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// adminFile restricts administrative file commands to the requested data directory.
func adminFile(root, name string) ([]byte, error) {
	if !filepath.IsLocal(name) {
		return nil, fmt.Errorf("invalid file name")
	}
	path := filepath.Join(root, name)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(base, resolved)
	if err != nil || !filepath.IsLocal(relative) {
		return nil, fmt.Errorf("file is outside data directory")
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if info.Size() > 8<<20 {
		return nil, fmt.Errorf("file too large")
	}
	return os.ReadFile(resolved)
}
func adminPacketElement(w *wire.Writer, kind byte, text string) error {
	switch kind {
	case 's':
		w.S(text)
	case 'f', 'e':
		value, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
			return fmt.Errorf("invalid float")
		}
		if kind == 'f' {
			if math.Abs(value) > math.MaxFloat32 {
				return fmt.Errorf("float out of range")
			}
			w.F(float32(value))
		} else {
			w.B(binary.LittleEndian.AppendUint64(nil, math.Float64bits(value)))
		}
	case 'c', 'h', 'd', 'q':
		value, err := strconv.ParseInt(text, 0, 64)
		if err != nil {
			return err
		}
		switch kind {
		case 'c':
			if value < -128 || value > 255 {
				return fmt.Errorf("byte out of range")
			}
			w.C(byte(value))
		case 'h':
			if value < -32768 || value > 65535 {
				return fmt.Errorf("short out of range")
			}
			w.H(uint16(value))
		case 'd':
			if value < math.MinInt32 || value > math.MaxUint32 {
				return fmt.Errorf("integer out of range")
			}
			w.D(int32(value))
		case 'q':
			w.Q(value)
		}
	default:
		return fmt.Errorf("unknown packet format")
	}
	if len(w.Data) > 65530 {
		return fmt.Errorf("packet too large")
	}
	return nil
}
func adminCustomPacket(opcode, format string, values []string) (*wire.Writer, error) {
	op, err := strconv.ParseUint(opcode, 0, 8)
	if err != nil {
		return nil, err
	}
	if len(format) != len(values) {
		return nil, fmt.Errorf("format and value counts differ")
	}
	w := wire.Packet(byte(op))
	for i := range format {
		if err := adminPacketElement(w, format[i], values[i]); err != nil {
			return nil, err
		}
	}
	return w, nil
}
func (s *Server) adminFakePacket(p *player, args []string) {
	if len(args) < 3 {
		s.adminUsage(p, "fsc <opcode> <format c|h|d|q|f|e|s> <values...>")
		return
	}
	w, err := adminCustomPacket(args[0], args[1], args[2:])
	if err != nil {
		s.tell(p, "Invalid packet: "+err.Error())
		return
	}
	p.conn.send(w)
}
func adminPacketRoot() string {
	if path := os.Getenv("AION_PACKETS"); path != "" {
		return path
	}
	return filepath.Join(filepath.Dir(adminDataRoot()), "packets")
}
func (s *Server) adminRawPacket(p *player, args []string) {
	if len(args) != 1 {
		s.adminUsage(p, "raw <name>")
		return
	}
	content, err := adminFile(adminPacketRoot(), args[0]+".txt")
	if s.adminError(p, err) {
		return
	}
	var payload []byte
	for _, line := range strings.Split(string(content), "\n") {
		if len(line) > 48 {
			line = line[:48]
		}
		for _, token := range strings.Fields(line) {
			part, e := hex.DecodeString(token)
			if e != nil || len(part) != 1 {
				s.tell(p, "Invalid raw hexadecimal packet.")
				return
			}
			payload = append(payload, part...)
		}
	}
	if len(payload) < 3 || len(payload) > 65530 {
		s.tell(p, "Invalid raw packet length.")
		return
	}
	w := wire.Packet(payload[0])
	w.B(payload[3:])
	p.conn.send(w)
}

type adminPacketMapping struct {
	Delay   int64               `xml:"delay,attr"`
	Packets []adminMappedPacket `xml:"packet"`
}
type adminMappedPacket struct {
	Opcode string            `xml:"opcode,attr"`
	Parts  []adminPacketPart `xml:"part"`
}
type adminPacketPart struct {
	Type   string `xml:"type,attr"`
	Value  string `xml:"value,attr"`
	Repeat int    `xml:"repeat,attr"`
}

func (s *Server) adminMappedPackets(p *player, args []string) {
	if len(args) != 1 {
		s.adminUsage(p, "send <mapping_name>")
		return
	}
	content, err := adminFile(adminPacketRoot(), args[0]+".xml")
	if s.adminError(p, err) {
		return
	}
	var mapping adminPacketMapping
	if s.adminError(p, xml.Unmarshal(content, &mapping)) {
		return
	}
	if len(mapping.Packets) == 0 || len(mapping.Packets) > 256 || mapping.Delay < 0 || mapping.Delay > 60000 {
		s.tell(p, "Invalid packet mapping count or delay.")
		return
	}
	q := s.adminTarget(p)
	replacer := strings.NewReplacer("${objectId}", strconv.Itoa(int(q.ID)), "${targetObjectId}", strconv.Itoa(int(q.ID)), "${senderObjectId}", strconv.Itoa(int(p.ID)))
	var packets []*wire.Writer
	for _, template := range mapping.Packets {
		op, e := strconv.ParseUint(template.Opcode, 0, 8)
		if e != nil {
			s.tell(p, "Invalid mapped opcode.")
			return
		}
		w := wire.Packet(byte(op))
		for _, part := range template.Parts {
			if len(part.Type) != 1 || part.Repeat < 0 || part.Repeat > 1024 {
				s.tell(p, "Invalid mapped part.")
				return
			}
			for range max(part.Repeat, 1) {
				if e := adminPacketElement(w, part.Type[0], replacer.Replace(part.Value)); e != nil {
					s.tell(p, "Invalid packet mapping: "+e.Error())
					return
				}
			}
		}
		packets = append(packets, w)
	}
	for i, w := range packets {
		if i == 0 || mapping.Delay == 0 {
			q.conn.send(w)
		} else {
			s.later(time.Duration(int64(i)*mapping.Delay)*time.Millisecond, func() {
				if q.spawned {
					q.conn.send(w)
				}
			})
		}
	}
}
func (s *Server) adminHTML(p *player, args []string) {
	if len(args) == 1 && args[0] == "reload" {
		content, err := adminFile(filepath.Join(adminDataRoot(), "HTML"), "welcome.xhtml")
		if s.adminError(p, err) {
			return
		}
		s.data.Welcome = string(content)
		s.tell(p, "HTML cache reloaded.")
		return
	}
	if len(args) != 2 || args[0] != "test" {
		s.adminUsage(p, "html reload | test <filename.html>")
		return
	}
	content, err := adminFile(filepath.Join(adminDataRoot(), "HTML"), args[1])
	if s.adminError(p, err) {
		return
	}
	s.showHTML(p, string(content))
}
func (s *Server) adminSystem(p *player, args []string) {
	if len(args) == 0 {
		s.adminUsage(p, "sys <info|memory|gc|restart|shutdown|cancel> [seconds] [announce interval]")
		return
	}
	switch args[0] {
	case "info":
		s.tell(p, fmt.Sprintf("AionGo %s %s/%s CPUs=%d Goroutines=%d Players=%d", runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.NumGoroutine(), len(s.spawned)))
	case "memory", "gc":
		if args[0] == "gc" {
			runtime.GC()
		}
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		s.tell(p, fmt.Sprintf("Heap %.2f MiB; system %.2f MiB; collections %d", float64(m.HeapAlloc)/(1<<20), float64(m.Sys)/(1<<20), m.NumGC))
	case "cancel":
		for _, t := range s.adminShutdownTasks {
			t.cancel()
		}
		s.adminShutdownTasks = nil
		s.announce("Server shutdown cancelled.")
	case "restart", "shutdown":
		if len(args) != 3 {
			s.adminUsage(p, "sys <restart|shutdown> <seconds> <announce interval>")
			return
		}
		seconds, e1 := strconv.Atoi(args[1])
		interval, e2 := strconv.Atoi(args[2])
		if e1 != nil || e2 != nil || seconds < 0 || seconds > 86400 || interval < 1 {
			s.tell(p, "Invalid shutdown time or interval.")
			return
		}
		for _, t := range s.adminShutdownTasks {
			t.cancel()
		}
		s.adminShutdownTasks = nil
		action := args[0]
		if seconds > 0 {
			left := seconds
			countdown := s.every(0, time.Duration(interval)*time.Second, func() {
				if left > 0 {
					s.announce(fmt.Sprintf("Server %s in %d seconds.", action, left))
					left -= interval
				}
			})
			s.adminShutdownTasks = append(s.adminShutdownTasks, countdown)
		}

		restart := args[0] == "restart"
		s.adminShutdownTasks = append(s.adminShutdownTasks, s.later(time.Duration(seconds)*time.Second, func() { s.shutdownPlayers(restart) }))
	default:
		s.adminUsage(p, "sys <info|memory|gc|restart|shutdown|cancel> [seconds] [announce interval]")
	}
}
func (s *Server) shutdownPlayers(restart bool) {
	for _, t := range s.adminShutdownTasks {
		t.cancel()
	}
	s.adminShutdownTasks = nil
	for _, p := range s.spawned {
		p.conn.close(quitResponse())
	}
	if s.config.Shutdown != nil {
		go s.config.Shutdown(restart)
	}
}
func (s *Server) adminPromote(p *player, args []string) { s.adminAccess(p, args, false) }
func (s *Server) adminRevoke(p *player, args []string)  { s.adminAccess(p, args, true) }
func (s *Server) adminAccess(p *player, args []string, revoke bool) {
	required := 3
	name := "promote"
	if revoke {
		required = 2
		name = "revoke"
	}
	if len(args) != required {
		s.adminUsage(p, name+" <player> <acceslevel|membership> [mask]")
		return
	}
	q := s.adminNamed(p, args[0])
	if q == nil {
		return
	}
	kind, maxMask := byte(1), 3
	switch args[1] {
	case "acceslevel", "accesslevel":
	case "membership":
		kind, maxMask = 2, 1
	default:
		s.adminUsage(p, name+" <player> <acceslevel|membership> [mask]")
		return
	}
	mask := 0
	if !revoke {
		n, err := strconv.Atoi(args[2])
		if err != nil || n < 0 || n > maxMask {
			s.tell(p, "Invalid account mask.")
			return
		}
		mask = n
	}
	w := wire.Packet(0x05)
	w.C(kind)
	w.S(p.Name)
	w.S(q.conn.account.name)
	w.S(q.Name)
	w.C(byte(mask))
	if s.login == nil || !s.login.send(w) {
		s.tell(p, "Login server is unavailable.")
	}
}

// adminConfigName maps Java configuration field names onto live Go settings.
func adminConfigName(group, property string) string {
	aliases := map[string]string{"custom.ENABLE_SIMPLE_2NDCLASS": "AION_SIMPLE_2NDCLASS", "custom.ENABLE_HTML_WELCOME": "AION_HTML_WELCOME", "custom.CROSS_FACTION_BINDING": "AION_CROSS_FACTION_BINDING", "gs.NAME": "SERVER_NAME", "gs.SERVER_NAME": "SERVER_NAME", "gs.MAX_PLAYERS": "MAX_PLAYERS", "gs.CHARACTER_NAME_PATTERN": "NAME_PATTERN"}
	key := strings.ToLower(group) + "." + strings.ToUpper(property)
	if name := aliases[key]; name != "" {
		return name
	}
	return strings.ToUpper(property)
}
func (s *Server) adminConfigure(p *player, args []string) {
	if len(args) < 3 || len(args) > 4 || args[0] == "show" && len(args) != 3 || args[0] == "set" && len(args) != 4 || !slices.Contains([]string{"set", "show"}, args[0]) {
		s.adminUsage(p, "configure <set|show> <admin|gs|custom> <property> [newvalue]")
		return
	}
	name := adminConfigName(args[1], args[2])
	if args[1] == "admin" {
		command := strings.TrimPrefix(strings.ToLower(args[2]), "command_")
		if actual := adminPermissionAliases[command]; actual != "" {
			command = actual
		}
		if !knownAdminCommand(command) {
			s.tell(p, "Unknown admin command property.")
			return
		}
		if args[0] == "show" {
			s.tell(p, fmt.Sprintf("Current value is %d", s.adminCommandLevel(command)))
			return
		}
		n, err := strconv.Atoi(args[3])
		if err != nil || n < 0 || n > 3 {
			s.tell(p, "Access level must be 0-3.")
			return
		}
		if s.adminCommands == nil {
			s.adminCommands = map[string]byte{}
		}
		s.adminCommands[command] = byte(n)
		s.tell(p, "Property changed for this process.")
		return
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	value := ""
	switch name {
	case "AION_SIMPLE_2NDCLASS":
		value = strconv.FormatBool(s.config.SimpleSecondClass)
	case "AION_HTML_WELCOME":
		value = strconv.FormatBool(s.config.HTMLWelcome)
	case "AION_CROSS_FACTION_BINDING":
		value = strconv.FormatBool(s.config.CrossFactionBinding)
	case "SERVER_NAME":
		value = s.config.Name
	case "MAX_PLAYERS":
		value = strconv.Itoa(int(s.config.MaxPlayers))
	case "NAME_PATTERN":
		value = s.config.NamePattern
	default:
		s.tell(p, "Unknown Go configuration property.")
		return
	}
	if args[0] == "show" {
		s.tell(p, "Current value is "+value)
		return
	}
	next := args[3]
	switch name {
	case "AION_SIMPLE_2NDCLASS", "AION_HTML_WELCOME", "AION_CROSS_FACTION_BINDING":
		enabled, err := strconv.ParseBool(next)
		if err != nil {
			s.tell(p, "Expected true or false.")
			return
		}
		switch name {
		case "AION_SIMPLE_2NDCLASS":
			s.config.SimpleSecondClass = enabled
		case "AION_HTML_WELCOME":
			s.config.HTMLWelcome = enabled
		case "AION_CROSS_FACTION_BINDING":
			s.config.CrossFactionBinding = enabled
		}
	case "SERVER_NAME":
		s.config.Name = next
	case "MAX_PLAYERS":
		n, err := strconv.ParseInt(next, 10, 32)
		if err != nil || n < 1 {
			s.tell(p, "Expected a positive player limit.")
			return
		}
		s.config.MaxPlayers = int32(n)
	case "NAME_PATTERN":
		compiled, err := compileNamePattern(next)
		if err != nil {
			s.tell(p, "Invalid name pattern.")
			return
		}
		s.names = compiled
		s.config.NamePattern = next
	}
	s.tell(p, "Property changed for this process.")
}

var adminPermissionAliases = map[string]string{"advsendfakeserverpacket": "send", "announce_faction": "announcefaction", "deletespawn": "delete", "questcommand": "quest", "reloadspawns": "reload_spawn", "savespawndata": "save_spawn", "sendfakeserverpacket": "fsc", "sendrawpacket": "raw", "spawnnpc": "spawn", "system": "sys", "resurrect": "rez", "prison": "sprison", "setap": "set", "setclass": "set", "setexp": "set", "setlevel": "set", "settitle": "set"}

func knownAdminCommand(name string) bool {
	if extendedAdminCommands[name] != nil {
		return true
	}
	return slices.Contains([]string{"add", "kinah", "goto", "moveto", "heal", "kill", "rez", "set", "morph", "siege", "announce", "petition", "gag", "ungag", "sprison", "rprison", "kick", "ban", "unban", "banip", "unbanip"}, name)
}
func (s *Server) adminCommandLevel(name string) byte {
	if level, ok := s.adminCommands[name]; ok {
		return level
	}
	return 3
}

func (s *Server) adminAccessResponse(r *wire.Reader) {
	kind, success, admin, target, mask, accountID := r.C(), r.C() == 1, r.S(), r.S(), r.C(), r.D()
	if r.Err != nil || kind != 1 && kind != 2 {
		return
	}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	if success {
		s.mu.Lock()
		if c := s.accounts[accountID]; c != nil {
			if kind == 1 {
				c.account.accessLevel = mask
			} else {
				c.account.membership = mask
			}
		}
		s.mu.Unlock()
	}
	if p := s.onlineNamed(admin); p != nil {
		if success {
			s.tell(p, fmt.Sprintf("Updated %s account mask to %d.", target, mask))
		} else {
			s.tell(p, "Account update failed.")
		}
	}
}
