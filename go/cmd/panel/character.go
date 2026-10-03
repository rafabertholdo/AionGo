package main

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// itemInfo is an item as the client describes it (items.json, from scripts/extract-panel-assets.py).
type itemInfo struct {
	Name        string      `json:"n"`
	Icon        string      `json:"i"`
	Quality     string      `json:"q"`
	Level       int         `json:"l"`
	Type        string      `json:"t"`
	MaxStack    int64       `json:"m"`
	Stats       [][2]string `json:"s"`
	Bonuses     [][2]string `json:"b"`
	Description string      `json:"d"`
}

// assets are the client's item data and UI art; without them the page still lists items by id.
type assets struct {
	dir   string
	items map[int32]itemInfo
	sets  []gearSetInfo
	exp   []int64 // total experience at the start of each level, from level 1
}

type gearSetInfo struct {
	Key     string  `json:"k"`
	Name    string  `json:"n"`
	ItemIDs []int32 `json:"i"`
}

type itemSearchResult struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

const maxItemSearchResults = 50

func (a *assets) searchItems(query, category string) []itemSearchResult {
	query = strings.ToLower(strings.TrimSpace(query))
	if category == "all" && query == "" {
		return nil
	}

	results := make([]itemSearchResult, 0)
	for id, item := range a.items {
		if query != "" && !strings.Contains(strings.ToLower(item.Name), query) &&
			!strings.Contains(strconv.FormatInt(int64(id), 10), query) {
			continue
		}
		if category == "stigma" && !isStigmaItem(id) {
			continue
		}
		if category == "spellbook" && !isSpellbookItem(item) {
			continue
		}

		kind := item.Type
		switch {
		case isStigmaItem(id):
			kind = "Stigma"
		case strings.Contains(strings.ToLower(item.Icon), "skillbook"):
			kind = "Skill book"
		}
		results = append(results, itemSearchResult{ID: id, Name: item.Name, Type: kind})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Name != results[j].Name {
			return results[i].Name < results[j].Name
		}
		return results[i].ID < results[j].ID
	})
	if len(results) > maxItemSearchResults {
		results = results[:maxItemSearchResults]
	}
	return results
}

func isStigmaItem(id int32) bool { return id > 140000000 && id < 140001000 }

func isSpellbookItem(item itemInfo) bool {
	return strings.EqualFold(item.Type, "Spellbook") || strings.Contains(strings.ToLower(item.Icon), "skillbook")
}

func loadAssets(dir string, log *slog.Logger) *assets {
	a := &assets{dir: dir, items: map[int32]itemInfo{}}
	for name, into := range map[string]any{"items.json": &a.items, "gear-sets.json": &a.sets, "exp.json": &a.exp} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			err = json.Unmarshal(data, into)
		}
		if err != nil {
			log.Warn("panel assets missing; run scripts/extract-panel-assets.py", "file", name, "err", err)
		}
	}
	return a
}

// level is the level a total experience reaches.
func (a *assets) level(exp int64) int {
	level := 0
	for level < len(a.exp) && exp >= a.exp[level] {
		level++
	}
	return level
}

const (
	kinahID       = 182400001
	cubeTabSlots  = 27 // 9 by 3, as the client's inventory listviews
	cubeTabs      = 4
	cubeLocation  = 0 // inventory.itemLocation: AL-Game's StorageType.CUBE
	baseCubeSlots = 27
)

// equipSlot is one equipment slot of the client's Profile window (UI_Game.xml, player_info_dialog),
// at its position inside the Info tab page.
type equipSlot struct {
	Mask       int64 // AL-Game's ItemSlot
	Preset     string
	X, Y, Size int
}

var equipSlots = []equipSlot{
	{1, "v4_slot_rhand", 32, 24, 44},           // main hand
	{1 << 1, "v4_slot_lhand", 164, 24, 44},     // off hand
	{1 << 2, "v4_slot_helmet", 13, 70, 44},     // helmet
	{1 << 3, "v4_slot_armor", 13, 158, 44},     // torso
	{1 << 4, "v4_slot_glove", 185, 202, 44},    // gloves
	{1 << 5, "v4_slot_shoes", 13, 290, 44},     // boots
	{1 << 6, "v4_slot_rearring", 185, 114, 44}, // left earring
	{1 << 7, "v4_slot_learring", 13, 114, 44},  // right earring
	{1 << 8, "v4_slot_rring", 185, 246, 44},    // left ring
	{1 << 9, "v4_slot_lring", 13, 246, 44},     // right ring
	{1 << 10, "v4_slot_amulet", 185, 70, 44},   // necklace
	{1 << 11, "v4_slot_shoulder", 185, 158, 44},
	{1 << 12, "v4_slot_pants", 13, 202, 44},
	{1 << 13, "v4_slot_battery", 77, 3, 35},  // right power shard
	{1 << 14, "v4_slot_battery", 127, 3, 35}, // left power shard
	{1 << 15, "v4_slot_wing", 98, 42, 44},
	{1 << 16, "v4_slot_belt", 185, 290, 44},
}

type itemView struct {
	itemInfo
	ID      int32
	Count   int64
	Enchant int
}

type slotView struct {
	equipSlot
	Item *itemView
}

type cellView struct {
	Item   *itemView
	Closed bool // past the character's cube size
}

