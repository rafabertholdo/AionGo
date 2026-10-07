package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestAscensionPreparation(t *testing.T) {
	for _, class := range []string{"WARRIOR", "SCOUT", "MAGE", "PRIEST"} {
		plan, err := prepareAscension("ELYOS", class, "au_server_gs")
		if err != nil || plan.firstQuest != 1000 || plan.quest != 1006 || plan.variable != 3 || plan.world != 210010000 || plan.npc != "Pernos" {
			t.Fatalf("Elyos %s: %+v, %v", class, plan, err)
		}
	}
	plan, err := prepareAscension("ASMODIANS", "SCOUT", "au_server_gs_java")
	if err != nil || plan.firstQuest != 2000 || plan.quest != 2008 || plan.variable != 4 || plan.world != 220010000 || plan.npc != "Munin" {
		t.Fatalf("Asmodian: %+v, %v", plan, err)
	}
	for _, input := range [][3]string{{"ASMODIANS", "SCOUT", "au_server_gs"}, {"ELYOS", "GLADIATOR", "au_server_gs"}, {"UNKNOWN", "MAGE", "au_server_gs"}} {
		if _, err := prepareAscension(input[0], input[1], input[2]); err == nil {
			t.Errorf("accepted unsupported preparation: %v", input)
		}
	}
}

func TestNTCPreparation(t *testing.T) {
	plan, err := prepareNTC("ELYOS", "WARRIOR", "TEMPLAR")
	if err != nil || plan.class != "TEMPLAR" || plan.world != 400010000 || plan.x != 2890 || len(plan.quests) != 28 ||
		plan.siegeQuest != 3702 || plan.siegeItem != 182202179 {
		t.Fatalf("Elyos warrior: %+v, %v", plan, err)
	}
	if got := plan.gear[len(plan.gear)-2:]; got[0] != (equipItem{1, 100000975}) || got[1] != (equipItem{2, 115001027}) {
		t.Fatalf("Templar weapons: %v", got)
	}
	plan, err = prepareNTC("ASMODIANS", "SPIRIT_MASTER", "")
	if err != nil || plan.class != "SPIRIT_MASTER" || plan.x != 876.7211 || plan.quests[len(plan.quests)-1] != 2947 ||
		plan.siegeQuest != 4702 || plan.siegeItem != 182205676 {
		t.Fatalf("Asmodian spiritmaster: %+v, %v", plan, err)
	}
	if plan.gear[0] != (equipItem{1 << 3, 110101127}) || plan.gear[5] != (equipItem{1, 100600815}) {
		t.Fatalf("Archon Recruit's gear: %v", plan.gear)
	}
	for _, input := range [][3]string{{"ELYOS", "WARRIOR", ""}, {"ELYOS", "WARRIOR", "RANGER"}, {"UNKNOWN", "CHANTER", ""}} {
		if _, err := prepareNTC(input[0], input[1], input[2]); err == nil {
			t.Errorf("accepted %v", input)
		}
	}
}

func TestDungeonPreparation(t *testing.T) {
	tests := []struct {
		action, race, class, choice string
		level, world                int
		x, y, z                     float64
	}{
		{"fire_temple", "ELYOS", "WARRIOR", "TEMPLAR", 30, 210020000, 1343.28, 350.96, 348.67},
		{"fire_temple", "ASMODIANS", "RANGER", "", 27, 220020000, 1592.178, 977.2572, 140.75},
		{"aether_lab", "ELYOS", "CLERIC", "", 41, 210040000, 233.95, 533.53, 158.75},
		{"aether_lab", "ASMODIANS", "MAGE", "SORCERER", 41, 210040000, 233.95, 533.53, 158.75},
	}
	for _, tt := range tests {
		plan, level, err := prepareDungeon(tt.action, tt.race, tt.class, tt.choice)
		if err != nil || level != tt.level || plan.world != tt.world || plan.x != tt.x || plan.y != tt.y || plan.z != tt.z ||
			len(plan.quests) != 0 || len(plan.gear) != 0 || plan.siegeItem != 0 {
			t.Errorf("%s %s: plan=%+v level=%d err=%v", tt.action, tt.race, plan, level, err)
		}
	}
	for _, tt := range [][4]string{
		{"fire_temple", "ELYOS", "WARRIOR", ""},
		{"aether_lab", "ELYOS", "WARRIOR", "RANGER"},
		{"fire_temple", "UNKNOWN", "CLERIC", ""},
		{"unknown", "ELYOS", "CLERIC", ""},
	} {
		if _, _, err := prepareDungeon(tt[0], tt[1], tt[2], tt[3]); err == nil {
			t.Errorf("accepted %v", tt)
		}
	}
}

