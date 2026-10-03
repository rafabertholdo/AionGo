package game

import (
	"aionlightning/commands"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

type memoryAdmin struct {
	fail          bool
	titles        [][2]int32
	drops         []store.Drop
	bookmarks     map[int32][]store.Bookmark
	announcements []store.AutoAnnouncement
}

func (m *memoryAdmin) err() error {
	if m.fail {
		return errors.New("storage unavailable")
	}
	return nil
}
func (m *memoryAdmin) AddTitle(p, id int32) error {
	if e := m.err(); e != nil {
		return e
	}
	m.titles = append(m.titles, [2]int32{p, id})
	return nil
}
func (m *memoryAdmin) AddDrop(_ int32, d store.Drop) error {
	if e := m.err(); e != nil {
		return e
	}
	m.drops = append(m.drops, d)
	return nil
}
func (m *memoryAdmin) Bookmarks(p int32) ([]store.Bookmark, error) {
	return slices.Clone(m.bookmarks[p]), m.err()
}
func (m *memoryAdmin) AddBookmark(p int32, b store.Bookmark) error {
	if e := m.err(); e != nil {
		return e
	}
	m.bookmarks[p] = append(m.bookmarks[p], b)
	return nil
}
func (m *memoryAdmin) DeleteBookmark(p int32, n string) error {
	if e := m.err(); e != nil {
		return e
	}
	m.bookmarks[p] = slices.DeleteFunc(m.bookmarks[p], func(b store.Bookmark) bool { return b.Name == n })
	return nil
}
func (m *memoryAdmin) SaveAnnouncement(a store.AutoAnnouncement) error {
	if e := m.err(); e != nil {
		return e
	}
	a.ID = int32(len(m.announcements) + 1)
	m.announcements = append(m.announcements, a)
	return nil
}
func (m *memoryAdmin) DeleteAnnouncement(id int32) error {
	if e := m.err(); e != nil {
		return e
	}
	m.announcements = slices.DeleteFunc(m.announcements, func(a store.AutoAnnouncement) bool { return a.ID == id })
	return nil
}
func (m *memoryAdmin) AutoAnnouncements() ([]store.AutoAnnouncement, error) {
	return slices.Clone(m.announcements), m.err()
}
func (m *memoryAdmin) LegionIDByName(string) (int32, error) { return 0, m.err() }

func adminFixture(t *testing.T) (*Server, *player, *player, *memoryAdmin) {
	t.Helper()
	// Commands mutate static maps, so give each fixture its own map containers.
	original := staticDataOrSkip(t)
	d := *original
	d.Spawns = map[int32][]*data.SpawnGroup{}
	d.SpawnsByNPC = map[int32][]*data.SpawnGroup{}
	s := testServer(&d)
	a, b, _, _ := twoPlayers(t, s)
	a.conn.account.accessLevel = 3
	a.conn.account.id = 1
	b.conn.account.id = 2
	a.conn.account.name = "admin"
	b.conn.account.name = "bob"
	s.accounts = map[int32]*conn{1: a.conn, 2: b.conn}
	s.weathers = map[int32]weatherState{}
	m := &memoryAdmin{bookmarks: map[int32][]store.Bookmark{}}
	s.adminDB = m
	t.Cleanup(func() {
		s.visMu.Lock()
		defer s.visMu.Unlock()
		for _, ts := range [][]*task{s.announcementTasks, s.adminShutdownTasks} {
			for _, task := range ts {
				task.cancel()
			}
		}
		for _, p := range s.spawned {
			p.restore.cancel()
		}
		for _, o := range s.byID {
			s.deleteAdminObject(o)
		}
	})
	return s, a, b, m
}
func command(s *Server, p *player, text string) {
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.adminCommand(p, "//"+text)
}

func TestJavaAdminCommandCatalog(t *testing.T) {
	names := []string{}
	for _, entry := range commands.Admin {
		names = append(names, entry.Name)
	}
	for _, name := range names {
		if !knownAdminCommand(name) {
			t.Errorf("Java command %s has no Go handler", name)
		}
	}

	if len(names) != 61 {
		t.Fatalf("catalog has %d commands", len(names))
	}
}
func TestAdminPermissionOverrides(t *testing.T) {
	s, a, b, _ := adminFixture(t)
	a.conn.account.accessLevel = 2
	command(s, a, "invul")
	if a.adminInvulnerable {
		t.Fatal("level 2 bypassed default Java access level 3")
	}
	a.conn.account.accessLevel = 3
	command(s, a, "configure set admin COMMAND_INVUL 1")
	b.conn.account.accessLevel = 1
	command(s, b, "invul")
	if !b.adminInvulnerable {
		t.Fatal("configured permission was ignored")
	}
	command(s, b, "configure set admin COMMAND_KILL 0")
	if s.adminCommandLevel("kill") != 3 {
		t.Fatal("low access changed permissions")
	}
}
func TestAdminInvulnerabilityAndInvisibility(t *testing.T) {
	s, a, b, _ := adminFixture(t)
	hp := a.life.HP
	command(s, a, "invul")
	s.reducePlayerHP(a, hp*2, b)
	if a.life.HP != hp || a.dead {
		t.Fatal("direct damage bypassed invulnerability")
	}
	command(s, a, "invul")
	s.reducePlayerHP(a, 1, b)
	if a.life.HP != hp-1 {
		t.Fatal("invulnerability did not toggle off")
	}
	command(s, a, "invis")
	if a.visualState != 3 {
		t.Fatal("not invisible")
	}
	command(s, a, "invis")
	if a.visualState != 0 {
		t.Fatal("not visible")
	}
}
func TestAdminMovementAndValidation(t *testing.T) {
	s, a, b, _ := adminFixture(t)
	command(s, a, "movetoplayer Bob")
	if a.X != b.X {
		t.Fatal("move to player failed")
	}
	command(s, a, "moveto 210010000 NaN 1000 130")
	if a.X != b.X {
		t.Fatal("NaN changed position")
	}
	command(s, a, "moveto 210010000 1008 1000 130")
	command(s, a, "movetome Bob")
	if b.X != 1008 {
		t.Fatal("move player to self failed")
	}
	command(s, a, "moveto 210010000 1010 1000 130")
	command(s, a, "moveplayertoplayer Bob Ann")
	if b.X != 1010 {
		t.Fatal("player to player failed")
	}
	command(s, a, "speed 100")
	if a.stats.current(data.Speed) != 12000 {
		t.Fatalf("speed=%d", a.stats.current(data.Speed))
	}
	command(s, a, "speed 201")
	if a.stats.current(data.Speed) != 12000 {
		t.Fatal("accepted invalid speed")
	}
}
func TestAdminSkillsTitlesAndAppearance(t *testing.T) {
	s, a, b, m := adminFixture(t)
	a.targetID = b.ID
	id := int32(0)
	for _, candidate := range sortedKeys(s.data.Skills) {
		if !b.hasSkill(candidate) {
			id = candidate
			break
		}
	}
	if id == 0 {
		t.Fatal("no unused skill")
	}
	prefix := "addskill " + strconv.Itoa(int(id))
	command(s, a, prefix+" 3")
	if !b.hasSkill(id) || b.skills[slices.IndexFunc(b.skills, func(k store.Skill) bool { return k.ID == id })].Level != 3 {
		t.Fatal("selected target didn't receive skill")
	}
	command(s, a, prefix+" 0")
	if b.skills[slices.IndexFunc(b.skills, func(k store.Skill) bool { return k.ID == id })].Level != 3 {
		t.Fatal("accepted zero level")
	}
	command(s, a, "addtitle 1")
	if !slices.Contains(b.titles, int32(1)) || len(m.titles) != 1 {
		t.Fatal("title not stored")
	}
	command(s, a, "addtitle 1")
	if len(m.titles) != 1 {
		t.Fatal("duplicate title persisted")
	}
	m.fail = true
	command(s, a, "addtitle 2")
	if slices.Contains(b.titles, int32(2)) {
		t.Fatal("failed title storage changed player")
	}
	m.fail = false
	old := *b.appearance
	command(s, a, "appearance hair 43")
	if b.appearance.Hair != 43 {
		t.Fatal("appearance target not changed")
	}
	command(s, a, "appearance hair 44")
	if b.appearance.Hair != 43 {
		t.Fatal("invalid hair accepted")
	}
	command(s, a, "appearance head_size 100")
	if b.appearance.HeadSize != 300 {
		t.Fatal("head-size offset differs from Java")
	}
	command(s, a, "appearance reset")
	if *b.appearance != old {
		t.Fatal("reset didn't restore appearance")
	}
	command(s, a, "set ap 50")
	if b.abyss.AP != 50 {
		t.Fatal("AP target not changed")
	}
	command(s, a, "set title 1")
	if b.TitleID != 1 {
		t.Fatal("title not selected")
	}
}
func TestAdminSpawnDeleteAndSave(t *testing.T) {
	s, a, _, _ := adminFixture(t)
	dir := t.TempDir()
	t.Setenv("AION_DATA", dir)
	command(s, a, "spawn 210133 norespawn")
	if len(s.byID) != 1 {
		t.Fatal("NPC not spawned")
	}
	var o *object
	for _, v := range s.byID {
		o = v
	}
	if !o.noRespawn {
		t.Fatal("norespawn ignored")
	}
	if groups := s.data.SpawnsByNPC[210133]; len(groups) != 1 || groups[0] != o.spawnGroup {
		t.Fatal("admin spawn is missing from the NPC map lookup")
	}
	command(s, a, "movetonpc 210133")
	if a.X != o.x {
		t.Fatal("NPC teleport failed")
	}
	command(s, a, "save_spawn")
	content, e := os.ReadFile(filepath.Join(dir, "spawns/new/210010000.xml"))
	if e != nil || !bytes.Contains(content, []byte(`npcid="210133"`)) {
		t.Fatalf("saved spawn=%s err=%v", content, e)
	}
	a.targetID = o.id
	command(s, a, "delete")
	if len(s.byID) != 0 || len(s.data.Spawns[a.WorldID]) != 0 || len(s.data.SpawnsByNPC[210133]) != 0 {
		t.Fatal("deleted spawn retained")
	}
	command(s, a, "spawn -1")
	if len(s.byID) != 0 {
		t.Fatal("invalid template spawned")
	}
}
func TestAdminBookmarksDropsAndAnnouncements(t *testing.T) {
	s, a, b, m := adminFixture(t)
	command(s, a, "bk add home")
	command(s, b, "bk list")
	if len(m.bookmarks[b.ID]) != 0 {
		t.Fatal("bookmarks leaked across players")
	}
	command(s, a, "moveto 210010000 1020 1000 130")
	command(s, a, "bk tele home")
	if a.X != 1000 {
		t.Fatal("bookmark teleport failed")
	}
	command(s, a, "bk del home")
	if len(m.bookmarks[a.ID]) != 0 {
		t.Fatal("bookmark wasn't removed")
	}
	command(s, a, "adddrop 210133 160010002 1 2 50")
	if len(m.drops) != 1 || len(s.drops[210133]) != 1 {
		t.Fatal("drop not persisted and applied")
	}
	m.fail = true
	command(s, a, "adddrop 210133 160010002 1 2 50")
	if len(s.drops[210133]) != 1 {
		t.Fatal("failed storage applied drop")
	}
	m.fail = false
	command(s, a, "announcements add ALL ANNOUNCE 3600 hello world")
	if len(m.announcements) != 1 || m.announcements[0].Text != "hello world" || len(s.announcementTasks) != 1 {
		t.Fatal("announcement wasn't scheduled")
	}
	task := s.announcementTasks[0]
	command(s, a, "announcements delete 1")
	if !task.cancelled || len(m.announcements) != 0 {
		t.Fatal("deleted announcement kept running")
	}
}
func TestAdminWeatherAndConfiguration(t *testing.T) {
	s, a, _, _ := adminFixture(t)
	command(s, a, "weather poeta 8")
	if s.weathers[a.WorldID].code != 8 {
		t.Fatal("weather not applied")
	}
	command(s, a, "weather poeta 9")
	if s.weathers[a.WorldID].code != 8 {
		t.Fatal("invalid weather applied")
	}
	command(s, a, "weather reset")
	if len(s.weathers) == 0 {
		t.Fatal("weather should be regenerated for online players")
	}
	command(s, a, "configure set custom ENABLE_SIMPLE_2NDCLASS true")
	if !s.currentConfig().SimpleSecondClass {
		t.Fatal("Java config alias not applied")
	}
	command(s, a, "configure set gs MAX_PLAYERS 0")
	if s.currentConfig().MaxPlayers != 0 {
		t.Fatal("invalid configuration applied")
	}
	command(s, a, "configure set gs CHARACTER_NAME_PATTERN [A-Z]{3}")
	if !s.names.valid("ANN") || s.names.valid("Ann") {
		t.Fatal("name-pattern not recompiled")
	}
}
func TestAdminPacketValidationAndFileConfinement(t *testing.T) {
	w, e := adminCustomPacket("0x7c", "chdfes", []string{"4", "0x1234", "-1", "1.5", "2.5", "hello"})
	if e != nil || w.Data[0] != smPingResponse || w.Data[1] != 4 {
		t.Fatalf("packet=%v err=%v", w, e)
	}
	for _, tc := range []struct {
		format string
		values []string
	}{{"c", []string{"256"}}, {"f", []string{"NaN"}}, {"h", nil}, {"x", []string{"0"}}} {
		if _, e := adminCustomPacket("1", tc.format, tc.values); e == nil {
			t.Errorf("accepted invalid packet %v", tc)
		}
	}
	root := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret"), []byte("secret"), 0600)
	os.Symlink(filepath.Join(outside, "secret"), filepath.Join(root, "escape"))
	for _, name := range []string{"../secret", "/etc/passwd", "escape"} {
		if _, e := adminFile(root, name); e == nil {
			t.Errorf("allowed escape %s", name)
		}
	}
	os.WriteFile(filepath.Join(root, "valid"), []byte("hello"), 0600)
	content, e := adminFile(root, "valid")
	if e != nil || string(content) != "hello" {
		t.Fatal("valid file rejected")
	}
}
func TestAdminMappedRawAndHTMLPackets(t *testing.T) {
	s, a, _, _ := adminFixture(t)
	dir := t.TempDir()
	t.Setenv("AION_PACKETS", dir)
	t.Setenv("AION_DATA", dir)
	var packets []*wire.Writer
	a.conn.tap = func(w *wire.Writer) { packets = append(packets, w) }
	os.WriteFile(filepath.Join(dir, "test.xml"), []byte(`<packets><packet opcode="0x7c"><part type="c" value="4"/></packet></packets>`), 0600)
	command(s, a, "send test")
	if len(packets) != 1 || !bytes.Equal(packets[0].Data, []byte{smPingResponse, 4}) {
		t.Fatal("mapped packet differs")
	}
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("7c 00 00 04"), 0600)
	command(s, a, "raw test")
	if !bytes.Equal(packets[1].Data, []byte{smPingResponse, 4}) {
		t.Fatal("raw packet differs")
	}
	os.Mkdir(filepath.Join(dir, "HTML"), 0700)
	os.WriteFile(filepath.Join(dir, "HTML/welcome.xhtml"), []byte("welcome"), 0600)
	command(s, a, "html reload")
	if s.data.Welcome != "welcome" {
		t.Fatal("HTML not reloaded")
	}
}
func TestAdminAccessControlResponse(t *testing.T) {
	s, a, b, _ := adminFixture(t)
	w := wire.Packet(4)
	w.C(1)
	w.Bool(true)
	w.S(a.Name)
	w.S(b.Name)
	w.C(2)
	w.D(b.conn.account.id)
	s.adminAccessResponse(wire.NewReader(w.Data[1:]))
	if b.conn.account.accessLevel != 2 {
		t.Fatal("promotion response ignored")
	}
	w = wire.Packet(4)
	w.C(2)
	w.Bool(false)
	w.S(a.Name)
	w.S(b.Name)
	w.C(1)
	w.D(b.conn.account.id)
	s.adminAccessResponse(wire.NewReader(w.Data[1:]))
	if b.conn.account.membership != 0 {
		t.Fatal("failed membership response applied")
	}
}
func TestAdminShutdownCancellation(t *testing.T) {
	s, a, _, _ := adminFixture(t)
	command(s, a, "sys restart 3600 60")
	if len(s.adminShutdownTasks) != 2 {
		t.Fatal("countdown and shutdown not scheduled")
	}
	tasks := slices.Clone(s.adminShutdownTasks)
	command(s, a, "sys cancel")
	for _, task := range tasks {
		if !task.cancelled {
			t.Fatal("shutdown task retained")
		}
	}
}

