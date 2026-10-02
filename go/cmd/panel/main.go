// Command panel is the server's website: anyone can create a game account on the
// front page, and players with an access level sign in with that account for the
// GM tools (level 1+: characters, account locks, IP bans) and the admin tools
// (level 3+: access levels, server options, game server rows).
//
// It shares the login server's environment: AION_DB, AION_DB_USER and
// AION_DB_PASSWORD, plus AION_GS_DB (default au_server_gs) for the characters
// and AION_PANEL_ASSETS (default /assets), the client art and item data that
// scripts/extract-panel-assets.py writes, for drawing inventories like the game.
// It serves on :8080. Server options are read by the login and game servers when
// they start, so they apply on the next restart; account changes apply on the next login.
package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"aionlightning/login"
	"aionlightning/options"
)

const (
	gmLevel    = 1 // AL-Game: any access level unlocks the in-game commands
	adminLevel = 3
)

var (
	//go:embed page.html
	pageHTML string
	page     = template.Must(template.New("page").Parse(pageHTML))

	// The 1.9 client's login fields take at most 16 characters.
	accountName = regexp.MustCompile(`^[A-Za-z0-9]{3,16}$`)
	// sessionKey signs session cookies. It lives in server_options (not in the catalog,
	// so the options form never shows or overwrites it), so restarts keep people signed in.
	sessionKey []byte
)

type panel struct {
	db     *sql.DB
	gsDB   string
	log    *slog.Logger
	assets *assets
}

// user is the signed-in account.
type user struct {
	ID          int32
	Name        string
	AccessLevel int
}

func (u *user) GM() bool    { return u != nil && u.AccessLevel >= gmLevel }
func (u *user) Admin() bool { return u != nil && u.AccessLevel >= adminLevel }

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := mysql.NewConfig()
	config.Net = "tcp"
	config.Addr = env("AION_DB", "localhost") + ":3306"
	config.User = env("AION_DB_USER", "root")
	config.Passwd = env("AION_DB_PASSWORD", "aion")
	config.DBName = "au_server_ls"
	config.ParseTime = true
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		fail(log, "opening the database", err)
	}
	for attempt := 0; ; attempt++ {
		if _, err = db.Exec(options.Table); err == nil {
			break
		}
		if attempt == 30 {
			fail(log, "preparing the options table", err)
		}
		time.Sleep(2 * time.Second)
	}

	if sessionKey, err = loadSessionKey(db); err != nil {
		fail(log, "loading the session key", err)
	}

	p := &panel{db: db, gsDB: env("AION_GS_DB", "au_server_gs"), log: log,
		assets: loadAssets(env("AION_PANEL_ASSETS", "/assets"), log)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", p.show)
	mux.HandleFunc("GET /help", p.help)
	mux.HandleFunc("POST /signup", p.signup)
	mux.HandleFunc("POST /signin", p.signin)
	mux.HandleFunc("POST /signout", p.signout)
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(p.assets.dir))))
	mux.HandleFunc("GET /character", p.gm(p.character))
	mux.HandleFunc("POST /gm/account", p.gm(p.lockAccount))
	mux.HandleFunc("POST /gm/ban", p.gm(p.ban))
	mux.HandleFunc("POST /gm/unban", p.gm(p.unban))
	mux.HandleFunc("POST /admin/access", p.admin(p.setAccess))
	mux.HandleFunc("POST /admin/options", p.admin(p.saveOptions))
	mux.HandleFunc("POST /admin/gameserver", p.admin(p.addGameServer))
	mux.HandleFunc("POST /admin/gameserver/delete", p.admin(p.deleteGameServer))
	log.Info("panel listening", "port", 8080)
	fail(log, "serving", http.ListenAndServe(":8080", mux))
}

// view is everything the page can show; the template skips what the user may not see.
type view struct {
	User        *user
	Next        string // where to go after signing in
	Message     string
	Error       bool
	Signup      bool
	Options     []optionRow
	Accounts    []accountRow
	Characters  []characterRow
	Bans        []login.Ban
	GameServers []login.GameServerRow
}

type optionRow struct {
	options.Option
	Value string // the stored value, empty when unset
}

type accountRow struct {
	ID          int32
	Name        string
	AccessLevel int
	Activated   bool
	LastIP      string
}

type characterRow struct {
	Name, Account, Race, Class string
	Exp                        int64
	Online                     bool
	LastOnline                 time.Time
}