// The preset's item ids must stay level-25 gear of the server's item data.
func TestNTCGearData(t *testing.T) {
	dir := os.Getenv("AION_DATA")
	if dir == "" {
		t.Skip("set AION_DATA to the static data folder")
	}
	data, err := os.ReadFile(dir + "/items/item_templates.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, race := range []string{"ELYOS", "ASMODIANS"} {
		for class := range ntcClassGear {
			plan, err := prepareNTC(race, class, "")
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range plan.gear {
				if !bytes.Contains(data, fmt.Appendf(nil, `<item_template id="%d" level="25"`, item.id)) {
					t.Errorf("%s %s: item %d is not level-25 gear", race, class, item.id)
				}
			}
		}
	}
}

func TestSkillsAt(t *testing.T) {
	a := &assets{skills: map[string][]skillLearn{
		"ALL":     {{1801, 1, 1, "ALL"}},
		"WARRIOR": {{169, 9, 9, "ALL"}},
		"TEMPLAR": {{200, 1, 10, "ALL"}, {200, 3, 20, "ALL"}, {200, 4, 26, "ALL"}, {300, 1, 12, "ASMODIANS"}},
		"RANGER":  {{400, 1, 10, "ALL"}},
	}}
	want := map[int32]int32{1801: 1, 169: 9, 200: 3}
	if got := a.skillsAt("TEMPLAR", "ELYOS", 25); !maps.Equal(got, want) {
		t.Fatalf("skillsAt = %v, want %v", got, want)
	}
}

func TestCharacterLevelBounds(t *testing.T) {
	a := &assets{exp: []int64{0, 650, 2567}}
	for level, want := range map[int]int64{1: 0, 2: 650, 3: 2567} {
		if got, err := a.experienceForLevel(level); err != nil || got != want {
			t.Errorf("level %d = %d, %v", level, got, err)
		}
	}
	for _, level := range []int{-1, 0, 4, 51} {
		if _, err := a.experienceForLevel(level); err == nil {
			t.Errorf("accepted level %d", level)
		}
	}
	if _, err := (&assets{}).experienceForLevel(9); err == nil {
		t.Error("accepted missing experience table")
	}
}

func TestCharacterGrantAmountBounds(t *testing.T) {
	for _, raw := range []string{"1", " 25 ", "2147483647"} {
		maximum := int64(2147483647)
		if raw == "2147483647" {
			if got, err := positiveAmount(raw, maximum); err != nil || got != maximum {
				t.Errorf("positiveAmount(%q) = %d, %v", raw, got, err)
			}
			continue
		}
		if got, err := positiveAmount(raw, maximum); err != nil || got < 1 {
			t.Errorf("positiveAmount(%q) = %d, %v", raw, got, err)
		}
	}
	for _, raw := range []string{"", "0", "-1", "2147483648", "not a number"} {
		if _, err := positiveAmount(raw, 2147483647); err == nil {
			t.Errorf("accepted invalid amount %q", raw)
		}
	}
}

func TestItemSearchCategories(t *testing.T) {
	a := &assets{items: map[int32]itemInfo{
		140000001: {Name: "Healing Light", Icon: "icon_item_stigma01"},
		140000002: {Name: "Stigma Two", Icon: "icon_item_stigma01"},
		100600001: {Name: "Mage Spellbook", Type: "Spellbook", Icon: "icon_item_book_u01"},
		169500001: {Name: "Boost HP", Icon: "icon_item_skillbook_02"},
		100000001: {Name: "Bronze Sword", Type: "Sword", Icon: "icon_item_sword"},
	}}
	stigmas := a.searchItems("", "stigma")
	if len(stigmas) != 2 || stigmas[0].Type != "Stigma" {
		t.Fatalf("stigma search = %+v", stigmas)
	}
	books := a.searchItems("", "spellbook")
	if len(books) != 2 {
		t.Fatalf("spellbook search = %+v", books)
	}
	if books[0].Name != "Boost HP" || books[0].Type != "Skill book" {
		t.Errorf("skill book result = %+v", books[0])
	}
	if books[1].Name != "Mage Spellbook" || books[1].Type != "Spellbook" {
		t.Errorf("spellbook result = %+v", books[1])
	}
	if got := a.searchItems("Bronze", "all"); len(got) != 1 || got[0].ID != 100000001 {
		t.Errorf("name search = %+v", got)
	}
}

