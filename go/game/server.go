// Package game is the Aion 1.9 game server, a port of Aion Lightning's AL-Game
// in progress: see PORTING.md for what it covers so far.
package game

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// Config is what AL-Game reads from its config files, as the game server needs it.
type Config struct {
	Shutdown      func(bool) // lifecycle callback, after clients are disconnected
	ID            byte       // network.login.gsid
	Name          string     // gameserver.name, for logs
	CountryCode   byte       // gameserver.country.code
	Mode          byte       // gameserver.mode: 0x80 one race, 0x01 free race, 0x22 per character
	HostAddress   [4]byte
	Port          uint16
	MaxPlayers    int32
	LoginAddress  string // login server, host:9014
	LoginPassword string
	ChatAddress   string // chat server, host:9021
	ChatPassword  string
	NamePattern   string // gameserver.character.name.pattern

	// custom.properties, all off in AL-Game unless set.
	SimpleSecondClass   bool // enable.simple.2ndclass: pick the level 9 class from a dialog
	HTMLWelcome         bool // enable.html.welcome: show HTML/welcome.xhtml on entering the world
	CrossFactionBinding bool // cross.faction.binding: bind in the other race's land
}

// Server is the game server: its links to the login and chat servers, and its players.
type Server struct {
	adminDB            adminSaver
	adminCommands      map[string]byte
	announcementTasks  []*task
	adminShutdownTasks []*task
	adminSpawnGroups   map[*data.SpawnGroup]bool
	configMu           sync.RWMutex
	config             Config
	data               *data.Data
	store              store.Store
	items              itemSaver // where items are kept: the store, or a stand-in in tests
	quests             questSaver
	skillDB            skillSaver
	social             socialSaver
	mailDB             mailSaver
	legionDB           legionSaver
	macroDB            macroSaver
	brokerDB           brokerSaver
	punishDB           punishSaver
	petitionDB         petitionSaver
	petitions          []*store.Petition      // the open ones, oldest first
	boards             map[string]*board      // the broker of each race
	kisks              map[int32]*object      // the kisk each player is bound to, by player id
	legions            map[int32]*legion      // the legions that have been loaded
	duels              map[int32]int32        // each duelling player's opponent
	pvpKills           map[[2]int32]int       // how many times a player has killed another, by winner and victim
	instances          map[[2]int32]*instance // the instances of instance maps, by map and instance id
	nextInstance       map[int32]int32        // the id the next instance of each map gets
	exchanges          map[int32]*exchange    // each trading player's side of the trade
	log                *slog.Logger
	ids                *idFactory
	names              *namePattern

	login *loginLink
	chat  *chatLink

	clientWG sync.WaitGroup
	clients  map[net.Conn]*conn
	mu       sync.Mutex
	accounts map[int32]*conn // logged in, by account id
	players  map[int32]*conn // in the world, by player id
	weathers map[int32]weatherState
	acctWH   map[int32]*accountWarehouse // the account warehouses of the accounts that have played, by account id

	// visMu guards what players see of each other: their positions, known lists and the spawned set.
	visMu   sync.Mutex
	spawned map[int32]*player // in the world and visible, by player id

	byID   map[int32]*object          // every object, by id
	pcells map[cell]map[int32]*player // the spawned players in each square of each map
	grid   map[cell][]*object         // the npcs and gatherables in each square of each map: fixed once spawned

	siegeOwners map[int32]store.SiegeOwner
	siegeMu     sync.Mutex          // guards sieges and siegeOwners
	sieges      map[int32]*siegeLoc // the state of each siege location, made when first read
	siegeDB     interface {
		SaveSiegeOwner(int32, store.SiegeOwner) error
	}
	fxTemplates map[*data.SkillTemplate][]*effectTemplate // each skill's effects, made when first used
	fxDirty     map[creature]bool                         // whose effect icons are to be sent
	drops       map[int32][]store.Drop                    // what each monster may drop, by npc id

	clockBase  int32 // game time at start
	clockStart time.Time
}

