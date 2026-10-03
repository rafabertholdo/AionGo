package store

import (
	"context"
	"errors"
	"fmt"
	"math"
)

// LootStack increases a stack only if its persisted count still matches Before.
type LootStack struct {
	Before Item
	Count  int64
}

// ReceiveLoot saves all stack increases and new inventory rows atomically.
// Ordinary loot counts fit the client's signed 32-bit item count; kinah has a separate path.
func (s Store) ReceiveLoot(ctx context.Context, owner int32, updates []LootStack, inserts []Item) error {
	if len(updates) == 0 && len(inserts) == 0 {
		return nil
	}
	if owner <= 0 {
		return errors.New("invalid loot owner")
	}
	ids := make(map[int32]struct{}, len(updates)+len(inserts))
	validate := func(item Item) error {
		if item.UniqueID <= 0 || item.ItemID <= 0 || item.ItemID == 182400001 || item.Owner != owner ||
			item.Location != 0 || item.Equipped || item.Count < 1 || item.Count > math.MaxInt32 {
			return errors.New("invalid loot inventory state")
		}
		if _, exists := ids[item.UniqueID]; exists {
			return errors.New("duplicate loot inventory row")
		}
		ids[item.UniqueID] = struct{}{}
		return nil
	}
	for _, update := range updates {
		if err := validate(update.Before); err != nil {
			return err
		}
		if update.Count <= update.Before.Count || update.Count > math.MaxInt32 {
			return errors.New("invalid loot stack increase")
		}
	}
	for _, item := range inserts {
		if err := validate(item); err != nil {
			return err
		}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin loot receipt: %w", err)
	}
	defer tx.Rollback()
	for _, update := range updates {
		item := update.Before
		result, err := tx.ExecContext(ctx, `UPDATE inventory SET itemCount = ? WHERE itemUniqueId = ?
			AND itemOwner = ? AND itemId = ? AND itemCount = ? AND itemLocation = 0 AND isEquiped = 0`,
			update.Count, item.UniqueID, owner, item.ItemID, item.Count)
		if err != nil {
			return fmt.Errorf("increase loot stack: %w", err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("check loot stack increase: %w", err)
		}
		if count != 1 {
			return errors.New("inventory changed during loot receipt")
		}
	}
	for _, item := range inserts {
		if _, err := tx.ExecContext(ctx, insertItem, item.row()...); err != nil {
			return fmt.Errorf("insert received loot: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit loot receipt: %w", err)
	}
	return nil
}
