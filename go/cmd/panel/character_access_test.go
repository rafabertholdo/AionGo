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

func TestCharacterAccessControls(t *testing.T) {
	for _, test := range []struct {
		name            string
		viewer          *user
		accountID       int32
		level           int
		promote, remove bool
	}{
		{"regular account", &user{ID: 1, AccessLevel: 3}, 2, 0, true, false},
		{"gm account", &user{ID: 1, AccessLevel: 3}, 2, 1, true, true},
		{"admin account", &user{ID: 1, AccessLevel: 3}, 2, 3, false, true},
		{"higher account", &user{ID: 1, AccessLevel: 3}, 2, 4, false, false},
		{"own account", &user{ID: 1, AccessLevel: 3}, 1, 3, false, false},
		{"gm viewer", &user{ID: 1, AccessLevel: 1}, 2, 0, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			v := characterView{Found: true, Name: "Hero", AccountName: "Example", AccountID: test.accountID, AccessLevel: test.level, User: test.viewer, Online: true}
			if err := characterPage.Execute(&out, v); err != nil {
				t.Fatal(err)
			}
			for action, enabled := range map[string]bool{"promote_admin": test.promote, "remove_gm": test.remove} {
				if got := strings.Contains(out.String(), `value="`+action+`">`); got != enabled {
					t.Errorf("%s enabled = %v, want %v", action, got, enabled)
				}
			}
			if strings.Contains(out.String(), "GM access</h3>") != test.viewer.Admin() {
				t.Fatal("incorrect access control visibility")
			}
		})
	}
}

func TestCharacterAccessAuthorization(t *testing.T) {
	p := &panel{}
	admin := &user{ID: 1, AccessLevel: adminLevel}
	for _, action := range []string{"promote_admin", "remove_gm"} {
		for _, viewer := range []*user{nil, {ID: 2, AccessLevel: gmLevel}, admin} {
			form := url.Values{"action": {action}, "name": {"Hero"}, "token": {characterToken(&user{ID: 9})}}
			r := httptest.NewRequest("POST", "/admin/character", strings.NewReader(form.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			p.editCharacter(viewer, w, r)
			if w.Code != 403 {
				t.Fatalf("%s status = %d", action, w.Code)
			}
		}
	}
	for _, level := range []int{-1, 1, 4} {
		if err := p.setCharacterAccess(httptest.NewRequest("POST", "/admin/character", nil), admin, "Hero", level); err == nil {
			t.Fatalf("accepted access %d", level)
		}
	}
}

// Both account and character tables are disposable; no persistent schema is used.
func TestCharacterAccessDatabase(t *testing.T) {
	host := os.Getenv("AION_TEST_DB")
	if host == "" {
		t.Skip("set AION_TEST_DB for MariaDB integration")
	}
	config := mysql.NewConfig()
	config.Net, config.Addr, config.User, config.Passwd = "tcp", host+":3306", "root", "aion"
	schema := fmt.Sprintf("panel_access_test_%d", time.Now().UnixNano())
	setup, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer setup.Close()
	if _, err := setup.Exec("CREATE DATABASE " + schema); err != nil {
		t.Fatal(err)
	}
	defer setup.Exec("DROP DATABASE " + schema)
	config.DBName = schema
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		`CREATE TABLE account_data (id INT PRIMARY KEY, name VARCHAR(50), access_level INT) ENGINE=InnoDB`,
		`CREATE TABLE players (id INT PRIMARY KEY, name VARCHAR(50), account_id INT, deletion_date DATETIME NULL, online BOOL, race VARCHAR(20) DEFAULT 'ELYOS', player_class VARCHAR(20) DEFAULT 'SCOUT', exp BIGINT DEFAULT 0, cube_size INT DEFAULT 0) ENGINE=InnoDB`,
		`CREATE TABLE inventory (itemId INT, itemCount BIGINT, isEquiped BOOL, slot INT, itemLocation INT, enchant INT, itemOwner INT) ENGINE=InnoDB`,
		`INSERT INTO account_data VALUES (1,'Admin',3),(2,'Example',0),(3,'Senior',4)`,
		`INSERT INTO players (id,name,account_id,deletion_date,online) VALUES (10,'Hero',2,NULL,1),(11,'Alt',2,NULL,0),(12,'Self',1,NULL,0),(13,'Senior',3,NULL,0),(14,'Deleted',2,NOW(),0),(15,'Orphan',99,NULL,0)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	p := &panel{db: db, gsDB: schema, assets: &assets{}, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	admin := &user{ID: 1, AccessLevel: 3}
	request := httptest.NewRequest("POST", "/admin/character", nil)
	checkLevel := func(want int) {
		t.Helper()
		var got int
		if err := db.QueryRow(`SELECT access_level FROM account_data WHERE id=2`).Scan(&got); err != nil || got != want {
			t.Fatalf("access = %d, %v; want %d", got, err, want)
		}
	}
	for _, name := range []string{"Missing", "Deleted", "Self", "Senior", "Orphan"} {
		if err := p.setCharacterAccess(request, admin, name, 3); err == nil {
			t.Fatalf("accepted %s", name)
		}
		checkLevel(0)
	}
	for _, action := range []string{"promote_admin", "promote_admin", "remove_gm"} {
		form := url.Values{"action": {action}, "name": {"Hero"}, "token": {characterToken(admin)}}
		r := httptest.NewRequest("POST", "/admin/character", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		p.editCharacter(admin, w, r)
		if w.Code != 303 || strings.Contains(w.Header().Get("Location"), "&e") {
			t.Fatalf("action %s: %d %s", action, w.Code, w.Header().Get("Location"))
		}
		want := 3
		if action == "remove_gm" {
			want = 0
		}
		checkLevel(want)
		view := characterView{Query: "Hero"}
		if err := p.findCharacter(&view); err != nil || !view.Found || view.AccountID != 2 || view.AccountName != "Example" || view.AccessLevel != want {
			t.Fatalf("character account view = %+v, %v", view, err)
		}
		var alt int
		if err := db.QueryRow(`SELECT a.access_level FROM players p JOIN account_data a ON a.id=p.account_id WHERE p.name='Alt'`).Scan(&alt); err != nil || alt != want {
			t.Fatalf("alt access = %d, %v", alt, err)
		}
	}
	// A failed account update must leave the previous access intact.
	if _, err := db.Exec(`CREATE TRIGGER reject_access BEFORE UPDATE ON account_data FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'fixture failure'`); err != nil {
		t.Fatal(err)
	}
	if err := p.setCharacterAccess(request, admin, "Hero", 3); err == nil {
		t.Fatal("accepted failed write")
	}
	checkLevel(0)
}
