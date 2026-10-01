package main

// field is a known region of a packet body (offsets exclude the 3-byte header).
type field struct {
	offset, length int
	name, kind     string // kind: id, coord, time
}

// fields are the layouts checked against captures; add an opcode when a diff
// points at bytes worth naming or masking.
var fields = map[string][]field{
	"SM_KEY":                {{0, 4, "key", "time"}},
	"SM_VERSION_CHECK":      {{1, 1, "gameServerId", ""}, {26, 6, "serverTime", "time"}},
	"SM_GAME_TIME":          {{0, 4, "gameTime", "time"}},
	"SM_WEATHER":            {{0, 3, "weather", "time"}},
	"SM_TIME_CHECK":         {{0, 8, "time", "time"}},
	"SM_NPC_INFO":           {{0, 4, "x", "coord"}, {4, 4, "y", "coord"}, {8, 4, "z", "coord"}, {12, 4, "objectId", "id"}, {16, 4, "npcTemplateId", ""}},
	"SM_GATHERABLE_INFO":    {{0, 4, "x", "coord"}, {4, 4, "y", "coord"}, {8, 4, "z", "coord"}, {12, 4, "objectId", "id"}},
	"SM_PLAYER_SPAWN":       {{12, 4, "x", "coord"}, {16, 4, "y", "coord"}, {20, 4, "z", "coord"}},
	"SM_MOVE":               {{0, 4, "objectId", "id"}},
	"SM_EMOTION":            {{0, 4, "objectId", "id"}},
	"SM_DELETE":             {{0, 4, "objectId", "id"}},
	"SM_ATTACK_STATUS":      {{0, 4, "targetId", "id"}},
	"SM_TARGET_SELECTED":    {{0, 4, "targetId", "id"}},
	"SM_DIALOG_WINDOW":      {{0, 4, "npcObjectId", "id"}, {4, 4, "dialogId", ""}, {8, 4, "questId", ""}},
	"SM_QUEST_ACCEPTED":     {{0, 4, "questId", ""}},
	"SM_SYSTEM_MESSAGE":     {{0, 4, "messageId", ""}},
	"SM_NEARBY_QUESTS":      {{0, 2, "count", ""}},
	"SM_STATUPDATE_EXP":     {{0, 8, "exp", ""}},
	"SM_PLAYER_INFO":        {{0, 4, "x", "coord"}, {4, 4, "y", "coord"}, {8, 4, "z", "coord"}, {12, 4, "objectId", "id"}},
	"SM_L2AUTH_LOGIN_CHECK": {},
}

func fieldHint(op string, start, end int) string {
	for _, f := range fields[op] {
		if start < f.offset+f.length && end > f.offset {
			return " [" + f.name + "]"
		}
	}
	return ""
}
