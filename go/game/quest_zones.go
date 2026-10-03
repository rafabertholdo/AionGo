package game

import "aionlightning/game/data"

// questZoneHandler is a custom quest's Java onEnterZoneEvent registration.
// Keep the map and zone together: zone names alone are not unique across maps.
type questZoneHandler struct {
	mapID  int32
	name   string
	handle func(*conn, string) bool
}

var questZoneHandlers = []questZoneHandler{
	{mapID: 210010000, name: "AKARIOS_VILLAGE", handle: (*conn).kaliosCallEnterZone},
	{mapID: 210010000, name: "Q1123", handle: (*conn).wheresTuttyEnterZone},
	{mapID: 210030000, name: "Q1012", handle: (*conn).maskedLoiterersEnterZone},
	{mapID: 210030000, name: "MYSTERIOUS_SHIPWRECK", handle: (*conn).aNestOfLepharistsEnterZone},
	{mapID: 210030000, name: "TURSIN_GARRISON", handle: (*conn).reducingTursinStrengthEnterZone},
	{mapID: 210030000, name: "VERTERON_CITADEL", handle: (*conn).summonsToCitadelEnterZone},
	{mapID: 210020000, name: "ELTNEN_FORTRESS", handle: (*conn).ordersFromTelemachusEnterZone},
	{mapID: 220010000, name: "ALDELLE_VILLAGE", handle: (*conn).orderOfTheCaptainEnterZone},
	{mapID: 220030000, name: altgardDutiesZone, handle: (*conn).altgardDutiesEnterZone},
	{mapID: 220030000, name: dangerousCropZone, handle: (*conn).dangerousCropEnterZone},
	{mapID: 220020000, name: morheimCommandersCallZone, handle: (*conn).morheimCommandersCallEnterZone},
	{mapID: 210060000, name: "Q1091", handle: (*conn).atroposRequestEnterZone},
}

// enterQuestZone runs only when the selected zone changes. Both map refreshes
// and ordinary movement can enter a quest zone in the Java server.
func (s *Server) enterQuestZone(p *player, previous, current *data.Zone) {
	if current == nil || current == previous || p.conn == nil {
		return
	}
	for _, handler := range questZoneHandlers {
		if handler.mapID == p.WorldID && handler.name == current.Name {
			handler.handle(p.conn, current.Name)
		}
	}
	p.conn.javaEnterZone(current.Name)
}
