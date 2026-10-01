// Command gameserver is the Aion 1.9 game server in Go, in place of AL-Game
// as it becomes complete (see PORTING.md).
//
// It takes the Java image's environment: AION_DB, AION_DB_USER,
// AION_DB_PASSWORD; AION_LS and AION_CS (the login and chat servers' hosts),
// AION_LS_PASSWORD; AION_GSID, SERVER_CC; HOST_NAME, the address players are
// sent to. AION_DATA is AL-Game's data/static_data folder.
package main

import (
	"database/sql"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"

	"aionlightning/game"
	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/options"
)

func main() {
	level := slog.LevelInfo
	if os.Getenv("AION_DEBUG") != "" {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	started := time.Now()

	host, err := netip.ParseAddr(env("HOST_NAME", "127.0.0.1"))
	if err != nil || !host.Is4() {
		fail(log, "HOST_NAME must be an IPv4 address", err)
	}
	static, err := data.Load(env("AION_DATA", "/data/static_data"))
	if err != nil {
		fail(log, "loading static data", err)
	}
	log.Info("static data loaded", "items", len(static.Items), "seconds", time.Since(started).Seconds())

	config := mysql.NewConfig()
	config.Net = "tcp"
	config.Addr = env("AION_DB", "localhost") + ":3306"
	config.User = env("AION_DB_USER", "root")
	config.Passwd = env("AION_DB_PASSWORD", "aion")
	config.DBName = "au_server_gs"
	config.ParseTime = true
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		fail(log, "opening the database", err)
	}
	for attempt := 0; ; attempt++ {
		if err = db.Ping(); err == nil {
			break
		}
		if attempt == 30 {
			fail(log, "connecting to the database", err)
		}
		time.Sleep(2 * time.Second)
	}

	// The panel's settings (au_server_ls.server_options) win over the environment.
	opts := options.Load(db)
	server, err := game.NewServer(game.Config{
		ID:            byte(number(os.Getenv("AION_GSID"), 1)),
		Name:          opts.Get("SERVER_NAME", "Siel"),
		CountryCode:   byte(number(os.Getenv("SERVER_CC"), 1)),
		Mode:          1,
		HostAddress:   host.As4(),
		Port:          7777,
		MaxPlayers:    int32(number(opts.Get("MAX_PLAYERS", ""), 100)),
		LoginAddress:  env("AION_LS", "localhost") + ":9014",
		LoginPassword: env("AION_LS_PASSWORD", "aion"),
		ChatAddress:   env("AION_CS", "localhost") + ":9021",
		ChatPassword:  env("AION_LS_PASSWORD", "aion"),
		NamePattern:   opts.Get("NAME_PATTERN", "[a-zA-Z]{2,16}"),

		SimpleSecondClass:   opts.On("AION_SIMPLE_2NDCLASS", false),
		HTMLWelcome:         opts.On("AION_HTML_WELCOME", false),
		CrossFactionBinding: opts.On("AION_CROSS_FACTION_BINDING", false),
	}, static, store.Store{DB: db}, log)
	if err != nil {
		fail(log, "starting", err)
	}
	clients, err := net.Listen("tcp", ":7777")
	if err != nil {
		fail(log, "listening for players", err)
	}
	// ReRun and aion-servers.sh wait for this line, as AL-Game prints it.
	log.Info("Total Boot Time", "seconds", time.Since(started).Seconds())
	fail(log, "serving players", server.Serve(clients))
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func number(text string, fallback int) int {
	if value, err := strconv.Atoi(text); err == nil {
		return value
	}
	return fallback
}

func fail(log *slog.Logger, what string, err error) {
	log.Error(what, "err", err)
	os.Exit(1)
}
