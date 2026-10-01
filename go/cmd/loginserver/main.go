// Command loginserver is the Aion 1.9 login server, in place of AL-Login.
//
// It takes the same environment as the Java image: AION_DB, AION_DB_USER and
// AION_DB_PASSWORD for the au_server_ls database, and AION_AUTOCREATE (default
// true) to create accounts on first login. Players connect on 2106 and game
// servers on 9014.
package main

import (
	"database/sql"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"aionlightning/login"
	"aionlightning/options"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	started := time.Now()

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
	db.SetMaxOpenConns(5)
	// The database container may still be starting.
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
	server, err := login.NewServer(login.SQLStore{DB: db}, opts.On("AION_AUTOCREATE", true), log)
	if err != nil {
		fail(log, "starting", err)
	}
	// AION_HIDE_SERVERS=2,3 registers those game servers but leaves them off the server list.
	for _, field := range strings.Split(opts.Get("AION_HIDE_SERVERS", ""), ",") {
		if id, err := strconv.ParseUint(strings.TrimSpace(field), 10, 8); err == nil {
			server.HideServers(byte(id))
		}
	}
	clients, err := net.Listen("tcp", ":2106")
	if err != nil {
		fail(log, "listening for players", err)
	}
	gameServers, err := net.Listen("tcp", ":9014")
	if err != nil {
		fail(log, "listening for game servers", err)
	}
	// ReRun and aion-servers.sh wait for this line, as AL-Login prints it.
	log.Info("Total Boot Time", "seconds", time.Since(started).Seconds())
	go func() { fail(log, "serving game servers", server.ServeGameServers(gameServers)) }()
	fail(log, "serving players", server.ServeClients(clients))
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
