package game

import (
	"bytes"
	"encoding/hex"
	"testing"

	"aionlightning/game/data"
	"aionlightning/wire"
)

func TestObjectSearchFirstSpawnPacket(t *testing.T) {
	d := &data.Data{}
	// File order wins over map ID, distance, pool, handler and current world.
	d.AddSpawnGroup(&data.SpawnGroup{Map: 300000000, NpcID: 123456})
	first := &data.SpawnGroup{Map: 300000000, NpcID: 123456, Handler: "quest", Spots: []data.Spot{
		{X: 1.5, Y: -2.25, Z: 3.75}, {X: 99, Y: 99, Z: 99},
	}}
	d.AddSpawnGroup(first)
	d.AddSpawnGroup(&data.SpawnGroup{Map: 100000000, NpcID: 123456, Pool: 1, Spots: []data.Spot{{X: 10}}})
	c := &conn{s: testServer(d), player: &player{}}
	var packets [][]byte
	c.tap = func(w *wire.Writer) { packets = append(packets, bytes.Clone(w.Data)) }
	handler := handlers[cmObjectSearch]
	if handler == nil || clientPacketStates[cmObjectSearch] != inGame {
		t.Fatal("object search must be registered for in-game requests")
	}
	request := wire.Packet(0)
	request.D(123456)
	handler(c, wire.NewReader(request.Data[1:]))
	// Java SM_SHOW_NPC_ON_MAP: opcode, NPC ID, world ID twice, X/Y/Z floats.
	want, err := hex.DecodeString("5740e2010000a3e11100a3e1110000c03f000010c000007040")
	if err != nil {
		t.Fatal(err)
	}
	if len(packets) != 1 || !bytes.Equal(packets[0], want) {
		t.Fatalf("packets %x, want one packet %x", packets, want)
	}
	// Deleting the first spot must expose the next spot, without a stale marker.
	first.Spots = first.Spots[1:]
	handler(c, wire.NewReader(request.Data[1:]))
	r := wire.NewReader(packets[1][1:])
	r.D()
	r.D()
	r.D()
	if x := r.F(); x != 99 {
		t.Fatalf("after deleting first spot, X = %v, want 99", x)
	}
	d.RemoveSpawnGroup(first)
	handler(c, wire.NewReader(request.Data[1:]))
	r = wire.NewReader(packets[2][1:])
	r.D()
	if world := r.D(); world != 100000000 {
		t.Fatalf("after deleting group, world = %d", world)
	}
}

func TestObjectSearchIgnoresMissingAndTruncatedRequests(t *testing.T) {
	d := &data.Data{}
	d.AddSpawnGroup(&data.SpawnGroup{NpcID: 0, Spots: []data.Spot{{}}})
	d.AddSpawnGroup(&data.SpawnGroup{NpcID: 7})
	c := &conn{s: testServer(d), player: &player{}}
	var sent int
	c.tap = func(*wire.Writer) { sent++ }
	for _, id := range []int32{7, 99, -1} {
		request := wire.Packet(0)
		request.D(id)
		c.objectSearch(wire.NewReader(request.Data[1:]))
	}
	for size := range 4 {
		c.objectSearch(wire.NewReader(make([]byte, size)))
	}
	c.player = nil
	c.objectSearch(wire.NewReader(make([]byte, 4)))
	if sent != 0 {
		t.Fatalf("missing, truncated or out-of-world request sent %d packets", sent)
	}
}
