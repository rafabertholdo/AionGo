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

// ItemCooldown is an item_cooldowns row: when a use delay group of items is ready again.
type ItemCooldown struct {
	UseDelay int32 // the group's delay in milliseconds
	Reuse    time.Time
}

// ItemCooldowns is ItemCooldownsDAO.loadItemCooldowns' read, keyed by delay id.
func (s Store) ItemCooldowns(playerID int32) (map[int32]ItemCooldown, error) {
	rows, err := s.DB.Query("SELECT delay_id, use_delay, reuse_time FROM item_cooldowns WHERE player_id = ?", playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var m map[int32]ItemCooldown
	for rows.Next() {
		var id int32
		var c ItemCooldown
		var reuse int64
		if err := rows.Scan(&id, &c.UseDelay, &reuse); err != nil {
			return nil, err
		}
		c.Reuse = time.UnixMilli(reuse)
		if m == nil {
			m = map[int32]ItemCooldown{}
		}
		m[id] = c
	}
	return m, rows.Err()
}

// SaveItemCooldowns is ItemCooldownsDAO.storeItemCooldowns: the player's rows are replaced by m, in one transaction.
func (s Store) SaveItemCooldowns(ctx context.Context, playerID int32, m map[int32]ItemCooldown) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin item cooldowns save: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_cooldowns WHERE player_id = ?`, playerID); err != nil {
		return fmt.Errorf("clear item cooldowns: %w", err)
	}
	for id, c := range m {
		if _, err := tx.ExecContext(ctx, `INSERT INTO item_cooldowns (player_id, delay_id, use_delay, reuse_time) VALUES (?, ?, ?, ?)`,
			playerID, id, c.UseDelay, c.Reuse.UnixMilli()); err != nil {
			return fmt.Errorf("save item cooldown %d: %w", id, err)
		}
	}
	return tx.Commit()
}
