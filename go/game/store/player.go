package store

import (
	"database/sql"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Quest is a player_quests row.
type Quest struct {
	ID            int32
	Status        string // START, REWARD, COMPLETE, …
	Vars          int32
	CompleteCount int32
}

// QuestCompletion persists the non-item effects of a completed quest with its status.
type QuestCompletion struct {
	TitleID       int32
	Abyss         *AbyssRank
	CubeSize      int
	WarehouseSize int
}

// Macro is a player_macrosses row.
type Macro struct {
	Order int32
	Text  string
}

// Settings are a player's player_settings: the client's UI layout and
// shortcuts, stored as it sent them, and the display and deny flags.
type Settings struct {
	UI, Shortcuts []byte
	Display, Deny int32
}

// AbyssRank is an abyss_rank row.
type AbyssRank struct {
	DailyAP, WeeklyAP, AP          int32
	Rank, TopRanking               int32
	DailyKill, WeeklyKill, AllKill int32
	MaxRank, LastKill, LastAP      int32
	LastUpdate                     int64 // ms
}

// Stone is an item_stones row: a manastone (category 0) or godstone (1) socketed in an item.
type Stone struct {
	ItemID   int32
	Slot     int32
	Category int32
}

// LifeStats is a player_life_stats row.
type LifeStats struct {
	HP, MP, FP int32
}

// Skills is a player's skills.
func (s Store) Skills(playerID int32) ([]Skill, error) {
	return queryAll(s.DB, `SELECT skillId, skillLevel FROM player_skills WHERE player_id = ?`, []any{playerID},
		func(r *sql.Rows) (Skill, error) {
			var k Skill
			return k, r.Scan(&k.ID, &k.Level)
		})
}

// Quests is a player's quests.
func (s Store) Quests(playerID int32) ([]Quest, error) {
	return queryAll(s.DB, `SELECT quest_id, status, quest_vars, complete_count FROM player_quests WHERE player_id = ?`, []any{playerID},
		func(r *sql.Rows) (Quest, error) {
			var q Quest
			return q, r.Scan(&q.ID, &q.Status, &q.Vars, &q.CompleteCount)
		})
}

// SaveQuest keeps a quest transition across disconnects, including a movie that has not finished yet.
func (s Store) SaveQuest(playerID int32, q Quest) error {
	_, err := s.DB.Exec(`INSERT INTO player_quests (player_id, quest_id, status, quest_vars, complete_count)
		VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE status = VALUES(status),
		quest_vars = VALUES(quest_vars), complete_count = VALUES(complete_count)`,
		playerID, q.ID, q.Status, q.Vars, q.CompleteCount)
	return err
}

// CompleteQuest commits quest status and its persistent character rewards together.
func (s Store) CompleteQuest(playerID int32, q Quest, reward QuestCompletion) (err error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO player_quests (player_id, quest_id, status, quest_vars, complete_count)
		VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE status = VALUES(status),
		quest_vars = VALUES(quest_vars), complete_count = VALUES(complete_count)`,
		playerID, q.ID, q.Status, q.Vars, q.CompleteCount); err != nil {
		return err
	}
	if reward.TitleID != 0 {
		if _, err = tx.Exec(`INSERT IGNORE INTO player_titles (player_id, title_id) VALUES (?, ?)`, playerID, reward.TitleID); err != nil {
			return err
		}
	}
	if reward.Abyss != nil {
		a := reward.Abyss
		if _, err = tx.Exec(`UPDATE abyss_rank SET ap = ?, daily_ap = ?, weekly_ap = ?, rank = ?, max_rank = ? WHERE player_id = ?`,
			a.AP, a.DailyAP, a.WeeklyAP, a.Rank, a.MaxRank, playerID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`UPDATE players SET cube_size = ?, warehouse_size = ? WHERE id = ?`, reward.CubeSize, reward.WarehouseSize, playerID); err != nil {
		return err
	}
	return tx.Commit()
}

// Recipes is the recipes a player has learned.
func (s Store) Recipes(playerID int32) ([]int32, error) {
	return queryAll(s.DB, `SELECT recipe_id FROM player_recipes WHERE player_id = ?`, []any{playerID},
		func(r *sql.Rows) (int32, error) {
			var id int32
			return id, r.Scan(&id)
		})
}

// AddRecipe and DeleteRecipe keep the recipes a player has learned.
func (s Store) AddRecipe(playerID, recipeID int32) error {
	_, err := s.DB.Exec(`INSERT INTO player_recipes (player_id, recipe_id) VALUES (?, ?)`, playerID, recipeID)
	return err
}

func (s Store) DeleteRecipe(playerID, recipeID int32) error {
	_, err := s.DB.Exec(`DELETE FROM player_recipes WHERE player_id = ? AND recipe_id = ?`, playerID, recipeID)
	return err
}

// Macros is a player's macros, in order.
func (s Store) Macros(playerID int32) ([]Macro, error) {
	macros, err := queryAll(s.DB, "SELECT `order`, macro FROM player_macrosses WHERE player_id = ?", []any{playerID},
		func(r *sql.Rows) (Macro, error) {
			var m Macro
			return m, r.Scan(&m.Order, &m.Text)
		})
	slices.SortFunc(macros, func(a, b Macro) int { return int(a.Order - b.Order) })
	return macros, err
}

// AddMacro and DeleteMacro keep a player's macros.
func (s Store) AddMacro(playerID, order int32, text string) error {
	_, err := s.DB.Exec("INSERT INTO player_macrosses (player_id, `order`, macro) VALUES (?, ?, ?)", playerID, order, text)
	return err
}

func (s Store) DeleteMacro(playerID, order int32) error {
	_, err := s.DB.Exec("DELETE FROM player_macrosses WHERE player_id = ? AND `order` = ?", playerID, order)
	return err
}

// Titles is the titles a player has earned.
func (s Store) Titles(playerID int32) ([]int32, error) {
	return queryAll(s.DB, `SELECT title_id FROM player_titles WHERE player_id = ?`, []any{playerID},
		func(r *sql.Rows) (int32, error) {
			var id int32
			return id, r.Scan(&id)
		})
}

// Settings is a player's settings.
func (s Store) Settings(playerID int32) (*Settings, error) {
	rows, err := s.DB.Query(`SELECT settings_type, settings FROM player_settings WHERE player_id = ?`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	settings := &Settings{}
	for rows.Next() {
		var kind int
		var blob []byte
		if err := rows.Scan(&kind, &blob); err != nil {
			return nil, err
		}
		switch kind {
		case 0:
			settings.UI = blob
		case 1:
			settings.Shortcuts = blob
		case 2:
			settings.Display = intSetting(blob)
		case 3:
			settings.Deny = intSetting(blob)
		}
	}
	return settings, rows.Err()
}

// intSetting is a display or deny setting: a number kept in the BLOB as text.
func intSetting(blob []byte) int32 {
	v, _ := strconv.Atoi(strings.TrimSpace(string(blob)))
	return int32(v)
}

// SaveSetting stores one of a player's settings (0 UI, 1 shortcuts).
func (s Store) SaveSetting(playerID int32, kind int, blob []byte) error {
	_, err := s.DB.Exec(`REPLACE INTO player_settings VALUES (?, ?, ?)`, playerID, kind, blob)
	return err
}

// AbyssRank is a player's abyss rank, or nil if there is none yet.
func (s Store) AbyssRank(playerID int32) (*AbyssRank, error) {
	a := &AbyssRank{}
	err := s.DB.QueryRow(`SELECT daily_ap, weekly_ap, ap, rank, top_ranking, daily_kill, weekly_kill, all_kill,
		max_rank, last_kill, last_ap, last_update FROM abyss_rank WHERE player_id = ?`, playerID).Scan(
		&a.DailyAP, &a.WeeklyAP, &a.AP, &a.Rank, &a.TopRanking, &a.DailyKill, &a.WeeklyKill, &a.AllKill,
		&a.MaxRank, &a.LastKill, &a.LastAP, &a.LastUpdate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return a, err
}

// InsertAbyssRank stores a new abyss rank.
func (s Store) InsertAbyssRank(playerID int32, a *AbyssRank) error {
	_, err := s.DB.Exec(`INSERT INTO abyss_rank (player_id, daily_ap, weekly_ap, ap, rank, top_ranking, daily_kill,
		weekly_kill, all_kill, max_rank, last_kill, last_ap, last_update) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		playerID, a.DailyAP, a.WeeklyAP, a.AP, a.Rank, a.TopRanking, a.DailyKill, a.WeeklyKill, a.AllKill,
		a.MaxRank, a.LastKill, a.LastAP, a.LastUpdate)
	return err
}

