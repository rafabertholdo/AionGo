package store

import (
	"database/sql"
	"time"
)

// Petition is a petitions row that has not been replied to.
type Petition struct {
	ID, PlayerID, Type    int32
	Title, Message, Extra string
}

// PrisonTimer is the milliseconds a player has left in prison, or 0 without a player_punishments row.
func (s Store) PrisonTimer(playerID int32) (int64, error) {
	var ms sql.NullInt64
	err := s.DB.QueryRow(`SELECT punishment_timer FROM player_punishments WHERE player_id = ?`, playerID).Scan(&ms)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return ms.Int64, err
}

// PunishPlayer is PlayerPunishmentsDAO.punishPlayer: it replaces the player's row.
func (s Store) PunishPlayer(playerID int32, status int, timerMS int64) error {
	_, err := s.DB.Exec(`REPLACE INTO player_punishments VALUES (?, ?, ?)`, playerID, status, timerMS)
	return err
}

// UnpunishPlayer removes the player's row.
func (s Store) UnpunishPlayer(playerID int32) error {
	_, err := s.DB.Exec(`DELETE FROM player_punishments WHERE player_id = ?`, playerID)
	return err
}

// SavePunishment updates the player's row, if it has one.
func (s Store) SavePunishment(playerID int32, status int, timerMS int64) error {
	_, err := s.DB.Exec(`UPDATE player_punishments SET punishment_status = ?, punishment_timer = ? WHERE player_id = ?`, status, timerMS, playerID)
	return err
}

// AccountIDByName is the account of the character with the name, or 0.
func (s Store) AccountIDByName(name string) (int32, error) {
	var id int32
	err := s.DB.QueryRow(`SELECT account_id FROM players WHERE name = ?`, name).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return id, err
}

// PlayerName is the name of the character with the id, or "".
func (s Store) PlayerName(id int32) (string, error) {
	var name string
	err := s.DB.QueryRow(`SELECT name FROM players WHERE id = ?`, id).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return name, err
}

const petitionColumns = `id, playerId, type, title, message, COALESCE(addData, '')`

func scanPetition(r interface{ Scan(...any) error }) (*Petition, error) {
	var p Petition
	err := r.Scan(&p.ID, &p.PlayerID, &p.Type, &p.Title, &p.Message, &p.Extra)
	return &p, err
}

// Petitions is the pending and in progress petitions, by id.
func (s Store) Petitions() ([]*Petition, error) {
	return queryAll(s.DB, `SELECT `+petitionColumns+` FROM petitions WHERE status = 'PENDING' OR status = 'IN_PROGRESS' ORDER BY id`, nil,
		func(r *sql.Rows) (*Petition, error) { return scanPetition(r) })
}

// PetitionByID is the petition, of any status, or nil.
func (s Store) PetitionByID(id int32) (*Petition, error) {
	p, err := scanPetition(s.DB.QueryRow(`SELECT `+petitionColumns+` FROM petitions WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// NextPetitionID is one more than the highest petition id.
func (s Store) NextPetitionID() (int32, error) {
	var next sql.NullInt32
	err := s.DB.QueryRow(`SELECT MAX(id) FROM petitions`).Scan(&next)
	return next.Int32 + 1, err
}

// InsertPetition stores a new, pending petition.
func (s Store) InsertPetition(p *Petition) error {
	_, err := s.DB.Exec(`INSERT INTO petitions (id, playerId, type, title, message, addData, time, status) VALUES (?, ?, ?, ?, ?, ?, ?, 'PENDING')`,
		p.ID, p.PlayerID, p.Type, p.Title, p.Message, p.Extra, time.Now().Unix())
	return err
}

// DeletePetition removes the player's pending and in progress petitions.
func (s Store) DeletePetition(playerID int32) error {
	_, err := s.DB.Exec(`DELETE FROM petitions WHERE playerId = ? AND (status = 'PENDING' OR status = 'IN_PROGRESS')`, playerID)
	return err
}

// SetPetitionReplied closes a petition.
func (s Store) SetPetitionReplied(id int32) error {
	_, err := s.DB.Exec(`UPDATE petitions SET status = 'REPLIED' WHERE id = ?`, id)
	return err
}
