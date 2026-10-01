package game

import (
	"math"
	"math/rand/v2"
	"time"

	"aionlightning/game/data"
)

// Zones, weather and the game clock: AL-Game's ZoneService, WeatherService and GameTimeService.
// A player is in the zone of its map that has the highest priority (the lowest number) and holds its position; the
// zone tells whether flying is allowed (CM_EMOTION) and whether it is breathable, and below a map's water level
// (less the player's height) a player in an unbreathable place drowns, and below its death level it dies.
// ponytail: AL-Game checks zones in a 4 second batch, here it's on each move. Entering a zone also dispatches
// registered custom quest handlers; the 1.9 ZoneService sends no SM_ZONE_UPDATE.
// Day and night (GameTimeService.sendDayTimeChangeEvents) has no caller in AL-Game, so it isn't ported.

const (
	msgFlyingForbiddenHere = 1300960 // STR_FLYING_FORBIDDEN_HERE
	drownPeriod            = 2 * time.Second
	drownDivisor           = 10  // a tenth of the maximum life at a time
	heightFactor           = 1.6 // "TODO need fix character height" in PlayerController.checkWaterLevel
	weatherCheckEvery      = 2 * time.Minute
	weatherKinds           = 9
	gameTimeEvery          = 3 * time.Minute
)

// startZoneTasks starts WeatherService's check of expired weathers and GameTimeService's broadcast of the clock.
func (s *Server) startZoneTasks() {
	s.every(weatherCheckEvery, weatherCheckEvery, s.checkWeathers)
	s.every(gameTimeEvery, gameTimeEvery, func() {
		for _, p := range s.spawned {
			p.conn.send(s.gameTimePacket())
		}
	})
}

// checkWeathers is WeatherService.checkWeathersTime: a map whose weather has lasted two hours gets a new one, which
// its players are told.
func (s *Server) checkWeathers() {
	s.mu.Lock()
	var expired []int32
	for world, w := range s.weathers {
		if time.Since(w.since) > weatherDuration {
			expired = append(expired, world)
		}
	}
	s.mu.Unlock()
	for _, world := range expired {
		s.setWeather(world, byte(rand.IntN(weatherKinds)))
	}
}

// setWeather is WeatherService.changeRegionWeather: the map has the weather from now, and its players are told.
func (s *Server) setWeather(world int32, code byte) {
	s.mu.Lock()
	if s.weathers == nil {
		s.weathers = map[int32]weatherState{}
	}
	s.weathers[world] = weatherState{code: code, since: time.Now()}
	s.mu.Unlock()
	for _, p := range s.spawned {
		if p.WorldID == world {
			p.conn.send(weather(code))
		}
	}
}

// refreshZone is ZoneService.findZoneInCurrentMap (ZONE_REFRESH, after entering a map or teleporting in one): the
// first zone by priority that holds the player. No zone holding it leaves it in none.
func (s *Server) refreshZone(p *player) {
	previous := p.zone
	p.zone = nil
	for _, z := range s.data.Zones[p.WorldID] {
		if z.Contains(p.X, p.Y, p.Z) {
			p.zone = z
			break
		}
	}
	s.enterQuestZone(p, previous, p.zone)
}

// updateZone is ZoneService.checkZone and PlayerController.checkWaterLevel (ZONE_UPDATE, on each move): the player
// changes zone when it walks into one linked to its own.
func (s *Server) updateZone(p *player) {
	if p.zone == nil {
		s.refreshZone(p)
	} else {
		previous := p.zone
		for _, z := range p.zone.Neighbors {
			if z.Contains(p.X, p.Y, p.Z) {
				p.zone = z
				break
			}
		}
		s.enterQuestZone(p, previous, p.zone)
	}
	s.checkWaterLevel(p)
}

// checkWaterLevel is PlayerController.checkWaterLevel: below the map's death level the player dies, and in water
// it drowns, losing a tenth of its life every two seconds, unless its zone is breathable.
func (s *Server) checkWaterLevel(p *player) {
	m := s.data.WorldMaps[p.WorldID]
	if p.dead || m == nil {
		return
	}
	if p.Z < m.DeathLevel {
		s.reducePlayerHP(p, p.life.HP, nil)
		return
	}
	if p.zone != nil && p.zone.Breath {
		return
	}
	var height float32
	if p.appearance != nil {
		height = p.appearance.Height
	}
	if p.Z < m.WaterLevel-height*heightFactor {
		s.startDrowning(p)
	} else {
		p.drowning.cancel()
		p.drowning = nil
	}
}

// startDrowning is ZoneService.startDrowning.
func (s *Server) startDrowning(p *player) {
	if p.drowning != nil {
		return
	}
	p.drowning = s.every(0, drownPeriod, func() {
		if p.dead || !p.spawned {
			p.drowning.cancel()
			p.drowning = nil
			return
		}
		s.reducePlayerHP(p, int32(math.Round(float64(p.stats.current(data.MaxHP)/drownDivisor))), nil)
	})
}

// flightAllowed is what CM_EMOTION asks of the player's zone: none, or one that says so, allows flying.
func (p *player) flightAllowed() bool { return p.zone == nil || p.zone.Fly }