func TestAdminInventoryDyeAndClass(t *testing.T) {
	s, a, b, _ := adminFixture(t)
	command(s, a, "add Bob 160010002 5")
	command(s, a, "remove Bob 160010002 2")
	if len(b.cube) != 1 || b.cube[0].Count != 3 {
		t.Fatal("inventory removal failed")
	}
	a.targetID = b.ID
	command(s, a, "dye true white")
	for _, item := range b.equipment {
		if s.template(item) != nil && !s.template(item).IsStigma() && item.Color != int32(-1) {
			t.Fatalf("item color=%x", item.Color)
		}
	}
	command(s, a, "dye no")
	for _, item := range b.equipment {
		if item.Color != 0 {
			t.Fatal("dye wasn't cleared")
		}
	}
	b.Class = "MAGE"
	b.level = 10
	command(s, a, "set class 7")
	if b.Class != "SORCERER" {
		t.Fatalf("class=%s", b.Class)
	}
}

func TestAdminQuestAndLegion(t *testing.T) {
	s, a, b, _ := adminFixture(t)
	a.targetID = b.ID
	b.quests = []store.Quest{{ID: 1000, Status: "START"}}
	command(s, a, "quest set 1000 REWARD 2")
	if b.quest(1000).Status != "REWARD" || b.quest(1000).Vars != 2 {
		t.Fatal("quest update failed")
	}
	command(s, a, "quest set 1000 INVALID 9")
	if b.quest(1000).Vars != 2 {
		t.Fatal("invalid quest status applied")
	}
	l := &legion{Legion: store.Legion{ID: 1, Name: "Guardians", Level: 1}}
	s.legions = map[int32]*legion{1: l}
	command(s, a, "legion setlevel Guardians 3")
	if l.Level != 3 {
		t.Fatal("legion level wasn't updated")
	}
	command(s, a, "legion setpoints Guardians 100")
	if l.Contribution != 100 {
		t.Fatal("legion points weren't updated")
	}
	command(s, a, "legion setlevel Guardians 6")
	if l.Level != 3 {
		t.Fatal("invalid legion level applied")
	}
}
