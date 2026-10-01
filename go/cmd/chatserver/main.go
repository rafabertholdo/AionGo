// Command chatserver is the Aion 1.9 chat server, in place of AL-CServer.
//
// Game servers connect on 9021 with AION_CS_PASSWORD (default aion) and are
// told clients connect at AION_CHAT_HOST (default 127.0.0.1, where ReRun and
// cloudflared give every player the servers' ports) on 10241.
package main

import (
	"log/slog"
	"net"
	"net/netip"
	"os"
	"time"

	"aionlightning/chat"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	started := time.Now()
	host, err := netip.ParseAddr(env("AION_CHAT_HOST", "127.0.0.1"))
	if err != nil || !host.Is4() {
		fail(log, "AION_CHAT_HOST must be an IPv4 address", err)
	}
	server := chat.NewServer(env("AION_CS_PASSWORD", "aion"), host.As4(), 10241, log)
	clients, err := net.Listen("tcp", ":10241")
	if err != nil {
		fail(log, "listening for players", err)
	}
	gameServers, err := net.Listen("tcp", ":9021")
	if err != nil {
		fail(log, "listening for game servers", err)
	}
	// ReRun and aion-servers.sh wait for this line, as AL-CServer prints it.
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
