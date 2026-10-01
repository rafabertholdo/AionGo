package store

// SaveSkill stores a player's skill: it is new, or its level changed.
func (s Store) SaveSkill(playerID int32, k Skill) error {
	_, err := s.DB.Exec(`INSERT INTO player_skills (player_id, skillId, skillLevel) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE skillLevel = VALUES(skillLevel)`, playerID, k.ID, k.Level)
	return err
}

// DeleteSkill removes a player's skill.
func (s Store) DeleteSkill(playerID, skillID int32) error {
	_, err := s.DB.Exec(`DELETE FROM player_skills WHERE player_id = ? AND skillId = ?`, playerID, skillID)
	return err
}
