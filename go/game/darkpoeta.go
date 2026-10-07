package game

import (
	"time"

	"aionlightning/wire"
)

// Dark Poeta (300040000): the timed, scored instance (beyond-aion's DarkPoetaInstance, as AL-Game 2.x sends it).
// A minute to prepare, then four hours that start when it runs out or someone opens a door. Kills score points;
// the three generators bring out Brigade General Anuhart, and his death ends the run: the points and the time left
// give its rank, and the rank's boss appears.

const (
	darkPoetaWorld = 300040000

	smInstanceScore = 0x77    // SM_INSTANCE_SCORE (2.x numbering 0x79)
	msgGetScore     = 1400237 // STR_MSG_GET_SCORE: "You have gained %num1 points from %0."

	// InstanceScoreType ids.
	scorePreparing = 0x100000
	scoreStarted   = 0x200000
	scoreEnded     = 0x300000

	darkPoetaPrepare = time.Minute
	darkPoetaRun     = 4 * time.Hour

	npcAnuhart = 214904
	rankFailed = 8 // shown as F, with the failure sound
)

// darkPoetaHeld are npcs of the map's spawns that only the run brings out: Anuhart and the rank bosses.
var darkPoetaHeld = map[int32]bool{npcAnuhart: true, 215280: true, 215281: true, 215282: true, 215283: true, 215284: true}

// darkPoetaRanks are checkRank's thresholds, best first: more time left than left, at least points, and the boss.
var darkPoetaRanks = []struct {
	left   time.Duration
	points int32
	boss   int32
}{
	{2 * time.Hour, 17817, 215280},    // S: Tahabata Pyrelord
	{90 * time.Minute, 15219, 215281}, // A: Calindi Flamelord
	{time.Hour, 10914, 215282},        // B: Vanuka Infernus
	{30 * time.Minute, 6657, 215283},  // C: Asaratu Bloodshade
	{time.Millisecond, 0, 215284},     // D: Chramati Firetail
}

type darkPoeta struct {
	state                 int32
	since                 time.Time     // when the current state began
	left                  time.Duration // the time left when the run ended
	points, kills, gather int32
	rank                  int32
	generators            map[int32]bool
	credited              map[*object]bool
	expire                *task
	start                 *task
}

func (s *Server) newDarkPoeta(in *instance) {
	in.dp = &darkPoeta{state: scorePreparing, since: time.Now()}
	in.dp.start = s.later(darkPoetaPrepare, func() { s.darkPoetaStart(in) })
}

func (d *darkPoeta) timeLeft() time.Duration {
	switch d.state {
	case scorePreparing:
		return max(darkPoetaPrepare-time.Since(d.since), 0)
	case scoreStarted:
		return max(darkPoetaRun-time.Since(d.since), 0)
	}
	return d.left
}

// darkPoetaScore is SM_INSTANCE_SCORE for Dark Poeta.
func darkPoetaScore(d *darkPoeta) *wire.Writer {
	w := wire.Packet(smInstanceScore)
	w.D(darkPoetaWorld)
	w.D(int32(d.timeLeft() / time.Millisecond))
	w.D(d.state)
	w.D(d.points)
	w.D(d.kills)
	w.D(d.gather)
	w.D(d.rank)
	return w
}

func (s *Server) darkPoetaOf(world, inst int32) *instance {
	if world != darkPoetaWorld {
		return nil
	}
	if in := s.instances[[2]int32{world, inst}]; in != nil && in.dp != nil {
		return in
	}
	return nil
}

func (s *Server) darkPoetaSend(in *instance, w *wire.Writer) {
	for _, p := range s.playersIn(in) {
		p.conn.send(w)
	}
}

// darkPoetaEnter is onEnterInstance: the player sees the score.
func (s *Server) darkPoetaEnter(p *player) {
	if in := s.darkPoetaOf(p.WorldID, p.instance); in != nil {
		p.conn.send(darkPoetaScore(in.dp))
	}
}

// darkPoetaDoor is onOpenDoor: opening a door while preparing starts the run.
// ponytail: any door counts (beyond-aion checks door 33); the start door is the only one reachable while preparing.
func (s *Server) darkPoetaDoor(p *player) {
	if in := s.darkPoetaOf(p.WorldID, p.instance); in != nil {
		s.darkPoetaStart(in)
	}
}

func (s *Server) darkPoetaStart(in *instance) {
	if s.instances[[2]int32{in.world, in.id}] != in {
		return
	}
	d := in.dp
	if d.state != scorePreparing {
		return
	}
	d.start.cancel()
	d.state, d.since = scoreStarted, time.Now()
	d.expire = s.later(darkPoetaRun, func() { s.darkPoetaExpire(in) })
	s.darkPoetaSend(in, darkPoetaScore(d))
}

// darkPoetaExpire ends an exhausted run even if nobody sends another event.
func (s *Server) darkPoetaExpire(in *instance) {
	if s.instances[[2]int32{in.world, in.id}] != in || in.dp.state != scoreStarted {
		return
	}
	in.dp.expire.cancel()
	in.dp.left, in.dp.state, in.dp.rank = 0, scoreEnded, rankFailed
	s.darkPoetaSend(in, darkPoetaScore(in.dp))
}

