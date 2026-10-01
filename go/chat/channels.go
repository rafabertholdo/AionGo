package chat

import (
	"fmt"
	"unicode/utf16"
)

// Channel kinds; a player is in at most one channel of each.
type channelKind int

const (
	public channelKind = iota
	trade
	group
	job
)

// channel is one of the fixed chat channels the client asks to join by name.
type channel struct {
	id   int32
	kind channelKind
	// name is how the client identifies it, in UTF-16LE.
	name []byte
}

// Region codes the client uses in channel names, per map.
var regions = []string{
	"lf1",  // Poeta
	"LF1A", // Verteron
	"LC1",  // Sanctum
	"lf2",  // Eltnen
	"lf2a", // Theobomos
	"LF3",  // Heiron
	"df1",  // Ishalgen
	"df2",  // Morheim
	"DC1",  // Pandaemonium
	"DF3",  // Beluslan
	"DF1A", // Altgard
	"DF2A", // Brusthonin
	"Ab1",  // Abyss (Reshanta)
}

var jobs = []string{"Gladiator", "Templar", "Sorcerer", "Spiritmaster", "Chanter", "Ranger", "Assassin", "Cleric"}

// newChannels is AL-CServer's channel list for game server id: for each race
// (0 Elyos, 1 Asmodians), looking for group, trade and public chat in every
// region, and a channel per class. Ids count from 1 in that order.
func newChannels(gameServerID byte) []*channel {
	var channels []*channel
	add := func(kind channelKind, name string, race int) {
		full := fmt.Sprintf("@\x01%s\x01%d.%d.AION.KOR", name, gameServerID, race)
		channels = append(channels, &channel{id: int32(len(channels) + 1), kind: kind, name: utf16LE(full)})
	}
	for race := range 2 {
		add(group, "partyFind_PF", race)
	}
	for race := range 2 {
		for _, region := range regions {
			add(trade, "trade_"+region, race)
		}
	}
	for race := range 2 {
		for _, region := range regions {
			add(public, "public_"+region, race)
		}
	}
	for _, class := range jobs {
		for race := range 2 {
			add(job, "job_"+class, race)
		}
	}
	return channels
}

func utf16LE(s string) []byte {
	units := utf16.Encode([]rune(s))
	b := make([]byte, 0, len(units)*2)
	for _, u := range units {
		b = append(b, byte(u), byte(u>>8))
	}
	return b
}

func decodeUTF16LE(b []byte) string {
	units := make([]uint16, len(b)/2)
	for i := range units {
		units[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(units))
}