// SaveAbyssRank stores a player's abyss rank.
func (s Store) SaveAbyssRank(playerID int32, a *AbyssRank) error {
	_, err := s.DB.Exec(`UPDATE abyss_rank SET daily_ap = ?, weekly_ap = ?, ap = ?, rank = ?, top_ranking = ?, daily_kill = ?,
		weekly_kill = ?, all_kill = ?, max_rank = ?, last_kill = ?, last_ap = ?, last_update = ? WHERE player_id = ?`,
		a.DailyAP, a.WeeklyAP, a.AP, a.Rank, a.TopRanking, a.DailyKill, a.WeeklyKill, a.AllKill,
		a.MaxRank, a.LastKill, a.LastAP, a.LastUpdate, playerID)
	return err
}

// Stones is the stones socketed in an item.
func (s Store) Stones(itemUniqueID int32) ([]Stone, error) {
	return queryAll(s.DB, `SELECT itemId, slot, category FROM item_stones WHERE itemUniqueId = ?`, []any{itemUniqueID},
		func(r *sql.Rows) (Stone, error) {
			var st Stone
			return st, r.Scan(&st.ItemID, &st.Slot, &st.Category)
		})
}

// AddStone sockets a stone in an item, and DeleteStones takes them all out.
func (s Store) AddStone(itemUniqueID int32, st Stone) error {
	_, err := s.DB.Exec(`INSERT INTO item_stones (itemUniqueId, itemId, slot, category) VALUES (?, ?, ?, ?)`,
		itemUniqueID, st.ItemID, st.Slot, st.Category)
	return err
}