// darkPoetaKill is onDie: the npc's points, and the end of the run when it is Anuhart.
func (s *Server) darkPoetaKill(o *object) {
	in := s.darkPoetaOf(o.worldID, o.instance)
	if in == nil || o.owner != nil {
		return
	}
	d := in.dp
	if d.state != scoreStarted {
		return
	}
	if d.timeLeft() == 0 {
		s.darkPoetaExpire(in)
		return
	}
	if d.credited[o] {
		return
	}
	if d.credited == nil {
		d.credited = map[*object]bool{}
	}
	d.credited[o] = true
	if points := darkPoetaPoints(o); points != 0 {
		d.kills++
		d.points += points
		s.darkPoetaSend(in, systemMessage(msgGetScore, descriptionID(o.npc.NameID*2+1), points))
	}
	switch o.npc.ID {
	case 214895, 214896, 214897: // the generators
		if d.generators == nil {
			d.generators = map[int32]bool{}
		}
		if d.generators[o.npc.ID] {
			break
		}
		d.generators[o.npc.ID] = true
		if len(d.generators) == 3 {
			s.darkPoetaSpawn(in, npcAnuhart, 275.34537, 323.02072, 130.9302, 52)
		}
	case npcAnuhart:
		d.expire.cancel()
		d.left = d.timeLeft()
		d.state = scoreEnded
		var boss int32
		if d.rank, boss = darkPoetaRank(d.left, d.points); boss != 0 {
			s.darkPoetaSpawnRankBoss(in, boss)
		}
	}
	s.darkPoetaSend(in, darkPoetaScore(d))
}

// Use each boss's authored 1.9 spawn. Chramati has no spawn in this data set,
// so retain the existing room placement for that rank until its location is verified.
func (s *Server) darkPoetaSpawnRankBoss(in *instance, boss int32) {
	for _, group := range s.data.Spawns[darkPoetaWorld] {
		if group.NpcID != boss || len(group.Spots) == 0 {
			continue
		}
		spot := group.Spots[0]
		s.darkPoetaSpawn(in, boss, spot.X, spot.Y, spot.Z, byte(spot.Heading))
		return
	}
	s.darkPoetaSpawn(in, boss, 1188.1594, 1241.2809, 142.61472, 34)
}

// darkPoetaRank is checkRank: the run's rank and its boss, none when the time ran out.
func darkPoetaRank(left time.Duration, points int32) (rank, boss int32) {
	for i, r := range darkPoetaRanks {
		if left > r.left && points >= r.points {
			return int32(i + 1), r.boss
		}
	}
	return rankFailed, 0
}

// darkPoetaGather is onGather.
func (s *Server) darkPoetaGather(p *player) {
	if in := s.darkPoetaOf(p.WorldID, p.instance); in != nil {
		if in.dp.state != scoreStarted {
			return
		}
		if in.dp.timeLeft() == 0 {
			s.darkPoetaExpire(in)
			return
		}
		in.dp.gather++
		s.darkPoetaSend(in, darkPoetaScore(in.dp))
	}
}

func (s *Server) darkPoetaSpawn(in *instance, npcID int32, x, y, z float32, heading byte) {
	t := s.data.Npcs[npcID]
	if t == nil {
		return
	}
	o := &object{id: s.ids.nextID(), worldID: in.world, instance: in.id, x: x, y: y, z: z, heading: heading,
		homeX: x, homeY: y, homeZ: z, npc: t, noRespawn: true,
		interval: 14400} // the map's spawns of them: the corpse, and its loot, stays the usual 4 minutes
	s.initNpc(o)
	s.byID[o.id] = o
	s.addObject(o)
}

// darkPoetaExit: the 1.9 data has one abyss gate npc both for going into Tahabata's room and for leaving it, and its
// portal leads into the room; the gate standing in the room takes the player out of the instance (2.x's exit 730211).
func (s *Server) darkPoetaExit(p *player, gate *object) bool {
	if gate.worldID != darkPoetaWorld || gate.npc.ID != 730211 {
		return false
	}
	dx, dy := gate.x-1188.8, gate.y-1241.4 // the gate's portal exit, in the room
	if dx*dx+dy*dy > 100*100 {
		return false
	}
	s.leaveInstance(p)
	return true
}

// darkPoetaPoints is calculatePointsReward.
// ponytail: no 10% bonus for a group led by an officer; 1.9 has no officer ranks worth checking yet.
func darkPoetaPoints(o *object) int32 {
	switch o.npc.ID {
	case 700520: // Drana
		return 52
	case 700517, 700518, 700556, 700558: // walls
		return 156
	case 214885: // Mutated Fungie
		return 21
	case 214841:
		return -209
	case 281116:
		return 1241
	case 215431:
		return 208
	case 215429, 215430:
		return 190
	case 214842, 215432:
		return 357
	case 214871, 215386, 215428:
		return 204
	case 214849, 214850, 214851: // Marabata
		return 319
	case 214895, 214896:
		return 377
	case 214897:
		return 330
	case 214843: // Atmach
		return 456
	case 214864, 214880, 214894, 215387, 215388, 215389:
		return 789
	case npcAnuhart:
		return 954
	case 281178: // drana lump
		return 0
	}
	if o.npc.Rank == "HERO" {
		if o.npc.HPGauge == 21 {
			return 786
		}
		return 300
	}
	switch o.npc.Race {
	case "":
		return 0
	case "UNDEAD":
		return 12
	case "BROWNIE":
		return 18
	case "LIZARDMAN":
		return 24
	case "NAGA", "DRAGON", "MAGICALMONSTER":
		return 30
	}
	return 11
}
