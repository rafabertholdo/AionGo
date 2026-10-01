// Package options holds the server settings the panel (cmd/panel) edits. They live in
// au_server_ls.server_options, which the login and game servers read once at start;
// a value there wins over the container's environment variable of the same name.
package options

import (
	"database/sql"
	"os"
)

// Option is one setting the panel shows.
type Option struct {
	Name    string // the table key, and the environment variable it replaces
	Server  string // which server reads it: login, game or panel
	Bool    bool   // a switch rather than text
	Default string
	Help    string
}

// Catalog is every setting the panel offers. Connection settings (AION_DB, AION_LS,
// HOST_NAME, AION_GSID...) stay in the environment: a wrong value there loses the database.
var Catalog = []Option{
	{"AION_AUTOCREATE", "login", true, "true", "Create an account on its first login, with whatever password was typed. Turn off to make players sign up on the panel."},
	{"AION_HIDE_SERVERS", "login", false, "", "Game server ids that register but stay off the server list, e.g. 2,3."},
	{"SERVER_NAME", "game", false, "Siel", "The game server's name in its logs (the client names servers itself)."},
	{"MAX_PLAYERS", "game", false, "100", "Players the game server takes before the login server calls it full."},
	{"NAME_PATTERN", "game", false, "[a-zA-Z]{2,16}", "Regular expression new character names must match."},
	{"AION_SIMPLE_2NDCLASS", "game", true, "false", "Pick the level 9 class from a dialog instead of doing the ascension quest."},
	{"AION_HTML_WELCOME", "game", true, "false", "Show the HTML welcome window on entering the world."},
	{"AION_CROSS_FACTION_BINDING", "game", true, "false", "Let players bind at obelisks in the other race's land."},
	{"PANEL_SIGNUP", "panel", true, "true", "Let anyone create an account on the panel's front page."},
}

// Table creates the options table; the panel runs it at start.
const Table = `CREATE TABLE IF NOT EXISTS au_server_ls.server_options (
	name VARCHAR(64) NOT NULL PRIMARY KEY,
	value VARCHAR(255) NOT NULL)`

// Values are the stored settings by name.
type Values map[string]string

// Load reads the stored settings. A database without the table (no panel yet) has none.
func Load(db *sql.DB) Values {
	values := Values{}
	rows, err := db.Query(`SELECT name, value FROM au_server_ls.server_options`)
	if err != nil {
		return values
	}
	defer rows.Close()
	for rows.Next() {
		var name, value string
		if rows.Scan(&name, &value) == nil {
			values[name] = value
		}
	}
	return values
}

// Get is the stored value, else the environment variable, else fallback.
func (v Values) Get(name, fallback string) string {
	if value := v[name]; value != "" {
		return value
	}
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// On reads a switch: "true", "1", "yes" and "on" are on, any other set value off.
func (v Values) On(name string, fallback bool) bool {
	switch v.Get(name, "") {
	case "":
		return fallback
	case "true", "1", "yes", "on":
		return true
	}
	return false
}
