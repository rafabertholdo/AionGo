package store

import (
	"context"
	"fmt"
	"time"
)

// SavedEffect is a player_effects row: an effect running at logout (Level > 0),
// or only a skill's cooldown (Level and Current 0).
type SavedEffect struct {
	SkillID int32
	Level   int32
	Current int32     // milliseconds the effect has been running
	Reuse   time.Time // when the skill can be used again; zero for none
}

// Effects is PlayerEffectsDAO.loadPlayerEffects' read.
func (s Store) Effects(playerID int32) ([]SavedEffect, error) {
	rows, err := s.DB.Query("SELECT skill_id, skill_lvl, `current_time`, reuse_delay FROM player_effects WHERE player_id = ?", playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []SavedEffect
	for rows.Next() {
		var e SavedEffect
		var reuse int64
		if err := rows.Scan(&e.SkillID, &e.Level, &e.Current, &reuse); err != nil {
			return nil, err
		}
		if reuse > 0 {
			e.Reuse = time.UnixMilli(reuse)
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

// SaveEffects is PlayerEffectsDAO.storePlayerEffects: the player's rows are replaced by list, in one transaction.
func (s Store) SaveEffects(ctx context.Context, playerID int32, list []SavedEffect) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin effects save: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM player_effects WHERE player_id = ?`, playerID); err != nil {
		return fmt.Errorf("clear effects: %w", err)
	}
	for _, e := range list {
		var reuse int64
		if !e.Reuse.IsZero() {
			reuse = e.Reuse.UnixMilli()
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO player_effects (player_id, skill_id, skill_lvl, `current_time`, reuse_delay) VALUES (?, ?, ?, ?, ?)",
			playerID, e.SkillID, e.Level, e.Current, reuse); err != nil {
			return fmt.Errorf("save effect %d: %w", e.SkillID, err)
		}
	}
	return tx.Commit()
}
