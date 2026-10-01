package store

import (
	"database/sql"
	"time"
)

// BrokerRow is a broker row: an item for sale, sold, or settled.
type BrokerRow struct {
	Pointer  int32 // the unique id of the item
	ItemID   int32
	Count    int64
	Seller   string
	SellerID int32
	Price    int64
	Race     string // ELYOS or ASMODIAN
	Expire   time.Time
	Settle   time.Time
	Sold     bool
	Settled  bool
}

// BrokerLocation is where the items for sale are kept.
const BrokerLocation = 126

// Broker is every row of the broker, and the items in it.
func (s Store) Broker() ([]BrokerRow, []*Item, error) {
	rows, err := queryAll(s.DB, `SELECT itemPointer, itemId, itemCount, seller, sellerId, price, brokerRace, expireTime, settleTime, isSold, isSettled
		FROM broker`, nil, func(r *sql.Rows) (BrokerRow, error) {
		var b BrokerRow
		var expire, settle sql.NullTime
		err := r.Scan(&b.Pointer, &b.ItemID, &b.Count, &b.Seller, &b.SellerID, &b.Price, &b.Race, &expire, &settle, &b.Sold, &b.Settled)
		b.Expire, b.Settle = expire.Time, settle.Time
		return b, err
	})
	if err != nil {
		return nil, nil, err
	}
	items, err := queryAll(s.DB, `SELECT itemUniqueId, itemOwner, itemId, itemCount, itemColor, slot, enchant, itemSkin, fusionedItem
		FROM inventory WHERE itemLocation = ?`, []any{BrokerLocation}, func(r *sql.Rows) (*Item, error) {
		i := &Item{Location: BrokerLocation}
		err := r.Scan(&i.UniqueID, &i.Owner, &i.ItemID, &i.Count, &i.Color, &i.Slot, &i.Enchant, &i.Skin, &i.Fusioned)
		return i, err
	})
	return rows, items, err
}

// InsertBroker stores an item put up for sale.
func (s Store) InsertBroker(b *BrokerRow) error {
	_, err := s.DB.Exec(`INSERT INTO broker (itemPointer, itemId, itemCount, seller, price, brokerRace, expireTime, sellerId, isSold, isSettled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, b.Pointer, b.ItemID, b.Count, b.Seller, b.Price, b.Race, b.Expire, b.SellerID, b.Sold, b.Settled)
	return err
}

// UpdateBroker stores that an item was sold, or settled unsold.
func (s Store) UpdateBroker(b *BrokerRow) error {
	_, err := s.DB.Exec(`UPDATE broker SET isSold = ?, isSettled = 1, settleTime = ? WHERE itemPointer = ? AND expireTime = ? AND sellerId = ? AND isSettled = 0`,
		b.Sold, b.Settle, b.Pointer, b.Expire, b.SellerID)
	return err
}

// DeleteBroker removes a row.
func (s Store) DeleteBroker(b *BrokerRow) error {
	_, err := s.DB.Exec(`DELETE FROM broker WHERE itemPointer = ? AND sellerId = ? AND expireTime = ?`, b.Pointer, b.SellerID, b.Expire)
	return err
}