func (p *panel) show(w http.ResponseWriter, r *http.Request) {
	u := p.current(r)
	v := view{User: u, Next: r.URL.Query().Get("next"), Message: r.URL.Query().Get("m"), Error: r.URL.Query().Has("e"),
		Signup: options.Load(p.db).On("PANEL_SIGNUP", true)}
	var err error
	if u.GM() {
		v.Accounts, err = p.accounts()
		if err == nil {
			v.Characters, err = p.characters()
		}
		if err == nil {
			v.Bans, err = login.SQLStore{DB: p.db}.Bans()
		}
	}
	if err == nil && u.Admin() {
		stored := options.Load(p.db)
		for _, o := range options.Catalog {
			v.Options = append(v.Options, optionRow{o, stored[o.Name]})
		}
		v.GameServers, err = login.SQLStore{DB: p.db}.GameServers()
	}
	if err != nil {
		p.log.Error("loading the page", "err", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Execute(w, v); err != nil {
		p.log.Error("rendering", "err", err)
	}
}

func (p *panel) accounts() ([]accountRow, error) {
	rows, err := p.db.Query(`SELECT id, name, access_level, activated, COALESCE(last_ip, '') FROM account_data ORDER BY access_level DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []accountRow
	for rows.Next() {
		var a accountRow
		if err := rows.Scan(&a.ID, &a.Name, &a.AccessLevel, &a.Activated, &a.LastIP); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (p *panel) characters() ([]characterRow, error) {
	// ponytail: newest 200 only; add search when there are more players than that.
	rows, err := p.db.Query(`SELECT name, account_name, race, player_class, exp, online, last_online FROM ` +
		p.gsDB + `.players WHERE deletion_date IS NULL ORDER BY online DESC, last_online DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []characterRow
	for rows.Next() {
		var c characterRow
		if err := rows.Scan(&c.Name, &c.Account, &c.Race, &c.Class, &c.Exp, &c.Online, &c.LastOnline); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (p *panel) signup(w http.ResponseWriter, r *http.Request) {
	if !options.Load(p.db).On("PANEL_SIGNUP", true) {
		back(w, r, "Sign-up is closed.", true)
		return
	}
	name, password := r.FormValue("name"), r.FormValue("password")
	switch {
	case !accountName.MatchString(name):
		back(w, r, "Account names are 3 to 16 letters or digits.", true)
		return
	case len(password) < 4 || len(password) > 16:
		back(w, r, "Passwords are 4 to 16 characters.", true)
		return
	case password != r.FormValue("confirm"):
		back(w, r, "The passwords don't match.", true)
		return
	}
	// The unique name column turns a taken name into an error.
	if _, err := (login.SQLStore{DB: p.db}).CreateAccount(name, login.HashPassword(password)); err != nil {
		back(w, r, "That account name is taken.", true)
		return
	}
	p.log.Info("account created", "name", name, "from", clientIP(r))
	back(w, r, "Account "+name+" created. Log in from the game.", false)
}

func (p *panel) signin(w http.ResponseWriter, r *http.Request) {
	a, err := login.SQLStore{DB: p.db}.Account(r.FormValue("name"))
	if err != nil || a == nil || subtle.ConstantTimeCompare([]byte(a.PasswordHash), []byte(login.HashPassword(r.FormValue("password")))) != 1 || a.Activated != 1 {
		back(w, r, "Wrong account name or password.", true)
		return
	}
	expires := time.Now().Add(12 * time.Hour)
	http.SetCookie(w, &http.Cookie{Name: "session", Value: sign(a.ID, expires), Path: "/", Expires: expires,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"})
	if next := r.FormValue("next"); localPath(next) {
		http.Redirect(w, r, next, http.StatusSeeOther)
		return
	}
	back(w, r, "", false)
}

// loadSessionKey reads the cookie signing key, making one the first time.
func loadSessionKey(db *sql.DB) ([]byte, error) {
	fresh := make([]byte, 32)
	rand.Read(fresh)
	if _, err := db.Exec(`INSERT IGNORE INTO server_options (name, value) VALUES ('PANEL_SESSION_KEY', ?)`, hex.EncodeToString(fresh)); err != nil {
		return nil, err
	}
	var key string
	if err := db.QueryRow(`SELECT value FROM server_options WHERE name = 'PANEL_SESSION_KEY'`).Scan(&key); err != nil {
		return nil, err
	}
	return hex.DecodeString(key)
}

// localPath is a path on this site, never another one ("//host" or "/\\host" would leave it).
func localPath(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "/\\")
}

func (p *panel) signout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "session", Path: "/", MaxAge: -1})
	back(w, r, "", false)
}

// sign makes a session cookie: the account id, the expiry and their HMAC.
func sign(id int32, expires time.Time) string {
	payload := strconv.Itoa(int(id)) + "." + strconv.FormatInt(expires.Unix(), 10)
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

// verify is the account id of a valid, unexpired session cookie.
func verify(cookie string, now time.Time) (int32, bool) {
	parts := strings.Split(cookie, ".")
	if len(parts) != 3 {
		return 0, false
	}
	id, err1 := strconv.Atoi(parts[0])
	expires, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil || now.Unix() > expires {
		return 0, false
	}
	return int32(id), hmac.Equal([]byte(sign(int32(id), time.Unix(expires, 0))), []byte(cookie))
}

// current is the signed-in account, read again on each request so a demotion or lock applies at once.
func (p *panel) current(r *http.Request) *user {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil
	}
	id, ok := verify(cookie.Value, time.Now())
	if !ok {
		return nil
	}
	u := &user{}
	if p.db.QueryRow(`SELECT id, name, access_level FROM account_data WHERE id = ? AND activated = 1`, id).
		Scan(&u.ID, &u.Name, &u.AccessLevel) != nil {
		return nil
	}
	return u
}

func (p *panel) gm(next func(*user, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return p.require(gmLevel, next)
}

func (p *panel) admin(next func(*user, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return p.require(adminLevel, next)
}

func (p *panel) require(level int, next func(*user, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := p.current(r)
		if u == nil && r.Method == http.MethodGet {
			// Signed out (or the cookie expired): sign in, then come back here.
			http.Redirect(w, r, "/?m="+template.URLQueryEscaper("Sign in to see that page.")+"&next="+
				template.URLQueryEscaper(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		if u == nil || u.AccessLevel < level {
			http.Error(w, "forbidden: this page needs a GM account", http.StatusForbidden)
			return
		}
		next(u, w, r)
		if r.Method == http.MethodPost {
			r.PostForm.Del("password")
			p.log.Info("panel action", "by", u.Name, "path", r.URL.Path, "form", r.PostForm.Encode())
		}
	}
}

// lockAccount turns an account's login on or off; a GM may only do it to lower levels.
func (p *panel) lockAccount(u *user, w http.ResponseWriter, r *http.Request) {
	activated := 0
	if r.FormValue("activated") == "1" {
		activated = 1
	}
	p.result(w, r, "Saved.")(p.db.Exec(`UPDATE account_data SET activated = ? WHERE id = ? AND access_level < ?`,
		activated, r.FormValue("id"), u.AccessLevel))
}

func (p *panel) ban(u *user, w http.ResponseWriter, r *http.Request) {
	mask := strings.TrimSpace(r.FormValue("mask"))
	if mask == "" {
		back(w, r, "Give an address or mask.", true)
		return
	}
	var end *time.Time
	if hours, err := strconv.Atoi(r.FormValue("hours")); err == nil && hours > 0 {
		t := time.Now().Add(time.Duration(hours) * time.Hour)
		end = &t
	}
	if err := (login.SQLStore{DB: p.db}).AddBan(mask, end); err != nil {
		back(w, r, err.Error(), true)
		return
	}
	back(w, r, "Banned "+mask+". The login server reads bans at start.", false)
}

func (p *panel) unban(u *user, w http.ResponseWriter, r *http.Request) {
	if err := (login.SQLStore{DB: p.db}).RemoveBan(r.FormValue("mask")); err != nil {
		back(w, r, err.Error(), true)
		return
	}
	back(w, r, "Unbanned.", false)
}

func (p *panel) setAccess(u *user, w http.ResponseWriter, r *http.Request) {
	level, err := strconv.Atoi(r.FormValue("level"))
	if err != nil || level < 0 || level > u.AccessLevel {
		back(w, r, "Access levels go from 0 to your own.", true)
		return
	}
	p.result(w, r, "Saved. It applies on that account's next login.")(p.db.Exec(
		`UPDATE account_data SET access_level = ? WHERE id = ? AND id <> ?`, level, r.FormValue("id"), u.ID))
}

func (p *panel) saveOptions(u *user, w http.ResponseWriter, r *http.Request) {
	for _, o := range options.Catalog {
		value := strings.TrimSpace(r.FormValue(o.Name))
		if o.Bool {
			value = strconv.FormatBool(value == "on")
		}
		var err error
		if value == "" {
			_, err = p.db.Exec(`DELETE FROM server_options WHERE name = ?`, o.Name)
		} else {
			_, err = p.db.Exec(`REPLACE INTO server_options (name, value) VALUES (?, ?)`, o.Name, value)
		}
		if err != nil {
			back(w, r, err.Error(), true)
			return
		}
	}
	back(w, r, "Options saved. Restart the login and game servers to apply them.", false)
}

func (p *panel) addGameServer(u *user, w http.ResponseWriter, r *http.Request) {
	p.result(w, r, "Game server added. Restart the login server to read it.")(p.db.Exec(
		`INSERT INTO gameservers (id, mask, password) VALUES (?, ?, ?)`, r.FormValue("id"), r.FormValue("mask"), r.FormValue("password")))
}

func (p *panel) deleteGameServer(u *user, w http.ResponseWriter, r *http.Request) {
	p.result(w, r, "Game server removed. Restart the login server to apply it.")(p.db.Exec(
		`DELETE FROM gameservers WHERE id = ?`, r.FormValue("id")))
}

// result reports an Exec's outcome back on the page.
func (p *panel) result(w http.ResponseWriter, r *http.Request, done string) func(sql.Result, error) {
	return func(_ sql.Result, err error) {
		if err != nil {
			back(w, r, err.Error(), true)
			return
		}
		back(w, r, done, false)
	}
}

// back returns to the page with a message (post, redirect, get).
func back(w http.ResponseWriter, r *http.Request, message string, failed bool) {
	target := "/"
	if message != "" {
		target += "?m=" + template.URLQueryEscaper(message)
		if failed {
			target += "&e"
		}
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func clientIP(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func fail(log *slog.Logger, what string, err error) {
	log.Error(what, "err", err)
	os.Exit(1)
}
