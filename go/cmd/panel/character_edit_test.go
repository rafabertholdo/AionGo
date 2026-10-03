package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
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
