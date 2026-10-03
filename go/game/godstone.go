package game

import (
	"context"
	"slices"
	"strconv"
	"time"

	"aionlightning/game/store"
	"aionlightning/wire"
)

func init() {
	handlers[cmGodstoneSocket] = (*conn).godstoneSocket
}

type godstoneSaver interface {
	SocketGodstone(context.Context, store.Item, store.Item, store.Item, int64) error
}

const (
	msgGodstoneEquipped = 1300503
	msgNotGodstone      = 1300505
	msgGodstoneSuccess  = 1300508
)

// godstoneSocket is CM_GODSTONE_SOCKET and ItemService.socketGodstone.
func (c *conn) godstoneSocket(r *wire.Reader) {
	npcID, weaponID, stoneID := r.D(), r.D(), r.D()
	if r.Err != nil {
		return
	}
	c.withPlayer(func(s *Server, p *player) {
		o := s.byID[npcID]
		if o == nil || o.npc == nil || o.worldID != p.WorldID || o.instance != p.instance || p.dead {
			return
		}
		// Java MathUtil.isInRange checks XY, with a strict 15-unit boundary.
		dx, dy := p.X-o.x, p.Y-o.y
		if !(dx*dx+dy*dy < 15*15) || s.restrictedInPrison(p, "godstone socketing") {
			return
		}
		weapon := p.cubeItem(weaponID)
		if weapon == nil || weapon.Equipped {
			c.send(systemMessage(msgGodstoneEquipped))
			return
		}
		wt := s.template(weapon)
		stone := p.cubeItem(stoneID)
		if wt == nil || !wt.IsWeapon() || weapon.Owner != p.ID || weapon.Location != 0 ||
			stone == nil || stone == weapon || stone.Count < 1 || stone.Equipped || stone.Owner != p.ID || stone.Location != 0 {
			return
		}
		st := s.template(stone)
		if st == nil || st.Godstone == nil {
			c.send(systemMessage(msgNotGodstone))
			return
		}
		price := int64(float64(int64(float64(int64(float64(100000)*priceDefault/100))*priceModifier/100)) * priceTaxes / 100)
		if p.kinah == nil || p.kinah.Count < price {
			c.send(systemMessage(msgNotEnoughKinah, strconv.FormatInt(price, 10)))
			return
		}
		db, ok := s.items.(godstoneSaver)
		if !ok {
			s.log.Error("godstone persistence unavailable")
			return
		}
		// Connections do not yet expose a lifecycle context; bound this transaction meanwhile.
		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		defer cancel()
		if err := db.SocketGodstone(ctx, *weapon, *stone, *p.kinah, price); err != nil {
			s.log.Error("socketing godstone", "err", err)
			return
		}
		p.kinah.Count -= price
		weapon.Godstone = stone.ItemID
		stone.Count--
		c.send(s.updateItemPacket(p, p.kinah))
		c.send(systemMessage(msgGodstoneSuccess, descriptionID(wt.NameID)))
		if stone.Count == 0 {
			p.cube = slices.DeleteFunc(p.cube, func(i *store.Item) bool { return i == stone })
			c.send(deleteItemPacket(stone.UniqueID))
			s.ids.release(stone.UniqueID)
		}
		c.send(s.updateItemPacket(p, stone))
		c.send(s.updateItemPacket(p, weapon))
	})
}
