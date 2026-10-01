// Command gamesniff relays Aion game clients to a game server and logs every
// packet decrypted, for comparing the Go game server with AL-Game.
//
//	gamesniff -listen :7777 -target 192.168.64.10:7777 -control :7778
//
// The control port takes one line per command: "target host:port" (retarget and
// drop open connections), "mark label" (a marker line in the log), "status".
package main

import (
	"flag"
	"log/slog"
	"net"
	"os"

	"aionlightning/game"
)

func main() {
	listen := flag.String("listen", ":7777", "address clients connect to")
	target := flag.String("target", "", "game server address")
	control := flag.String("control", ":7778", "address for the control commands (empty to disable)")
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	l, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Error("listen", "err", err)
		os.Exit(1)
	}
	sniffer := game.NewSniffer(*target, log)
	if *control != "" {
		c, err := net.Listen("tcp", *control)
		if err != nil {
			log.Error("control listen", "err", err)
			os.Exit(1)
		}
		go func() { log.Error("control", "err", sniffer.Control(c)) }()
	}
	log.Error("sniffing", "err", sniffer.Serve(l))
}