// DeleteStone removes one socketed stone without disturbing the others.
func (s Store) DeleteStone(itemUniqueID, slot int32) error {
	_, err := s.DB.Exec(`DELETE FROM item_stones WHERE itemUniqueId = ? AND slot = ? AND category = 0`, itemUniqueID, slot)
	return err
}

func (s Store) DeleteStones(itemUniqueID int32) error {
	_, err := s.DB.Exec(`DELETE FROM item_stones WHERE itemUniqueId = ?`, itemUniqueID)
	return err
}

// LifeStats is a player's HP, MP and flight time, or nil if none are stored.
func (s Store) LifeStats(playerID int32) (*LifeStats, error) {
	l := &LifeStats{}
	err := s.DB.QueryRow(`SELECT hp, mp, fp FROM player_life_stats WHERE player_id = ?`, playerID).Scan(&l.HP, &l.MP, &l.FP)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return l, err
}

// SaveLifeStats stores a player's HP, MP and flight time.
func (s Store) SaveLifeStats(playerID int32, l LifeStats) error {
	_, err := s.DB.Exec(`INSERT INTO player_life_stats (player_id, hp, mp, fp) VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE hp = VALUES(hp), mp = VALUES(mp), fp = VALUES(fp)`, playerID, l.HP, l.MP, l.FP)
	return err
}

// SaveCharacter is PlayerDAO.storePlayer: a player's row as it is now.
func (s Store) SaveCharacter(c *Character) error {
	_, err := s.DB.Exec(`UPDATE players SET name = ?, exp = ?, recoverexp = ?, x = ?, y = ?, z = ?, heading = ?,
		world_id = ?, player_class = ?, last_online = ?, cube_size = ?, advenced_stigma_slot_size = ?,
		warehouse_size = ?, note = ?, bind_point = ?, title_id = ?, mailboxLetters = ? WHERE id = ?`,
		c.Name, c.Exp, c.RecoverExp, c.X, c.Y, c.Z, c.Heading, c.WorldID, c.Class, c.LastOnline, c.CubeSize,
		c.StigmaSlots, c.WarehouseSize, c.Note, c.BindPoint, c.TitleID, c.Letters, c.ID)
	return err
}

// SetOnline marks a player online or offline.
func (s Store) SetOnline(playerID int32, online bool) error {
	_, err := s.DB.Exec(`UPDATE players SET online = ? WHERE id = ?`, online, playerID)
	return err
}

// SetLastOnline records when a player was last in the game.
func (s Store) SetLastOnline(playerID int32, at time.Time) error {
	_, err := s.DB.Exec(`UPDATE players SET last_online = ? WHERE id = ?`, at, playerID)
	return err
}

func queryAll[T any](db *sql.DB, query string, args []any, scan func(*sql.Rows) (T, error)) ([]T, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GameTime is the saved game clock, in game minutes: 0 if never saved.
func (s Store) GameTime() (int32, error) {
	var t int32
	err := s.DB.QueryRow("SELECT `value` FROM server_variables WHERE `key` = 'time'").Scan(&t)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return t, err
}

// SaveGameTime saves the game clock.
func (s Store) SaveGameTime(t int32) error {
	_, err := s.DB.Exec("REPLACE INTO server_variables (`key`, `value`) VALUES ('time', ?)", t)
	return err
}

// SiegeOwner is who holds a siege location: siege_locations' race and legion.
type SiegeOwner struct {
	Race   string // ELYOS, ASMODIANS, BALAUR
	Legion int32
}

// SiegeOwners is the owner of each siege location by id.
func (s Store) SiegeOwners() (map[int32]SiegeOwner, error) {
	rows, err := s.DB.Query(`SELECT id, race, legion_id FROM siege_locations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	owners := map[int32]SiegeOwner{}
	for rows.Next() {
		var id int32
		var o SiegeOwner
		if err := rows.Scan(&id, &o.Race, &o.Legion); err != nil {
			return nil, err
		}
		owners[id] = o
	}
	return owners, rows.Err()
}

// SaveSiegeOwner records who holds a siege location (SiegeDAO.updateSiegeLocation).
func (s Store) SaveSiegeOwner(id int32, o SiegeOwner) error {
	_, err := s.DB.Exec("INSERT INTO siege_locations (id, race, legion_id) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE race = VALUES(race), legion_id = VALUES(legion_id)", id, o.Race, o.Legion)
	return err
}
