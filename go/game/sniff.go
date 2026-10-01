package game

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"

	"aionlightning/crypt"
	"aionlightning/wire"
)

// Sniffer relays game clients to a game server and logs every packet
// decrypted, so a Go packet can be compared byte for byte with Java's. The
// target can be changed while it runs, which drops the connections made to the
// old one.
type Sniffer struct {
	log *slog.Logger

	mu      sync.Mutex
	target  string
	clients map[net.Conn]bool
}

// NewSniffer relays to target, the address of the game server.
func NewSniffer(target string, log *slog.Logger) *Sniffer {
	return &Sniffer{log: log, target: target, clients: map[net.Conn]bool{}}
}

// SetTarget sends new connections to target and drops the open ones.
func (s *Sniffer) SetTarget(target string) {
	s.mu.Lock()
	s.target = target
	for c := range s.clients {
		_ = c.Close()
	}
	s.mu.Unlock()
	s.log.Info("switch", "target", target)
}

// Mark writes a marker line into the log, for windowing it later (pktdiff -label).
func (s *Sniffer) Mark(label string) { s.log.Info("mark", "label", label) }

// Serve relays the clients accepted from l until it fails.
func (s *Sniffer) Serve(l net.Listener) error {
	for {
		client, err := l.Accept()
		if err != nil {
			return err
		}
		s.mu.Lock()
		target := s.target
		s.clients[client] = true
		s.mu.Unlock()
		go func() {
			sniffConnection(client, target, s.log)
			s.mu.Lock()
			delete(s.clients, client)
			s.mu.Unlock()
		}()
	}
}

// Control serves one command per line on l: "target host:port", "mark label"
// or "status"; each is answered with one line.
func (s *Sniffer) Control(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go func() {
			defer conn.Close()
			scanner := bufio.NewScanner(conn)
			for scanner.Scan() {
				command, argument, _ := strings.Cut(strings.TrimSpace(scanner.Text()), " ")
				argument = strings.TrimSpace(argument)
				switch {
				case command == "target" && argument != "":
					s.SetTarget(argument)
					fmt.Fprintf(conn, "ok target %s\n", argument)
				case command == "mark" && argument != "":
					s.Mark(argument)
					fmt.Fprintf(conn, "ok mark %s\n", argument)
				case command == "status":
					s.mu.Lock()
					fmt.Fprintf(conn, "ok target %s clients %d\n", s.target, len(s.clients))
					s.mu.Unlock()
				default:
					fmt.Fprintf(conn, "error: target host:port | mark label | status\n")
				}
			}
		}()
	}
}

func sniffConnection(client net.Conn, target string, log *slog.Logger) {
	defer client.Close()
	log.Info("connection", "target", target)
	server, err := net.Dial("tcp", target)
	if err != nil {
		log.Error("dial", "target", target, "err", err)
		return
	}
	defer server.Close()
	// The first server packet, SM_KEY, is in the clear and gives the connection's key.
	keyed := make(chan uint32, 1)
	go func() {
		defer client.Close()
		var fromServer *crypt.GameCipher
		for {
			payload, err := wire.ReadFrame(server)
			if err != nil {
				return
			}
			_, _ = client.Write(wire.Frame(payload))
			plain := append([]byte(nil), payload...)
			if fromServer == nil {
				key := crypt.GameKeyReceived(int32(binary.LittleEndian.Uint32(plain[3:])))
				fromServer = crypt.NewGameCipher(key)
				keyed <- key
			} else {
				fromServer.Decrypt(plain)
			}
			op := crypt.DecodeServerOpcode(plain[0])
			log.Info("server", "op", serverNames[op], "size", len(plain), "hex", hex.EncodeToString(plain[3:]))
		}
	}()
	fromClient := crypt.NewGameCipher(<-keyed)
	for {
		payload, err := wire.ReadFrame(client)
		if err != nil {
			return
		}
		_, _ = server.Write(wire.Frame(payload))
		plain := append([]byte(nil), payload...)
		fromClient.Decrypt(plain)
		log.Info("client", "op", clientNames[plain[0]], "size", len(plain), "hex", hex.EncodeToString(plain[3:]))
	}
}
