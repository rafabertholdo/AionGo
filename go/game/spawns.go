package game

import (
	"cmp"
	"maps"
	"math"
	"slices"
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// object is an npc or a gatherable standing in a map.
type object struct {
	spawnGroup  *data.SpawnGroup
	spawnSpot   data.Spot
	id          int32
	worldID     int32
	x, y, z     float32
	heading     byte
	staticID    int32
	transformed int32 // the npc model it looks like under a transform effect, or 0
	npc         *data.NpcTemplate
	gatherable  *data.GatherableTemplate

	// An npc's: where it spawned and how often, how it stands and fights.
	homeX, homeY, homeZ     float32
	interval                int32 // respawn, in seconds
	walker, randomWalk      int32 // the route it walks, or the area it wanders
	stats                   *gameStats
	maxHP, hp               int32
	dead                    bool
	noRespawn               bool
	state                   uint16
	targetID                int32
	attackCounter           int32
	aggro                   map[int32]*aggroInfo
	watchers                map[int32]*player // those who see it
	ai                      *npcAI
	move                    mover
	restore, decay, respawn *task
	loot                    *lootState   // what it left when it died
	useTask                 *task        // a player collecting a quest action item
	gathering               *interaction // a player gathering it
	owner                   *player      // who made it, if it is a trap or a servant
	kisk                    *kisk        // what it is besides, if it is a kisk
	rift                    *rift        // what it is besides, if it is a rift
	objectType              uint16       // NpcObjectType, if it isn't a normal npc
	summonLevel             int32        // its level, if it is a summon
	summonMode              int32        // SummonMode of a summon
	lastAttack              time.Time
	instance                int32   // the instance of its map it stands in, from 0
	timers                  []*task // what a trap or a servant waits on
	gatherCount             int32
	cast                    *skill
	moves                   int32
	fx                      effectController
}

// cell is a square of a map: objects are looked up by the squares around a player.
type cell struct{ world, inst, x, y int32 }

// cellSize is bigger than the visibility distance, so the objects a player
// sees are in the nine squares around it (AL-Game's MapRegions do the same).
const cellSize = 256

// cellOf is the square at a place of the first instance of a map.
func cellOf(world int32, x, y float32) cell { return cellAt(world, 0, x, y) }

// cellAt is the square at a place of an instance of a map.
func cellAt(world, inst int32, x, y float32) cell {
	return cell{world, inst, int32(math.Floor(float64(x) / cellSize)), int32(math.Floor(float64(y) / cellSize))}
}

// Ids of gatherable templates are in this range (SpawnEngine.spawnObject).
func isGatherable(id int32) bool { return id > 400000 && id < 499999 }

// spawnAll is SpawnEngine.spawnAll: every spawn group of every map that isn't an instance
// puts as many of its npcs at its first spots as its pool says.
// ponytail: only the first channel of a map; rifts and static objects (handler groups) wait for
// their milestones; walking, respawning and time-of-day spawns too.
func (s *Server) spawnAll() {
	var npcs, gatherables int
	for _, id := range sortedKeys(s.data.WorldMaps) {
		if s.data.WorldMaps[id].Instance {
			continue
		}
		n, g := s.spawnMap(id, 0)
		npcs, gatherables = npcs+n, gatherables+g
	}
	s.log.Info("spawned", "npcs", npcs, "gatherables", gatherables)
}

// spawnMap puts the spawn groups of a map in an instance of it, and returns how many npcs and gatherables that made.
func (s *Server) spawnMap(id, inst int32) (npcs, gatherables int) {
	for _, group := range s.data.Spawns[id] {
		if group.Handler != "" {
			continue
		}
		for _, spot := range group.Spots[:group.Pool] {
			o := &object{spawnGroup: group, spawnSpot: spot, id: s.ids.nextID(), worldID: id, instance: inst, x: spot.X, y: spot.Y, z: spot.Z,
				heading: byte(spot.Heading), staticID: spot.StaticID, homeX: spot.X, homeY: spot.Y, homeZ: spot.Z,
				interval: group.Interval, walker: spot.Walker, randomWalk: max(spot.Random, group.Random)}
			if isGatherable(group.NpcID) {
				if o.gatherable = s.data.Gatherables[group.NpcID]; o.gatherable == nil {
					s.ids.release(o.id)
					continue
				}
				gatherables++
			} else {
				if o.npc = s.data.Npcs[group.NpcID]; o.npc == nil {
					s.ids.release(o.id)
					continue
				}
				npcs++
			}
			if o.npc != nil {
				s.initNpc(o)
			}
			s.byID[o.id] = o
			c := cellAt(id, inst, o.x, o.y)
			s.grid[c] = append(s.grid[c], o)
			if o.npc != nil {
				o.ai.handleEvent(evRespawned)
			}
		}
	}
	return npcs, gatherables
}

// inRange is KnownList.checkObjectInRange for a player and an object.
func (o *object) inRange(p *player) bool {
	if o.worldID != p.WorldID || o.instance != p.instance || math.Abs(float64(o.z-p.Z)) > visibilityDistance {
		return false
	}
	dx, dy := o.x-p.X, o.y-p.Y
	return dx*dx+dy*dy < visibilityDistance*visibilityDistance
}

// NpcType ids the client is sent (NpcType.getId).
var npcTypeIDs = map[string]byte{
	"ATTACKABLE": 0, "AGGRESSIVE": 8, "NON_ATTACKABLE": 38, "RESURRECT": 38, "POSTBOX": 38,
	"USEITEM": 38, "PORTAL": 38, "ARTIFACT": 38, "ARTIFACT_PROTECTOR": 0,
}

const (
	stateNpcIdle = 1 << 6
	npcTypeAggro = 8
	npcObjectID  = 1 // NpcObjectType.NORMAL
)

// npcInfo is SM_NPC_INFO: an npc as a player sees it.
func (s *Server) npcInfo(o *object, p *player) *wire.Writer {
	t := o.npc
	kind := npcTypeIDs[t.Type]
	race, _ := raceGender(p.Character)
	if s.data.Tribes.AggroIcon(race == 1, t.Tribe) {
		kind = npcTypeAggro
	}
	if o.kisk != nil && o.kisk.ownerRace != p.Race {
		kind = npcTypeIDs["ATTACKABLE"]
	}
	state := uint16(stateActive | stateNpcIdle)
	if t.State != 0 {
		state = uint16(t.State)
	}
	maxHP := npcMaxHP(t)
	w := wire.Packet(smNpcInfo)
	w.F(o.x)
	w.F(o.y)
	w.F(o.z)
	w.D(o.id)
	w.D(t.ID)
	w.D(t.ID)
	w.C(kind)
	w.H(state)
	w.C(o.heading)
	w.D(t.NameID)
	w.D(t.TitleID)
	w.H(0)
	w.C(0)
	w.D(0)
	if o.owner != nil {
		w.D(o.owner.ID)
		w.S(o.owner.Name)
	} else {
		w.D(0) // master
		w.S("")
	}
	w.C(100) // %hp
	w.D(maxHP)
	w.C(byte(o.clevel()))
	mask, gear := s.npcGear(t)
	w.H(mask)
	for _, item := range gear {
		w.D(item)
		w.D(0)
		w.D(0)
		w.H(0)
	}
	w.F(1.5)
	w.F(t.Height)
	w.F(0) // speed: an npc at rest has none; one that walks has its template's walk speed
	w.H(2000)
	w.H(2000)
	w.C(0)
	w.F(o.x)
	w.F(o.y)
	w.F(o.z)
	w.C(0) // move type
	w.H(uint16(o.staticID))
	w.B(make([]byte, 8))
	w.C(0) // visual state
	objectType := uint16(npcObjectID)
	if o.objectType != 0 {
		objectType = o.objectType
	}
	w.H(objectType)
	w.C(0)
	w.D(0) // target
	return w
}

// npcGear is NpcEquippedGear: each item of the template in the first free slot it fits, by slot,
// and the mask of the slots taken (a short, as AL-Game sends it).
func (s *Server) npcGear(t *data.NpcTemplate) (uint16, []int32) {
	var slots [31]int32
	var mask uint16
	for _, id := range t.Equipment {
		item := s.data.Items[id]
		if item == nil {
			continue
		}
		for bit := range 31 {
			if item.Slot&(1<<bit) == 0 || slots[bit] != 0 {
				continue
			}
			slots[bit] = id
			mask |= uint16(1 << bit)
			break
		}
	}
	var gear []int32
	for _, id := range slots {
		if id != 0 {
			gear = append(gear, id)
		}
	}
	return mask, gear
}

// gatherableInfo is SM_GATHERABLE_INFO.
func gatherableInfo(o *object) *wire.Writer {
	w := wire.Packet(smGatherableInfo)
	w.F(o.x)
	w.F(o.y)
	w.F(o.z)
	w.D(o.id)
	w.D(o.staticID)
	w.D(o.gatherable.ID)
	w.H(1)
	w.C(0)
	w.D(o.gatherable.NameID)
	w.H(0)
	w.H(0)
	w.H(0)
	w.C(100)
	return w
}

func sortedKeys[V any](m map[int32]V) []int32 {
	return slices.SortedFunc(maps.Keys(m), cmp.Compare[int32])
}

// npcMaxHP is NpcGameStats' maximum HP: the template's, and its HP gauge's share of the level.
func npcMaxHP(t *data.NpcTemplate) int32 {
	return t.Stats.MaxHP + data.Round(float32(float32(t.HPGauge)*1.5)*float32(t.Level))
}
