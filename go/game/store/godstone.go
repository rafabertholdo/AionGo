package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SocketGodstone replaces a weapon's godstone and spends its stone and service fee atomically.
// Expected inventory counts prevent a stale session from overwriting persisted resources.
func (s Store) SocketGodstone(ctx context.Context, weapon, stone, kinah Item, price int64) error {
	if price < 0 || stone.Count < 1 || kinah.Count < price || weapon.Equipped ||
		weapon.Location != 0 || stone.Location != 0 || kinah.Location != 0 ||
		stone.Equipped || kinah.Equipped || kinah.ItemID != 182400001 ||
		weapon.Owner != stone.Owner || weapon.Owner != kinah.Owner ||
		weapon.UniqueID == stone.UniqueID || weapon.UniqueID == kinah.UniqueID || stone.UniqueID == kinah.UniqueID {
		return errors.New("invalid godstone inventory state")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin godstone socket: %w", err)
	}
	defer tx.Rollback()
	var id int32
	if err := tx.QueryRowContext(ctx, `SELECT itemId FROM inventory WHERE itemUniqueId = ? AND itemOwner = ?
		AND itemLocation = 0 AND isEquiped = 0 FOR UPDATE`, weapon.UniqueID, weapon.Owner).Scan(&id); err != nil {
		return fmt.Errorf("lock godstone weapon: %w", err)
	}
	if id != weapon.ItemID {
		return errors.New("godstone weapon changed")
	}
	consume := `UPDATE inventory SET itemCount = itemCount - 1 WHERE itemUniqueId = ? AND itemOwner = ?
		AND itemId = ? AND itemCount = ? AND itemLocation = 0 AND isEquiped = 0`
	if stone.Count == 1 {
		consume = `DELETE FROM inventory WHERE itemUniqueId = ? AND itemOwner = ?
			AND itemId = ? AND itemCount = ? AND itemLocation = 0 AND isEquiped = 0`
	}
	if err := godstoneWrite(ctx, tx, consume, stone.UniqueID, stone.Owner, stone.ItemID, stone.Count); err != nil {
		return fmt.Errorf("consume godstone: %w", err)
	}
	if err := godstoneWrite(ctx, tx, `UPDATE inventory SET itemCount = itemCount - ? WHERE itemUniqueId = ?
		AND itemOwner = ? AND itemId = 182400001 AND itemCount = ? AND itemLocation = 0 AND isEquiped = 0`,
		price, kinah.UniqueID, kinah.Owner, kinah.Count); err != nil {
		return fmt.Errorf("pay godstone fee: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_stones WHERE itemUniqueId = ? AND category = 1`, weapon.UniqueID); err != nil {
		return fmt.Errorf("replace godstone: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO item_stones (itemUniqueId, itemId, slot, category) VALUES (?, ?, 0, 1)`, weapon.UniqueID, stone.ItemID); err != nil {
		return fmt.Errorf("save godstone: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit godstone socket: %w", err)
	}
	return nil
}

func godstoneWrite(ctx context.Context, tx *sql.Tx, query string, args ...any) error {
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("inventory changed during godstone socket")
	}
	return nil
}
