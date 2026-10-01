// Command aionbot is a player without the game: it logs in, enters the world
// with a character (creating it if the account has none by that name), and
// walks and talks, so the real client can watch another player.
//
//	aionbot -login 192.168.64.10:2106 -game 192.168.64.12:7777 -character Botty -say hello
package main

import (
	"flag"
	"log/slog"
	"math"
	"os"
	"time"

	"aionlightning/client"
)

func main() {
	loginAddress := flag.String("login", "127.0.0.1:2106", "login server")
	gameAddress := flag.String("game", "", "game server, if not the address the login server gives")
	account := flag.String("account", "bot", "account (the login server creates it)")
	password := flag.String("password", "bot", "password")
	serverID := flag.Uint("server", 1, "game server id to play on (the login server's list entry)")
	name := flag.String("character", "Botty", "character")
	say := flag.String("say", "", "something to say in normal chat, now and then")
	stay := flag.Duration("stay", time.Minute, "how long to stay in the world")
	fight := flag.Bool("fight", false, "attack the nearest monster until it dies, and say what happened")
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	fail := func(what string, err error) {
		log.Error(what, "err", err)
		os.Exit(1)
	}

	session, err := client.Login(*loginAddress, *account, *password, byte(*serverID))
	if err != nil {
		fail("logging in", err)
	}
	for i, server := range session.Servers {
		log.Info("server list", "entry", i, "id", server.ID, "address", server.Address, "port", server.Port)
	}
	log.Info("playing on", "id", session.Server.ID, "address", session.Server.Address, "port", session.Server.Port, "override", *gameAddress)
	game, err := client.Dial(session, *gameAddress)
	if err != nil {
		fail("connecting to the game server", err)
	}
	defer game.Close()
	characters, err := game.Characters()
	if err != nil {
		fail("listing characters", err)
	}
	id := int32(0)
	for _, c := range characters {
		if c.Name == *name {
			id = c.ID
		}
	}
	if id == 0 {
		if id, err = game.Create(*name, client.Elyos, client.Female, client.Mage); err != nil {
			fail("creating the character", err)
		}
	}
	if err := game.EnterWorld(id); err != nil {
		fail("entering the world", err)
	}
	log.Info("in the world", "character", *name, "id", id)

	if *fight {
		fightMonster(game, log)
	}

	// Walk in a circle around where it appeared, saying something now and then.
	spawn, err := game.Position()
	if err != nil {
		fail("reading the position", err)
	}
	const radius = 6
	end := time.Now().Add(*stay)
	for step := 0; time.Now().Before(end); step++ {
		angle := float64(step) / 8 * 2 * math.Pi
		x := spawn.X + float32(radius*math.Cos(angle))
		y := spawn.Y + float32(radius*math.Sin(angle))
		heading := byte(math.Mod(angle/(2*math.Pi)*120+30, 120))
		game.Move(x, y, spawn.Z, heading, client.MoveStartKeyboard, x+1, y+1, spawn.Z)
		if *say != "" && step%8 == 0 {
			game.Say(*say)
		}
		time.Sleep(time.Second)
		game.Move(x, y, spawn.Z, heading, client.MoveStop, 0, 0, 0)
	}
	if err := game.Quit(false); err != nil {
		log.Warn("quitting", "err", err)
	}
	log.Info("left the world")
}

// fightMonster attacks the nearest monster it saw on entering the world until it dies, and logs what the server sent.
func fightMonster(game *client.Game, log *slog.Logger) {
	position, _ := game.Position()
	type monster struct {
		id       int32
		npc      int32
		distance float64
	}
	var nearest *monster
	seen := map[byte]int{}
	settle := time.After(4 * time.Second)
collect:
	for {
		select {
		case p, ok := <-game.Packets:
			if !ok {
				return
			}
			seen[p.Op]++
			if p.Op != client.SmNpcInfo {
				continue
			}
			r := p.Reader()
			x, y, z := r.F(), r.F(), r.F()
			id, npc := r.D(), r.D()
			r.D()
			kind := r.C()
			if kind != 0 && kind != 8 {
				continue
			}
			d := math.Sqrt(float64((x-position.X)*(x-position.X) + (y-position.Y)*(y-position.Y) + (z-position.Z)*(z-position.Z)))
			if nearest == nil || d < nearest.distance {
				nearest = &monster{id, npc, d}
			}
		case <-settle:
			break collect
		}
	}
	if nearest == nil {
		log.Warn("no monster in sight")
		return
	}
	log.Info("fighting", "object", nearest.id, "npc", nearest.npc, "distance", nearest.distance)
	game.Target(nearest.id)
	counts := map[byte]int{}
	died := false
	deadline := time.After(2 * time.Minute)
	swing := time.NewTicker(2200 * time.Millisecond)
	defer swing.Stop()
	for number := byte(1); !died; {
		select {
		case p, ok := <-game.Packets:
			if !ok {
				return
			}
			counts[p.Op]++
			switch p.Op {
			case 0x32: // SM_ATTACK
				r := p.Reader()
				attacker := r.D()
				r.C()
				r.H()
				r.C()
				target := r.D()
				tp, ap := r.C(), r.C()
				r.H()
				r.C()
				log.Info("attack", "attacker", attacker, "target", target, "target%", tp, "attacker%", ap, "damage", r.D(), "status", int8(r.C()))
			case 0x23: // SM_ATTACK_STATUS
				r := p.Reader()
				log.Info("status", "object", r.D(), "value", r.D(), "kind", r.C(), "hp%", r.C())
			case 0xbf:
				log.Info("the bot died")
				return
			}
			if p.Op == client.SmEmotion {
				r := p.Reader()
				if r.D() == nearest.id && r.C() == 16 {
					died = true
				}
			}
		case <-swing.C:
			if number%3 == 0 {
				game.Cast(1351, nearest.id) // Flame Bolt, if the character has it
			} else {
				game.Attack(nearest.id, number)
			}
			number++
		case <-deadline:
			log.Warn("the monster didn't die", "packets", counts)
			return
		}
	}
	log.Info("the monster died", "packets", counts)

	// Loot it: open the list, take every item, close.
	game.OpenLoot(nearest.id, false)
	var indexes []byte
	listen := time.After(3 * time.Second)
wait:
	for {
		select {
		case p, ok := <-game.Packets:
			if !ok {
				return
			}
			if p.Op == 0xea { // SM_LOOT_ITEMLIST
				r := p.Reader()
				r.D()
				for range r.C() {
					index := r.C()
					item, count := r.D(), r.H()
					r.D()
					log.Info("loot", "index", index, "item", item, "count", count)
					indexes = append(indexes, index)
				}
				break wait
			}
		case <-listen:
			break wait
		}
	}
	for _, index := range indexes {
		game.TakeLoot(nearest.id, index)
		time.Sleep(300 * time.Millisecond)
	}
	game.OpenLoot(nearest.id, true)
	time.Sleep(time.Second)
}
