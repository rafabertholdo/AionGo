package game

import (
	"encoding/hex"
	"testing"
)

// The SM_EMOTIONs the 1.9 server sent Wrathchild (testdata/play-session-1.9.txt), rebuilt.
func TestEmotionsMatchClient19(t *testing.T) {
	for _, c := range []struct {
		name string
		got  string
		want string
	}{
		{"sit", emotionHex(emoteSit, stateActive|stateResting, 0, 0), "770501000205000000c040"},
		{"stand", emotionHex(emoteStand, stateActive, 0, 0), "770501000301000000c040"},
		{"weapon out", emotionHex(emoteAttackMode2, stateActive|stateWeapon, 0, 0), "770501002421000000c040"},
		{"weapon away", emotionHex(emoteNeutralMode2, stateActive, 0, 0), "770501002501000000c040"},
		{"jump", emotionHex(emoteJump, stateActive, 0, 0), "770501000101000000c040"},
		{"wave", emotionHex(emoteEmote, stateActive, 4, 0x6d6), "770501001301000000c040d6060000040001"},
		{"start looting", emotionHex(emoteStartLoot, stateWeapon|stateLooting, 0, 0x4f5), "77050100262c000000c040f5040000"},
		{"end looting", emotionHex(emoteEndLoot, stateActive|stateWeapon, 0, 0x4f5), "770501002721000000c040f5040000"},
	} {
		if c.got != c.want {
			t.Errorf("%s: %s, want %s", c.name, c.got, c.want)
		}
	}
}

func emotionHex(kind byte, state uint16, emote int32, target int32) string {
	return hex.EncodeToString(emotionPacket(0x10577, kind, state, 6, emote, target, 0, 0, 0, 0, 0, 0).Data[1:])
}