type characterView struct {
	Message, Token string
	Error          bool
	User           *user
	Query          string
	Matches        []string // names like the query, when none is exactly it
	Found          bool

	Name, Race, Class string
	Level             int
	Online            bool
	Equipment         []slotView
	Cube              [][]cellView // cubeTabs pages of cubeTabSlots cells
	GearSets          []gearSetInfo
	Used, Limit       int
	Kinah             int64
}

var (
	//go:embed character.html
	characterHTML string
	characterPage = template.Must(template.New("character").Funcs(template.FuncMap{
		"title": func(s string) string {
			s = strings.ToLower(strings.ReplaceAll(s, "_", " "))
			words := strings.Fields(s)
			for i, w := range words {
				words[i] = strings.ToUpper(w[:1]) + w[1:]
			}
			return strings.Join(words, " ")
		},
		"list":   func(items ...string) []string { return items },
		"commas": commas,
		"add":    func(a, b int) int { return a + b },
		"mul":    func(a, b int) int { return a * b },
		"mod":    func(a, b int) int { return a % b },
		"div":    func(a, b int) int { return a / b },
	}).Parse(characterHTML))
)

func (p *panel) character(u *user, w http.ResponseWriter, r *http.Request) {
	v := characterView{User: u, Query: strings.TrimSpace(r.URL.Query().Get("name"))}
	v.Message, v.Error = r.URL.Query().Get("m"), r.URL.Query().Has("e")
	if u.Admin() {
		v.Token = characterToken(u)
	}
	if v.Query != "" {
		if err := p.findCharacter(&v); err != nil {
			p.log.Error("loading a character", "err", err)
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
	}
	if v.Found && u.Admin() {
		v.GearSets = p.assets.sets
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := characterPage.Execute(w, v); err != nil {
		p.log.Error("rendering", "err", err)
	}
}

func (p *panel) searchItems(_ *user, w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	if category == "" {
		category = "all"
	}
	if category != "all" && category != "stigma" && category != "spellbook" {
		http.Error(w, "unknown item category", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(p.assets.searchItems(r.URL.Query().Get("q"), category)); err != nil {
		p.log.Error("encoding item search results", "err", err)
	}
}

func (p *panel) findCharacter(v *characterView) error {
	var id int32
	var exp int64
	var cubeSize int
	err := p.db.QueryRow(`SELECT id, name, race, player_class, exp, online, cube_size FROM `+p.gsDB+
		`.players WHERE name = ? AND deletion_date IS NULL`, v.Query).
		Scan(&id, &v.Name, &v.Race, &v.Class, &exp, &v.Online, &cubeSize)
	if errors.Is(err, sql.ErrNoRows) {
		rows, err := p.db.Query(`SELECT name FROM `+p.gsDB+`.players WHERE name LIKE ? AND deletion_date IS NULL ORDER BY name LIMIT 20`,
			"%"+likeEscape(v.Query)+"%")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return err
			}
			v.Matches = append(v.Matches, name)
		}
		return rows.Err()
	}
	if err != nil {
		return err
	}
	v.Found = true
	v.Level = p.assets.level(exp)
	v.Limit = baseCubeSlots + cubeSize*9

	rows, err := p.db.Query(`SELECT itemId, itemCount, isEquiped, slot, COALESCE(itemLocation, 0), COALESCE(enchant, 0)
		FROM `+p.gsDB+`.inventory WHERE itemOwner = ? ORDER BY slot`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	equipped := map[int64]*itemView{}
	var cube []*itemView
	var cubePositions []int64
	for rows.Next() {
		var item itemView
		var isEquipped bool
		var slot int64
		var location int
		if err := rows.Scan(&item.ID, &item.Count, &isEquipped, &slot, &location, &item.Enchant); err != nil {
			return err
		}
		item.itemInfo = p.assets.items[item.ID]
		switch {
		case isEquipped:
			equipped[slot] = &item
		case location != cubeLocation:
			// ponytail: warehouses aren't drawn yet; add a tab when someone asks.
		case item.ID == kinahID:
			v.Kinah = item.Count
		default:
			cube = append(cube, &item)
			cubePositions = append(cubePositions, slot)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, s := range equipSlots {
		v.Equipment = append(v.Equipment, slotView{s, equipped[s.Mask]})
	}
	cells := make([]cellView, cubeTabs*cubeTabSlots)
	for i := range cells {
		cells[i].Closed = i >= v.Limit
	}
	place(cells, cube, cubePositions)
	for tab := range cubeTabs {
		v.Cube = append(v.Cube, cells[tab*cubeTabSlots:(tab+1)*cubeTabSlots])
	}
	v.Used = len(cube)
	return nil
}

// place puts each cube item at its saved position, or the first free cell when that one is taken or out of range.
func place(cells []cellView, items []*itemView, positions []int64) {
	var later []*itemView
	for i, item := range items {
		if at := positions[i]; at >= 0 && at < int64(len(cells)) && cells[at].Item == nil {
			cells[at].Item = item
		} else {
			later = append(later, item)
		}
	}
	for i := range cells {
		if len(later) == 0 {
			return
		}
		if cells[i].Item == nil {
			cells[i].Item, later = later[0], later[1:]
		}
	}
}

func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// commas groups a number's digits by thousands, as the client shows kinah.
func commas(n int64) string {
	digits := strconv.FormatInt(n, 10)
	sign := ""
	if n < 0 {
		sign, digits = "-", digits[1:]
	}
	for i := len(digits) - 3; i > 0; i -= 3 {
		digits = digits[:i] + "," + digits[i:]
	}
	return sign + digits
}
