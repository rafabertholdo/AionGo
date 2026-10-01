package game

import (
	"bufio"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

var staticData = os.Getenv("AION_DATA")

var (
	loadOnce   sync.Once
	loadedData *data.Data
	loadErr    error
)

func staticDataOrSkip(t *testing.T) *data.Data {
	t.Helper()
	if staticData == "" {
		t.Skip("AION_DATA is not set")
	}
	if _, err := os.Stat(staticData); err != nil {
		t.Skipf("static data not available at %s: %v", staticData, err)
	}
	loadOnce.Do(func() { loadedData, loadErr = data.Load(staticData) })
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	return loadedData
}

// javaPackets is the payload of each server packet AL-Game sent entering the
// world with Wrathchild (testdata/java-enter-world.txt, from cmd/gamesniff),
// first of each kind.
func javaPackets(t *testing.T) map[string][]string {
	t.Helper()
	f, err := os.Open("testdata/java-enter-world.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	packets := map[string][]string{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 4 && fields[0] == "server" {
			packets[fields[1]] = append(packets[fields[1]], fields[3])
		}
	}
	return packets
}

// wrathchild is the level 1 mage the capture entered the world with, as the database had her.
func wrathchild(s *Server) *player {
	ch := &character{Character: &store.Character{ID: 0x10577, Name: "Wrathchild", Class: "MAGE", Race: "ELYOS",
		Gender: "FEMALE", Exp: 206, WorldID: 210010000, TitleID: -1, X: 1142.48, Y: 1032.39, Z: 129.06}}
	p := &player{character: ch, state: stateActive, level: 1, life: store.LifeStats{HP: 132, MP: 452, FP: 60},
		settings: &store.Settings{}, abyss: &store.AbyssRank{Rank: 1, MaxRank: 1}, stones: map[int32][]store.Stone{}}
	p.equipment = []*store.Item{
		{UniqueID: 0x10579, ItemID: 100600034, Count: 1, Equipped: true, Slot: 1},
		{UniqueID: 0x1057a, ItemID: 110100009, Count: 1, Equipped: true, Slot: 8},
		{UniqueID: 0x1057b, ItemID: 113100005, Count: 1, Equipped: true, Slot: 0x1000},
	}
	for _, id := range []int32{4, 64, 67, 1351, 1373, 1801, 1803, 30001} {
		p.skills = append(p.skills, store.Skill{ID: id, Level: 1})
	}
	p.fx = newEffectController(p)
	p.cube = make([]*store.Item, 7)
	p.stats = s.playerStats(p)
	return p
}

func TestStatsMatchJava(t *testing.T) {
	d := staticDataOrSkip(t)
	java := javaPackets(t)
	s := &Server{data: d, clockStart: time.Now()}
	p := wrathchild(s)
	got := hex.EncodeToString(s.statsInfo(p).Data[1:])
	want := java["SM_STATS_INFO"][0]
	// The game time, at bytes 4…8, is whatever the clock said.
	got = got[:8] + want[8:16] + got[16:]
	if got != want {
		t.Errorf("SM_STATS_INFO differs from AL-Game's:\n%s", diffHex(got, want))
	}
}

func TestEquipmentMatchesJava(t *testing.T) {
	d := staticDataOrSkip(t)
	java := javaPackets(t)
	s := &Server{data: d, clockStart: time.Now()}
	p := wrathchild(s)
	got := hex.EncodeToString(s.inventoryInfo(p, p.equipment).Data[1:])
	if want := java["SM_INVENTORY_INFO"][0]; got != want {
		t.Errorf("equipment differs from AL-Game's:\n%s", diffHex(got, want))
	}
}

