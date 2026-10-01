package game

import (
	"testing"

	"aionlightning/client"
)

// The test client keeps its own copies of the opcodes it uses.
func TestClientOpcodesMatchTheServers(t *testing.T) {
	for name, pair := range map[string][2]byte{
		"SM_MESSAGE":            {client.SmMessage, smMessage},
		"SM_PLAYER_INFO":        {client.SmPlayerInfo, smPlayerInfo},
		"SM_PLAYER_SPAWN":       {client.SmPlayerSpawn, smPlayerSpawn},
		"SM_QUIT_RESPONSE":      {client.SmQuitResponse, smQuitResponse},
		"SM_KEY":                {client.SmKey, smKey},
		"SM_CHARACTER_LIST":     {client.SmCharacterList, smCharacterList},
		"SM_L2AUTH_LOGIN_CHECK": {client.SmL2authLoginCheck, smL2authLoginCheck},
		"SM_CREATE_CHARACTER":   {client.SmCreateCharacter, smCreateCharacter},
	} {
		if pair[0] != pair[1] {
			t.Errorf("%s: the client has %#x, the server %#x", name, pair[0], pair[1])
		}
	}
}