func TestCharacterEditAuthorizationAndCSRF(t *testing.T) {
	p := &panel{}
	admin := &user{ID: 1, AccessLevel: adminLevel}
	for _, test := range []struct {
		u     *user
		token string
	}{{nil, ""}, {&user{ID: 1, AccessLevel: gmLevel}, characterToken(admin)}, {admin, ""}, {admin, characterToken(&user{ID: 2})}} {
		form := url.Values{"token": {test.token}}
		request := httptest.NewRequest("POST", "/admin/character", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		p.editCharacter(test.u, response, request)
		if response.Code != 403 {
			t.Errorf("status = %d", response.Code)
		}
	}
}

func TestCharacterAdminControls(t *testing.T) {
	for _, level := range []int{gmLevel, adminLevel} {
		var out bytes.Buffer
		v := characterView{Found: true, Name: "Hero", User: &user{AccessLevel: level}, Online: true}
		if err := characterPage.Execute(&out, v); err != nil {
			t.Fatal(err)
		}
		has := strings.Contains(out.String(), "Prepare for Ascension trial")
		if has != (level == adminLevel) {
			t.Errorf("controls visible for access %d: %v", level, has)
		}
		if has && !strings.Contains(out.String(), "<button disabled>") {
			t.Error("online character controls enabled")
		}
		if strings.Contains(out.String(), "Find an item to give") != (level == adminLevel) ||
			strings.Contains(out.String(), "Give Kinah") != (level == adminLevel) {
			t.Errorf("grant controls visible for access %d", level)
		}
	}
}

// This integration test creates its own schema and never writes a game character.
func TestCharacterEditDatabase(t *testing.T) {
	host := os.Getenv("AION_TEST_DB")
	if host == "" {
		t.Skip("set AION_TEST_DB for MariaDB integration")
	}
	config := mysql.NewConfig()
	config.Net, config.Addr, config.User, config.Passwd = "tcp", host+":3306", "root", "aion"
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema := fmt.Sprintf("panel_edit_test_%d", time.Now().UnixNano())
	if _, err := db.Exec("CREATE DATABASE " + schema); err != nil {
		t.Fatal(err)
	}
	defer db.Exec("DROP DATABASE " + schema)
	for _, statement := range []string{
		`CREATE TABLE ` + schema + `.players (id INT PRIMARY KEY, name VARCHAR(50), race VARCHAR(20), player_class VARCHAR(20), online BOOL, cube_size INT, deletion_date DATETIME NULL, exp BIGINT, recoverexp BIGINT, world_id INT, x FLOAT, y FLOAT, z FLOAT, heading INT) ENGINE=InnoDB`,
		`CREATE TABLE ` + schema + `.player_quests (player_id INT, quest_id INT, status VARCHAR(20), quest_vars INT, complete_count INT, PRIMARY KEY(player_id,quest_id)) ENGINE=InnoDB`,
		`CREATE TABLE ` + schema + `.inventory (itemUniqueId INT PRIMARY KEY, itemId INT, itemCount BIGINT, itemColor INT, itemOwner INT, isEquiped BOOL, isSoulBound BOOL, slot INT, itemLocation INT NULL, enchant TINYINT, itemSkin INT, fusionedItem INT) ENGINE=InnoDB`,
		`INSERT INTO ` + schema + `.players VALUES (1,'Hero','ELYOS','SCOUT',0,0,NULL,0,42,210010000,1,2,3,4)`,
		`INSERT INTO ` + schema + `.player_quests VALUES (1,1100,'START',3,0)`,
		`INSERT INTO ` + schema + `.player_quests VALUES (1,1006,'START',5,0)`,
		`INSERT INTO ` + schema + `.inventory VALUES (10,182400001,100,0,1,0,0,65535,0,0,0,0)`,
		`INSERT INTO ` + schema + `.inventory VALUES (11,3001,8,0,1,0,0,0,0,0,0,0)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	p := &panel{db: db, gsDB: schema, log: slog.New(slog.NewTextHandler(io.Discard, nil)), assets: &assets{
		items: map[int32]itemInfo{
			3001: {Name: "Potion", MaxStack: 10},
			3002: {Name: "Test Sword", MaxStack: 1},
			3003: {Name: "Test Shield", MaxStack: 1},
			3004: {Name: "Test Helmet", MaxStack: 1},
		},
		sets: []gearSetInfo{{Key: "test-set", Name: "Test Set", ItemIDs: []int32{3002, 3003}}},
	}}
	request := httptest.NewRequest("POST", "/admin/character", nil)
	if err := p.updateCharacter(request, "Hero", "ascension", 140329); err != nil {
		t.Fatal(err)
	}
	var exp, recover int64
	var world int
	var x, y, z float64
	if err := db.QueryRow(`SELECT exp,recoverexp,world_id,x,y,z FROM `+schema+`.players WHERE id=1`).Scan(&exp, &recover, &world, &x, &y, &z); err != nil {
		t.Fatal(err)
	}
	if exp != 140329 || recover != 0 || world != 210010000 || x != 242 || y != 1638 || z != 100 {
		t.Fatalf("character: %d %d %d %f %f %f", exp, recover, world, x, y, z)
	}
	for quest := 1000; quest <= 1006; quest++ {
		var status string
		var variable, count int
		if err := db.QueryRow(`SELECT status,quest_vars,complete_count FROM `+schema+`.player_quests WHERE player_id=1 AND quest_id=?`, quest).Scan(&status, &variable, &count); err != nil {
			t.Fatal(err)
		}
		if quest == 1006 {
			if status != "START" || variable != 3 || count != 0 {
				t.Fatalf("Ascension: %s %d %d", status, variable, count)
			}
		} else if status != "COMPLETE" || count != 1 {
			t.Fatalf("quest %d: %s %d", quest, status, count)
		}
	}
	var unchanged int
	if err := db.QueryRow(`SELECT quest_vars FROM ` + schema + `.player_quests WHERE quest_id=1100`).Scan(&unchanged); err != nil || unchanged != 3 {
		t.Fatal("unrelated quest changed", err)
	}
	if err := p.updateCharacter(request, "Hero", "ascension", 140329); err != nil {
		t.Fatal("repeat preset", err)
	}
	if _, err := db.Exec(`UPDATE ` + schema + `.players SET online=1 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if err := p.updateCharacter(request, "Hero", "level", 650); err == nil {
		t.Fatal("edited online character")
	}
	if _, err := db.Exec(`UPDATE ` + schema + `.players SET online=0 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if err := p.updateCharacter(request, "Hero", "level", 650); err != nil {
		t.Fatal(err)
	}
	// Force a later write to fail: earlier preset quest writes must roll back.
	if _, err := db.Exec(`ALTER TABLE ` + schema + `.players DROP COLUMN heading`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE ` + schema + `.player_quests SET status='START', complete_count=0 WHERE quest_id=1001`); err != nil {
		t.Fatal(err)
	}
	if err := p.updateCharacter(request, "Hero", "ascension", 140329); err == nil {
		t.Fatal("accepted broken schema")
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM ` + schema + `.player_quests WHERE quest_id=1001`).Scan(&status); err != nil || status != "START" {
		t.Fatal("partial quest writes survived rollback", status, err)
	}
}

func TestCharacterGrantsDatabase(t *testing.T) {
	host := os.Getenv("AION_TEST_DB")
	if host == "" {
		t.Skip("set AION_TEST_DB for MariaDB integration")
	}
	config := mysql.NewConfig()
	config.Net, config.Addr, config.User, config.Passwd = "tcp", host+":3306", "root", "aion"
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema := fmt.Sprintf("panel_grant_test_%d", time.Now().UnixNano())
	if _, err := db.Exec("CREATE DATABASE " + schema); err != nil {
		t.Fatal(err)
	}
	defer db.Exec("DROP DATABASE " + schema)
	for _, statement := range []string{
		`CREATE TABLE ` + schema + `.players (id INT PRIMARY KEY, name VARCHAR(50), race VARCHAR(20), player_class VARCHAR(20), online BOOL, cube_size INT, deletion_date DATETIME NULL) ENGINE=InnoDB`,
		`CREATE TABLE ` + schema + `.inventory (itemUniqueId INT PRIMARY KEY, itemId INT, itemCount BIGINT, itemColor INT, itemOwner INT, isEquiped BOOL, isSoulBound BOOL, slot INT, itemLocation INT NULL, enchant TINYINT, itemSkin INT, fusionedItem INT) ENGINE=InnoDB`,
		`INSERT INTO ` + schema + `.players VALUES (1,'Hero','ELYOS','SCOUT',0,0,NULL)`,
		`INSERT INTO ` + schema + `.inventory VALUES (10,182400001,100,0,1,0,0,65535,0,0,0,0)`,
		`INSERT INTO ` + schema + `.inventory VALUES (11,3001,8,0,1,0,0,0,0,0,0,0)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	p := &panel{db: db, gsDB: schema, log: slog.New(slog.NewTextHandler(io.Discard, nil)), assets: &assets{
		items: map[int32]itemInfo{
			3001: {Name: "Potion", MaxStack: 10},
			3002: {Name: "Test Sword", MaxStack: 1},
			3003: {Name: "Test Shield", MaxStack: 1},
			3004: {Name: "Test Helmet", MaxStack: 1},
		},
		sets: []gearSetInfo{{Key: "test-set", Name: "Test Set", ItemIDs: []int32{3002, 3003}}},
	}}
	request := httptest.NewRequest("POST", "/admin/character", nil)
	if err := p.grantKinah(request, "Hero", 25); err != nil {
		t.Fatal(err)
	}
	var kinah int64
	if err := db.QueryRow(`SELECT itemCount FROM ` + schema + `.inventory WHERE itemId=182400001`).Scan(&kinah); err != nil || kinah != 125 {
		t.Fatalf("kinah = %d, %v", kinah, err)
	}
	if _, err := p.grantItem(request, "Hero", 3001, 5); err != nil {
		t.Fatal(err)
	}
	var potionCount int64
	if err := db.QueryRow(`SELECT SUM(itemCount) FROM ` + schema + `.inventory WHERE itemOwner=1 AND itemId=3001`).Scan(&potionCount); err != nil || potionCount != 13 {
		t.Fatalf("potion count = %d, %v", potionCount, err)
	}
	if _, err := p.grantGearSet(request, "Hero", "test-set"); err != nil {
		t.Fatal(err)
	}
	var setItems int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + schema + `.inventory WHERE itemOwner=1 AND itemId IN (3002,3003)`).Scan(&setItems); err != nil || setItems != 2 {
		t.Fatalf("set item count = %d, %v", setItems, err)
	}
	if already, err := p.expandCube(request, "Hero"); err != nil || already {
		t.Fatalf("expand cube = already %v, %v", already, err)
	}
	var cubeSize int
	if err := db.QueryRow(`SELECT cube_size FROM ` + schema + `.players WHERE id=1`).Scan(&cubeSize); err != nil || cubeSize != maxCubeExpansion {
		t.Fatalf("cube size = %d, %v", cubeSize, err)
	}
	if already, err := p.expandCube(request, "Hero"); err != nil || !already {
		t.Fatalf("repeat cube expansion = already %v, %v", already, err)
	}
	if _, err := db.Exec(`UPDATE ` + schema + `.players SET online=1 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := p.grantItem(request, "Hero", 3004, 1); err == nil {
		t.Fatal("granted an item to an online character")
	}
}

// This integration test creates its own schema and never writes a game character.
func TestNTCDatabase(t *testing.T) {
	host := os.Getenv("AION_TEST_DB")
	if host == "" {
		t.Skip("set AION_TEST_DB for MariaDB integration")
	}
	config := mysql.NewConfig()
	config.Net, config.Addr, config.User, config.Passwd = "tcp", host+":3306", "root", "aion"
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema := fmt.Sprintf("panel_ntc_test_%d", time.Now().UnixNano())
	if _, err := db.Exec("CREATE DATABASE " + schema); err != nil {
		t.Fatal(err)
	}
	defer db.Exec("DROP DATABASE " + schema)
	for _, statement := range []string{
		`CREATE TABLE ` + schema + `.players (id INT PRIMARY KEY, name VARCHAR(50), race VARCHAR(20), player_class VARCHAR(20), online BOOL, cube_size INT, deletion_date DATETIME NULL, exp BIGINT, recoverexp BIGINT, world_id INT, x FLOAT, y FLOAT, z FLOAT, heading INT) ENGINE=InnoDB`,
		`CREATE TABLE ` + schema + `.player_quests (player_id INT, quest_id INT, status VARCHAR(20), quest_vars INT, complete_count INT, PRIMARY KEY(player_id,quest_id)) ENGINE=InnoDB`,
		`CREATE TABLE ` + schema + `.player_skills (player_id INT, skillId INT, skillLevel INT, PRIMARY KEY(player_id,skillId)) ENGINE=InnoDB`,
		`CREATE TABLE ` + schema + `.inventory (itemUniqueId INT PRIMARY KEY, itemId INT, itemCount BIGINT, itemColor INT, itemOwner INT, isEquiped BOOL, isSoulBound BOOL, slot INT, itemLocation INT NULL, enchant TINYINT, itemSkin INT, fusionedItem INT) ENGINE=InnoDB`,
		`INSERT INTO ` + schema + `.players VALUES (1,'Hero','ELYOS','PRIEST',0,0,NULL,0,42,210010000,1,2,3,4)`,
		`INSERT INTO ` + schema + `.player_quests VALUES (1,1031,'START',2,0)`,
		`INSERT INTO ` + schema + `.player_skills VALUES (1,169,12)`,
		`INSERT INTO ` + schema + `.inventory VALUES (20,100100001,1,0,1,1,0,1,0,0,0,0)`, // old mace
		`INSERT INTO ` + schema + `.inventory VALUES (21,115000001,1,0,1,1,0,2,0,0,0,0)`, // old shield
		`INSERT INTO ` + schema + `.inventory VALUES (22,125000001,1,0,1,1,0,4,0,0,0,0)`, // helmet stays
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	exp := make([]int64, 50)
	for i := range exp {
		exp[i] = int64(i) * 1000
	}
	p := &panel{db: db, gsDB: schema, log: slog.New(slog.NewTextHandler(io.Discard, nil)), assets: &assets{
		exp:    exp,
		skills: map[string][]skillLearn{"CHANTER": {{169, 5, 20, "ALL"}, {500, 2, 25, "ALL"}, {501, 1, 30, "ALL"}}},
	}}
	request := httptest.NewRequest("POST", "/admin/character", nil)
	if _, err := p.prepareNTC(request, "Hero", ""); err == nil {
		t.Fatal("prepared a base class without a choice")
	}
	for range 2 { // repeating the preset keeps one set of gear
		if class, err := p.prepareNTC(request, "Hero", "CHANTER"); err != nil || class != "CHANTER" {
			t.Fatal(class, err)
		}
	}
	var class string
	var level int64
	var world int
	if err := db.QueryRow(`SELECT player_class,exp,world_id FROM `+schema+`.players WHERE id=1`).Scan(&class, &level, &world); err != nil {
		t.Fatal(err)
	}
	if class != "CHANTER" || level != 24000 || world != 400010000 {
		t.Fatalf("character: %s %d %d", class, level, world)
	}
	var completed int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + schema + `.player_quests WHERE status='COMPLETE' AND complete_count=1`).Scan(&completed); err != nil || completed != 28 {
		t.Fatal("completed quests", completed, err)
	}
	var siegeStatus string
	var siegeCount int
	if err := db.QueryRow(`SELECT status,complete_count FROM `+schema+`.player_quests WHERE player_id=1 AND quest_id=3702`).Scan(&siegeStatus, &siegeCount); err != nil || siegeStatus != "START" || siegeCount != 0 {
		t.Fatal("siege quest", siegeStatus, siegeCount, err)
	}
	skills := map[int32]int32{}
	rows, err := db.Query(`SELECT skillId, skillLevel FROM ` + schema + `.player_skills`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, level int32
		if err := rows.Scan(&id, &level); err != nil {
			t.Fatal(err)
		}
		skills[id] = level
	}
	if !maps.Equal(skills, map[int32]int32{169: 12, 500: 2}) {
		t.Fatalf("skills: %v", skills)
	}
	equipped := map[int64]int32{}
	var cube []int32
	rows, err = db.Query(`SELECT itemId, isEquiped, slot FROM ` + schema + `.inventory`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int32
		var isEquipped bool
		var slot int64
		if err := rows.Scan(&id, &isEquipped, &slot); err != nil {
			t.Fatal(err)
		}
		if isEquipped {
			equipped[slot] = id
		} else {
			cube = append(cube, id)
		}
	}
	if len(equipped) != 7 || equipped[1] != 101500760 || equipped[4] != 125000001 || equipped[8] != 110501041 {
		t.Fatalf("equipped: %v", equipped)
	}
	if slices.Sort(cube); !slices.Equal(cube, []int32{100100001, 115000001, 182202179}) {
		t.Fatalf("cube: %v", cube)
	}
}