func TestEnterWorldPacketsMatchJava(t *testing.T) {
	d := staticDataOrSkip(t)
	java := javaPackets(t)
	s := &Server{data: d, clockStart: time.Now()}
	p := wrathchild(s)
	for name, w := range map[string]*wire.Writer{
		"SM_SKILL_LIST":          skillList(p),
		"SM_RECIPE_LIST":         recipeList(p),
		"SM_CUBE_UPDATE":         cubeUpdate(p),
		"SM_SET_BIND_POINT":      s.bindPoint(p),
		"SM_PLAYER_ID":           playerID(p),
		"SM_MACRO_LIST":          macroList(p),
		"SM_TITLE_LIST":          titleList(p),
		"SM_CHANNEL_INFO":        s.channelInfo(p),
		"SM_PLAYER_SPAWN":        playerSpawn(p),
		"SM_EMOTION_LIST":        emotionList(),
		"SM_INFLUENCE_RATIO":     s.influenceRatio(),
		"SM_PRICES":              prices(),
		"SM_SIEGE_LOCATION_INFO": s.siegeLocations(),
		"SM_ABYSS_RANK":          abyssRank(p.abyss),
		"SM_FLY_TIME":            statUpdate(smFlyTime, p.life.FP, p.stats.current(data.FlyTime)),
		"SM_STATUPDATE_MP":       statUpdate(smStatupdateMp, p.life.MP, p.stats.current(data.MaxMP)),
		"SM_FRIEND_LIST":         friendList(),
		"SM_BLOCK_LIST":          blockList(),
		"SM_ABNORMAL_STATE":      abnormalState(),
		"SM_STATUPDATE_HP":       statUpdate(smStatupdateHp, p.life.HP, p.stats.current(data.MaxHP)),
		"SM_ENTER_WORLD_CHECK":   enterWorldCheck(),
	} {
		if got, want := hex.EncodeToString(w.Data[1:]), java[name][0]; got != want {
			t.Errorf("%s differs from AL-Game's:\n%s", name, diffHex(got, want))
		}
	}
}

// diffHex lists the byte offsets where two hex strings differ.
func diffHex(got, want string) string {
	var b strings.Builder
	if len(got) != len(want) {
		b.WriteString("lengths " + strconv.Itoa(len(got)/2) + " and " + strconv.Itoa(len(want)/2) + "\n")
	}
	for i := 0; i+1 < min(len(got), len(want)); i += 2 {
		if got[i:i+2] != want[i:i+2] {
			b.WriteString("byte " + strconv.Itoa(i/2) + ": " + got[i:i+2] + " want " + want[i:i+2] + "\n")
		}
	}
	return b.String()
}

// TestPlayerInfoMatchesClient19 compares SM_PLAYER_INFO with the packet the
// server the 1.9 client was written for sent for Wrathchild (testdata/player-info-1.9.txt).
// AL-Game's source tree has an extra byte in front of the race, from a later client,
// which left the 1.9 client's own character invisible and unable to move.
func TestPlayerInfoMatchesClient19(t *testing.T) {
	d := staticDataOrSkip(t)
	raw, err := os.ReadFile("testdata/player-info-1.9.txt")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{data: d, clockStart: time.Now()}
	p := wrathchild(s)
	p.Heading = 0x3f
	p.life.HP = 93                 // 70% of 132
	p.visualState = visualBlinking // as levelReady sends it
	p.appearance = &store.Appearance{Face: 22, Hair: 18, SkinRGB: 13950968, HairRGB: 2102302, LipRGB: 10531027, EyeRGB: 12418951,
		FaceShape: 15, EyeHeight: 25, EyeSpace: 36, EyeWidth: 15, EyeSize: 133, EyeAngle: 37, BrowHeight: 46, BrowShape: 40,
		Nose: 196, NoseWidth: 25, Cheek: 177, LipHeight: 50, MouthSize: 133, LipSize: 14, Smile: 165, JawHeight: 138,
		ChinJut: 10, EarShape: 9, HeadSize: 243, Neck: 253, NeckLength: 244, Shoulders: 242, ShoulderSize: 161, Torso: 237,
		Chest: 222, Waist: 242, Hips: 241, ArmThickness: 236, ArmLength: 244, HandSize: 236, LegThickness: 247, FootSize: 247,
		Height: 1.03339}
	if got, want := hex.EncodeToString(s.playerInfo(p, false).Data[1:]), strings.TrimSpace(string(raw)); got != want {
		t.Errorf("SM_PLAYER_INFO differs from the 1.9 server's:\n%s", diffHex(got, want))
	}
}
