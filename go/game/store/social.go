package store

import "database/sql"

// Block is a blocks row with the blocked character's name.
type Block struct {
	ID     int32
	Name   string
	Reason string
}

// Friends is the characters a player has as friends.
func (s Store) Friends(playerID int32) ([]*Character, error) {
	rows, err := s.DB.Query(`SELECT `+characterColumns+` FROM players WHERE id IN (SELECT friend FROM friends WHERE player = ?)`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var friends []*Character
	for rows.Next() {
		c, err := scanCharacter(rows)
		if err != nil {
			return nil, err
		}
		friends = append(friends, c)
	}
	return friends, rows.Err()
}

// AddFriends makes two players friends of each other.
func (s Store) AddFriends(a, b int32) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, pair := range [][2]int32{{a, b}, {b, a}} {
		if _, err := tx.Exec(`INSERT INTO friends (player, friend) VALUES (?, ?)`, pair[0], pair[1]); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DelFriends ends a friendship, from both sides.
func (s Store) DelFriends(a, b int32) error {
	_, err := s.DB.Exec(`DELETE FROM friends WHERE (player = ? AND friend = ?) OR (player = ? AND friend = ?)`, a, b, b, a)
	return err
}

// Blocks is the characters a player has blocked.
func (s Store) Blocks(playerID int32) ([]Block, error) {
	rows, err := s.DB.Query(`SELECT b.blocked_player, p.name, b.reason FROM blocks b JOIN players p ON p.id = b.blocked_player WHERE b.player = ?`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var blocks []Block
	for rows.Next() {
		var b Block
		var reason sql.NullString
		if err := rows.Scan(&b.ID, &b.Name, &reason); err != nil {
			return nil, err
		}
		b.Reason = reason.String
		blocks = append(blocks, b)
	}
	return blocks, rows.Err()
}

func (s Store) AddBlock(playerID, blocked int32, reason string) error {
	_, err := s.DB.Exec(`INSERT INTO blocks (player, blocked_player, reason) VALUES (?, ?, ?)`, playerID, blocked, reason)
	return err
}

func (s Store) DelBlock(playerID, blocked int32) error {
	_, err := s.DB.Exec(`DELETE FROM blocks WHERE player = ? AND blocked_player = ?`, playerID, blocked)
	return err
}

func (s Store) SetBlockReason(playerID, blocked int32, reason string) error {
	_, err := s.DB.Exec(`UPDATE blocks SET reason = ? WHERE player = ? AND blocked_player = ?`, reason, playerID, blocked)
	return err
}
