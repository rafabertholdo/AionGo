package store

import (
	"database/sql"
	"time"
)

// Legion is a legions row, with what belongs to it.
type Legion struct {
	ID                                            int32
	Name                                          string
	Level, Contribution                           int32
	LegionarPerm2, CenturionPerm1, CenturionPerm2 int32
	DisbandTime                                   int32
	EmblemID, EmblemR, EmblemG, EmblemB           int32
}

// LegionMemberRow is a legion_members row with the player it is of, as LegionMemberEx has it.
type LegionMemberRow struct {
	PlayerID, LegionID int32
	Name               string
	Exp                int64
	Class              string
	LastOnline         time.Time
	WorldID            int32
	Nickname           string
	Rank               string // BRIGADE_GENERAL, CENTURION or LEGIONARY
	SelfIntro          string
}

// Announcement is a legion's notice.
type Announcement struct {
	Text string
	At   time.Time
}

// LegionByID is a legion, or nil.
func (s Store) LegionByID(id int32) (*Legion, error) {
	l := &Legion{}
	err := s.DB.QueryRow(`SELECT id, name, level, contribution_points, legionar_permission2, centurion_permission1,
		centurion_permission2, disband_time, COALESCE(e.emblem_id, 0), COALESCE(e.color_r, 0), COALESCE(e.color_g, 0),
		COALESCE(e.color_b, 0) FROM legions LEFT JOIN legion_emblems e ON e.legion_id = legions.id WHERE id = ?`, id).Scan(
		&l.ID, &l.Name, &l.Level, &l.Contribution, &l.LegionarPerm2, &l.CenturionPerm1, &l.CenturionPerm2, &l.DisbandTime,
		&l.EmblemID, &l.EmblemR, &l.EmblemG, &l.EmblemB)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return l, err
}

// LegionNameUsed says whether a legion has the name.
func (s Store) LegionNameUsed(name string) (bool, error) {
	var n int
	err := s.DB.QueryRow(`SELECT count(id) FROM legions WHERE name = ?`, name).Scan(&n)
	return n > 0, err
}

// InsertLegion stores a new legion, with its emblem.
func (s Store) InsertLegion(id int32, name string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO legions (id, name) VALUES (?, ?)`, id, name); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO legion_emblems (legion_id, emblem_id, color_r, color_g, color_b) VALUES (?, 0, 0, 0, 0)`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateLegion saves what changes in a legion.
func (s Store) UpdateLegion(l *Legion) error {
	if _, err := s.DB.Exec(`UPDATE legion_emblems SET emblem_id = ?, color_r = ?, color_g = ?, color_b = ? WHERE legion_id = ?`,
		l.EmblemID, l.EmblemR, l.EmblemG, l.EmblemB, l.ID); err != nil {
		return err
	}
	_, err := s.DB.Exec(`UPDATE legions SET name = ?, level = ?, contribution_points = ?, legionar_permission2 = ?,
		centurion_permission1 = ?, centurion_permission2 = ?, disband_time = ? WHERE id = ?`,
		l.Name, l.Level, l.Contribution, l.LegionarPerm2, l.CenturionPerm1, l.CenturionPerm2, l.DisbandTime, l.ID)
	return err
}

// DeleteLegion removes a legion and, by the tables' keys, its members, notices and emblem.
func (s Store) DeleteLegion(id int32) error {
	_, err := s.DB.Exec(`DELETE FROM legions WHERE id = ?`, id)
	return err
}

// LegionMembers is the members of a legion, or the one member if playerID isn't 0.
func (s Store) LegionMembers(legionID int32) ([]LegionMemberRow, error) {
	return s.legionMemberRows(`legion_members.legion_id = ?`, legionID)
}

// LegionMemberOf is the row of a player who is in a legion, or nil.
func (s Store) LegionMemberOf(playerID int32) (*LegionMemberRow, error) {
	rows, err := s.legionMemberRows(`legion_members.player_id = ?`, playerID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

func (s Store) legionMemberRows(where string, arg int32) ([]LegionMemberRow, error) {
	return queryAll(s.DB, `SELECT legion_members.player_id, legion_members.legion_id, players.name, players.exp, players.player_class,
		players.last_online, players.world_id, legion_members.nickname, legion_members.rank, legion_members.selfintro
		FROM legion_members JOIN players ON players.id = legion_members.player_id WHERE `+where, []any{arg},
		func(r *sql.Rows) (LegionMemberRow, error) {
			var m LegionMemberRow
			var last sql.NullTime
			var self sql.NullString
			err := r.Scan(&m.PlayerID, &m.LegionID, &m.Name, &m.Exp, &m.Class, &last, &m.WorldID, &m.Nickname, &m.Rank, &self)
			m.LastOnline, m.SelfIntro = last.Time, self.String
			return m, err
		})
}

// InsertLegionMember, UpdateLegionMember and DeleteLegionMember keep a player's place in its legion.
func (s Store) InsertLegionMember(legionID, playerID int32, rank string) error {
	_, err := s.DB.Exec(`INSERT INTO legion_members (legion_id, player_id, rank) VALUES (?, ?, ?)`, legionID, playerID, rank)
	return err
}

func (s Store) UpdateLegionMember(playerID int32, nickname, rank, selfIntro string) error {
	_, err := s.DB.Exec(`UPDATE legion_members SET nickname = ?, rank = ?, selfintro = ? WHERE player_id = ?`, nickname, rank, selfIntro, playerID)
	return err
}

func (s Store) DeleteLegionMember(playerID int32) error {
	_, err := s.DB.Exec(`DELETE FROM legion_members WHERE player_id = ?`, playerID)
	return err
}

// LegionAnnouncements is a legion's notices, the oldest of the last seven first.
func (s Store) LegionAnnouncements(legionID int32) ([]Announcement, error) {
	return queryAll(s.DB, `SELECT announcement, date FROM legion_announcement_list WHERE legion_id = ? ORDER BY date ASC LIMIT 0, 7`, []any{legionID},
		func(r *sql.Rows) (Announcement, error) {
			var a Announcement
			var at sql.NullTime
			err := r.Scan(&a.Text, &at)
			a.At = at.Time
			return a, err
		})
}

// InsertAnnouncement stores a new notice.
func (s Store) InsertAnnouncement(legionID int32, text string, at time.Time) error {
	_, err := s.DB.Exec(`INSERT INTO legion_announcement_list (legion_id, announcement, date) VALUES (?, ?, ?)`, legionID, text, at)
	return err
}

// HistoryEntry is a legion_history row: what happened, to whom, when.
type HistoryEntry struct {
	Type string // CREATE, JOIN, KICK, APPOINTED, EMBLEM_REGISTER or EMBLEM_MODIFIED
	Name string
	At   time.Time
}

// LegionHistory is a legion's history, oldest first.
func (s Store) LegionHistory(legionID int32) ([]HistoryEntry, error) {
	return queryAll(s.DB, `SELECT history_type, name, date FROM legion_history WHERE legion_id = ? ORDER BY date ASC, id ASC`,
		[]any{legionID}, func(r *sql.Rows) (HistoryEntry, error) {
			var h HistoryEntry
			var at sql.NullTime
			err := r.Scan(&h.Type, &h.Name, &at)
			h.At = at.Time
			return h, err
		})
}

// InsertHistory stores a new entry.
func (s Store) InsertHistory(legionID int32, h HistoryEntry) error {
	_, err := s.DB.Exec(`INSERT INTO legion_history (legion_id, date, history_type, name) VALUES (?, ?, ?, ?)`, legionID, h.At, h.Type, h.Name)
	return err
}
