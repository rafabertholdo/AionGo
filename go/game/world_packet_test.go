package game

import (
	"math"
	"testing"

	"aionlightning/game/store"
	"aionlightning/wire"
)

func TestMovementRejectsInvalidPacketsBeforeMutation(t *testing.T) {
	for _, handler := range []struct {
		name string
		run  func(*conn, *wire.Reader)
		op   byte
	}{
		{"player", (*conn).move, cmMove},
		{"summon", (*conn).summonMove, cmSummonMove},
		{"flight", (*conn).flightTeleport, cmFlightTeleport},
	} {
		t.Run(handler.name, func(t *testing.T) {
			for _, tc := range []struct {
				name  string
				value float32
			}{
				{"nan", float32(math.NaN())},
				{"positive_infinity", float32(math.Inf(1))},
				{"negative_infinity", float32(math.Inf(-1))},
			} {
				t.Run(tc.name, func(t *testing.T) {
					for axis := range 3 {
						coordinates := [3]float32{10, 20, 30}
						coordinates[axis] = tc.value
						w := wire.Packet(handler.op)
						if handler.op != cmMove {
							w.D(1)
						}
						for _, v := range coordinates {
							w.F(v)
						}
						w.C(0)
						if handler.op == cmFlightTeleport {
							w.D(123)
						} else {
							w.C(moveStop)
						}
						assertMovementPacketRejected(t, handler.run, w.Data[1:])
					}
				})
			}
			// Every prefix of a full fixed-size stop/update packet is truncated.
			w := wire.Packet(handler.op)
			if handler.op != cmMove {
				w.D(1)
			}
			w.F(10)
			w.F(20)
			w.F(30)
			w.C(0)
			if handler.op == cmFlightTeleport {
				w.D(123)
			} else {
				w.C(moveStop)
			}
			for n := 0; n < len(w.Data)-1; n++ {
				assertMovementPacketRejected(t, handler.run, w.Data[1:1+n])
			}
		})
	}
}

func TestMovementRejectsNonFiniteDestinations(t *testing.T) {
	for _, handler := range []struct {
		name string
		run  func(*conn, *wire.Reader)
		op   byte
	}{
		{"player", (*conn).move, cmMove},
		{"summon", (*conn).summonMove, cmSummonMove},
	} {
		t.Run(handler.name, func(t *testing.T) {
			for _, kind := range []byte{moveStartMouse, moveStartKeyboard, moveGlideDown, moveGlideStartMouse} {
				if handler.op == cmSummonMove && kind != moveStartMouse && kind != moveStartKeyboard {
					continue
				}
				for axis := range 3 {
					to := [3]float32{10, 20, 30}
					to[axis] = float32(math.NaN())
					w := wire.Packet(handler.op)
					if handler.op == cmSummonMove {
						w.D(1)
					}
					w.F(10)
					w.F(20)
					w.F(30)
					w.C(0)
					w.C(kind)
					for _, v := range to {
						w.F(v)
					}
					if kind == moveGlideDown || kind == moveGlideStartMouse {
						w.C(0)
					}
					assertMovementPacketRejected(t, handler.run, w.Data[1:])
				}
			}
		})
	}
}

func assertMovementPacketRejected(t *testing.T, run func(*conn, *wire.Reader), input []byte) {
	t.Helper()
	// A live player with no server: an invalid request must return before world
	// access, timer changes, zone updates or any network broadcasts.
	p := &player{character: &character{Character: &store.Character{X: 1, Y: 2, Z: 3}},
		spawned: true, state: stateFlying, flightTeleportID: 1, flightDistance: 7}
	c := &conn{player: p}
	run(c, wire.NewReader(input))
	if p.X != 1 || p.Y != 2 || p.Z != 3 || p.flightDistance != 7 || p.moves != 0 {
		t.Fatal("invalid movement changed player state")
	}
}
