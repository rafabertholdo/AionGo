package game

import (
	"slices"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// The account warehouse (StorageType.ACCOUNT_WAREHOUSE): inventory rows in location 2 whose itemOwner is the account id
// (AccountService.loadAccount), shared by every character of the account. It has 17 slots and cannot be expanded.
// Only the items move; the dialog that opens it is the regular warehouse's (WarehouseService.sendWarehouseInfo).

const accountWarehouseSlots = 17

type accountWarehouse struct {
	items []*store.Item
	kinah *store.Item
}

// all is the items with the money last, as SM_WAREHOUSE_INFO lists them.
func (a *accountWarehouse) all() []*store.Item {
	items := slices.Clone(a.items)
	if a.kinah != nil {
		items = append(items, a.kinah)
	}
	return items
}

// accountWarehouseOf is the account's warehouse, loaded the first time one of its characters plays.
func (s *Server) accountWarehouseOf(account int32) (*accountWarehouse, error) {
	s.mu.Lock()
	a := s.acctWH[account]
	s.mu.Unlock()
	if a != nil {
		return a, nil
	}
	stored, err := s.store.Items(account, storageAccount, false)
	if err != nil {
		return nil, err
	}
	a = &accountWarehouse{}
	for _, item := range stored {
		if item.ItemID == data.Kinah && a.kinah == nil {
			a.kinah = item
		} else {
			a.items = append(a.items, item)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.acctWH == nil {
		s.acctWH = map[int32]*accountWarehouse{}
	}
	if have := s.acctWH[account]; have != nil {
		return have, nil // another character loaded it meanwhile
	}
	s.acctWH[account] = a
	return a, nil
}