// NewServer loads the used object ids and starts the login and chat server links.
func NewServer(config Config, d *data.Data, s store.Store, log *slog.Logger) (*Server, error) {
	names, err := compileNamePattern(config.NamePattern)
	if err != nil {
		return nil, err
	}
	used, err := s.UsedIDs()
	if err != nil {
		return nil, fmt.Errorf("loading used ids: %w", err)
	}
	owners, err := s.SiegeOwners()
	if err != nil {
		return nil, fmt.Errorf("loading siege locations: %w", err)
	}
	drops, err := s.DropList()
	if err != nil {
		return nil, fmt.Errorf("loading drops: %w", err)
	}
	clock, err := s.GameTime()
	if err != nil {
		return nil, fmt.Errorf("loading the game time: %w", err)
	}
	server := &Server{adminDB: s, items: s, quests: s, skillDB: s, social: s, mailDB: s, legionDB: s, macroDB: s, brokerDB: s, punishDB: s, petitionDB: s, config: config, data: d, store: s, log: log, ids: newIDFactory(used), names: names,
		accounts: map[int32]*conn{}, players: map[int32]*conn{}, weathers: map[int32]weatherState{}, spawned: map[int32]*player{}, grid: map[cell][]*object{}, byID: map[int32]*object{}, pcells: map[cell]map[int32]*player{},
		drops: drops, siegeOwners: owners, siegeDB: s, clockBase: clock, clockStart: time.Now()}
	log.Info("IDFactory", "used", len(used))
	if err := server.loadPetitions(); err != nil {
		return nil, fmt.Errorf("loading petitions: %w", err)
	}
	server.spawnAll()
	if err := server.loadBroker(); err != nil {
		return nil, fmt.Errorf("loading the broker: %w", err)
	}
	server.startWorldTasks()
	server.startRifts()
	server.startZoneTasks()
	if list, err := s.AutoAnnouncements(); err != nil {
		log.Warn("loading the announcements", "err", err)
	} else {
		server.announcementTasks = server.startAnnouncements(list)
	}
	server.login = newLoginLink(server)
	server.chat = newChatLink(server)
	go server.login.run()
	go server.chat.run()
	return server, nil
}

// Serve accepts game clients on l until it closes.
func (s *Server) Serve(l net.Listener) error {
	for {
		c, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		if tcp, ok := c.(*net.TCPConn); ok {
			_ = tcp.SetNoDelay(true)
		}

		s.mu.Lock()
		if s.clients == nil {
			s.clients = map[net.Conn]*conn{}
		}
		client := s.newClient(c)
		s.clients[c] = client
		s.clientWG.Add(1)
		s.mu.Unlock()
		go func() {
			defer s.clientWG.Done()
			defer func() { s.mu.Lock(); delete(s.clients, c); s.mu.Unlock() }()
			s.handle(client)
		}()

	}
}

// idFactory hands out object ids for players and items, skipping those the database uses.
type idFactory struct {
	mu   sync.Mutex
	used map[int32]bool
	next int32
}

// firstObjectID is the lowest id handed out. AL-Game's spawns take the first
// tens of thousands at startup, so its players and items never get low ids,
// and the 1.9 client shows a player with one as an unnamed object in the world.
// ponytail: a fixed floor until spawns are ported (PORTING.md 5), which then
// take ids from it as AL-Game's do.
const firstObjectID = 0x10000

func newIDFactory(used []int32) *idFactory {
	f := &idFactory{used: map[int32]bool{0: true}, next: firstObjectID}
	for _, id := range used {
		f.used[id] = true
	}
	return f
}

func (f *idFactory) nextID() int32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	for f.used[f.next] {
		f.next++
	}
	id := f.next
	f.used[id] = true
	f.next++
	return id
}

func (f *idFactory) release(id int32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.used, id)
	if id < f.next && id >= firstObjectID {
		f.next = id
	}
}

func (s *Server) currentConfig() Config {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.config
}

// CloseClients is called after Serve returns. Every connected client receives the
// same final packet as an administrative kick, including the character screen.
// It waits for reader cleanup and character saves before process exit.
func (s *Server) CloseClients() {
	s.mu.Lock()
	clients := make([]*conn, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()
	// A stalled client must not delay sending the kick to everyone else.
	var kicks sync.WaitGroup
	for _, c := range clients {
		kicks.Go(func() { c.close(quitResponse()) })
	}
	kicks.Wait()
	s.clientWG.Wait()
	s.log.Info("game clients disconnected and character saves completed", "clients", len(clients))
}
