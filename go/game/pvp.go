package game

import (
	"math"
	"strconv"

	"aionlightning/wire"
)

// Killing players of the other race: abyss points for the killers, and the loss of some for the killed
// (AL-Game's PvpService, PlayerCommonData.addAp and StatFunctions).
// ponytail: a killer's count of kills of one victim isn't reset daily, and the skills that come with the top abyss
// ranks, the legion's contribution points and alliances aren't ported.

const (
	msgEarnedAP      = 1320000
	maxDailyPvpKills = 5 // custom.properties pvp.maxkills
)

// abyssRanks are AbyssRankEnum's points gained and lost by rank, from rank 1.
var abyssRanks = [][2]int32{{120, 24}, {168, 37}, {235, 58}, {329, 91}, {461, 143}, {645, 225}, {903, 356}, {1264, 561}, {1770, 885},
	{2124, 1195}, {2549, 1616}, {3059, 2184}, {3671, 2949}, {4405, 3981}, {5286, 5374}, {6343, 7258}, {7612, 9799}, {9134, 13229}}

// addAP is PlayerCommonData.addAp: the player gains abyss points, or loses them if value is negative.
func (s *Server) addAP(p *player, value int32) {
	p.conn.send(systemMessage(msgEarnedAP, strconv.Itoa(int(value))))
	a := p.abyss
	a.DailyAP = max(a.DailyAP+value, 0)
	a.WeeklyAP = max(a.WeeklyAP+value, 0)
	a.AP = max(a.AP+value, 0)
	old := a.Rank
	for rank, required := range abyssRankAP {
		if a.AP >= required {
			a.Rank = int32(rank + 1)
		}
	}
	a.MaxRank = max(a.MaxRank, a.Rank)
	if a.Rank != old {
		w := wire.Packet(smAbyssRankUpdate)
		w.D(p.ID)
		w.D(a.Rank)
		p.broadcast(w, true)
	}
	p.conn.send(abyssRank(a))
	if err := s.skillDB.SaveAbyssRank(p.ID, a); err != nil {
		s.log.Error("saving the abyss rank", "err", err)
	}
}

// pvpApGained is StatFunctions.calculatePvpApGained.
func pvpApGained(victim *player, maxRank, maxLevel int32) int32 {
	round := func(f float32) int32 { return int32(math.Floor(float64(f) + 0.5)) }
	points := abyssRanks[victim.abyss.Rank-1][0]
	switch diff := maxLevel - int32(victim.level); {
	case diff > 4:
		points = round(float32(points) * 0.1)
	case diff < -3:
		points = round(float32(points) * 1.3)
	case diff == 3:
		points = round(float32(points) * 0.85)
	case diff == 4:
		points = round(float32(points) * 0.65)
	case diff == -2:
		points = round(float32(points) * 1.1)
	case diff == -3:
		points = round(float32(points) * 1.2)
	}
	if diff := maxRank - victim.abyss.Rank; maxRank <= 7 && diff > 0 {
		points -= round(float32(points) * float32(diff) * 0.05)
	}
	return points
}

// pvpApLost is StatFunctions.calculatePvPApLost.
func pvpApLost(victim, winner *player) int32 {
	round := func(f float32) int32 { return int32(math.Floor(float64(f) + 0.5)) }
	points := abyssRanks[victim.abyss.Rank-1][1]
	switch diff := int32(winner.level) - int32(victim.level); {
	case diff > 4:
		points = round(float32(points) * 0.1)
	case diff == 3:
		points = round(float32(points) * 0.85)
	case diff == 4:
		points = round(float32(points) * 0.65)
	}
	return points
}

// recordDamage remembers who hurt a player, for the reward its killers get.
func (p *player) recordDamage(attacker creature, damage int32) {
	var who *player
	switch a := attacker.(type) {
	case *player:
		who = a
	case *object:
		who = a.owner
	}
	if who == nil || who == p {
		return
	}
	if p.hurtBy == nil {
		p.hurtBy = map[int32]int32{}
	}
	p.hurtBy[who.ID] += damage
}

// pvpReward is PvpService.doReward: those who hurt the dead player share the abyss points, by the damage they did, unless
// they are of its race.
func (s *Server) pvpReward(victim *player) {
	hurt := victim.hurtBy
	victim.hurtBy = nil
	var total, best int32
	var winner *player
	for id, damage := range hurt {
		total += damage
		if p := s.spawned[id]; p != nil && damage > best {
			winner, best = p, damage
		}
	}
	if total == 0 || winner == nil {
		return
	}
	if s.pvpKills[[2]int32{winner.ID, victim.ID}] < maxDailyPvpKills {
		winner.abyss.AllKill++
		winner.abyss.DailyKill++
		winner.abyss.WeeklyKill++
	}
	// A group's damage counts once, for the group.
	shares := map[*group]int32{}
	var dealt int32
	for id, damage := range hurt {
		p := s.spawned[id]
		if p == nil || p.Race == victim.Race {
			continue
		}
		dealt += damage
		if p.group != nil {
			shares[p.group] += damage
			continue
		}
		s.rewardKiller(victim, []*player{p}, damage, total, p.abyss.Rank, int32(p.level))
	}
	for g, damage := range shares {
		var near []*player
		maxRank, maxLevel := int32(1), int32(0)
		for _, m := range g.members {
			if !m.dead && inRange3D(m, victim, groupMaxDistance) {
				near = append(near, m)
				maxLevel = max(maxLevel, int32(m.level))
				maxRank = max(maxRank, m.abyss.Rank)
			}
		}
		if len(near) > 0 {
			s.rewardKiller(victim, near, damage, total, maxRank, maxLevel)
		}
	}
	if lost := pvpApLost(victim, winner) * dealt / total; lost > 0 {
		s.addAP(victim, -lost)
	}
}

// rewardKiller gives the killers of a player, one or a group's members, their share of the abyss points.
func (s *Server) rewardKiller(victim *player, killers []*player, damage, total, maxRank, maxLevel int32) {
	if s.pvpKills == nil {
		s.pvpKills = map[[2]int32]int{}
	}
	base := int32(1)
	if s.pvpKills[[2]int32{killers[0].ID, victim.ID}] < maxDailyPvpKills {
		base = pvpApGained(victim, maxRank, maxLevel)
	}
	each := int32(math.Floor(float64(float32(base)*float32(damage)/float32(total)/float32(len(killers))) + 0.5))
	for _, k := range killers {
		gain := int32(1)
		if s.pvpKills[[2]int32{k.ID, victim.ID}] < maxDailyPvpKills {
			gain = each
		}
		s.addAP(k, gain)
		s.pvpKills[[2]int32{k.ID, victim.ID}]++
	}
}
