package game

import (
	"slices"
	"strconv"
)

// The services an npc offers by dialog (NpcController.onDialogSelect): shops, soul healing, expanding the cube
// and the warehouse, and the dialogs that only open a window.

// windows are the dialogs that answer with a window of the client's own.
var windows = map[uint16]uint16{4: 1, 5: 2, 27: 13, 35: 21, 36: 20, 37: 19, 52: 28, 60: 29, 61: 30}

// Messages and questions of the services.
const (
	dialogWarehouse   = 20
	dialogTeleport    = 38
	dialogCraftMaster = 40
	dialogSoulHealing = 29
	dialogExpandCube  = 41
	dialogExpandStore = 42

	questionExpandWarehouse = 900686
	msgSoulHealed           = 1300674
	msgNoRecoverableExp     = 1300682
	msgNotEnoughKinah       = 901285
	msgCannotExpand         = 1300430
	msgCubeExpanded         = 1300431
	msgExpandNoKinah        = 1300831
	msgWarehouseExpanded    = 1300433
)

// serviceDialog answers a dialog select the npc's services cover, and says whether it did.
func (c *conn) serviceDialog(id int32, dialog uint16) bool {
	// A player's store, opened by asking its owner.
	if dialog == dialogBuy && c.player != nil {
		c.s.visMu.Lock()
		seller := c.s.spawned[id]
		if seller != nil && seller.store != nil {
			c.send(c.s.privateStorePacket(seller))
		}
		c.s.visMu.Unlock()
		if seller != nil {
			return true
		}
	}
	switch dialog {
	case dialogBuy, dialogSell:
		return c.shopDialog(id, dialog)
	case dialogSoulHealing, dialogExpandCube, dialogExpandStore, dialogWarehouse, dialogTeleport, dialogCraftMaster, dialogDisbandLegion, dialogRecreateLegion:
	default:
		if _, ok := windows[dialog]; !ok {
			return false
		}
	}
	s := c.s
	s.visMu.Lock()
	defer s.visMu.Unlock()
	o := c.dialogNpc(id)
	if o == nil {
		return false
	}
	p := c.player
	switch dialog {
	case dialogDisbandLegion:
		s.disbandDialog(p)
	case dialogRecreateLegion:
		s.recreateDialog(p)
	case dialogTeleport:
		s.showTeleportMap(p, o)
	case dialogCraftMaster:
		s.learnCraftSkill(p, o)
	case dialogWarehouse:
		c.send(dialogWindow(o.id, 26, 0))
		s.sendWarehouseInfo(p, true)
	case dialogSoulHealing:
		s.soulHealing(p)
	case dialogExpandCube:
		s.expand(p, s.data.CubeExpand[o.npc.ID], p.CubeSize, 9, questionExpandWarehouse, func() {
			p.CubeSize++
			p.conn.send(systemMessage(msgCubeExpanded, "9"))
			p.conn.send(cubeSizeUpdate(p))
		}, msgExpandNoKinah)
	case dialogExpandStore:
		s.expand(p, s.data.WarehouseExpand[o.npc.ID], p.WarehouseSize, 10, questionExpandWarehouse, func() {
			p.WarehouseSize++
			p.conn.send(systemMessage(msgWarehouseExpanded, "8"))
			s.sendWarehouseInfo(p, false)
		}, msgExpandNoKinah)
	default:
		c.send(dialogWindow(o.id, windows[dialog], 0))
	}
	return true
}

// soulHealing is the dialog of a soul healer: the experience lost by dying is bought back.
func (s *Server) soulHealing(p *player) {
	lost := p.RecoverExp
	if lost <= 0 {
		p.conn.send(systemMessage(msgNoRecoverableExp))
		return
	}
	factor := 0.1
	if lost < 1000000 {
		factor = 0.25 - 0.00000015*float64(lost)
	}
	price := int64(float64(lost) * factor)
	asked := p.putRequest(questionSoulHealing, func(accepted bool) {
		if !accepted {
			return
		}
		if !s.decreaseKinah(p, price) {
			p.conn.send(systemMessage(msgNotEnoughKinah, strconv.FormatInt(price, 10)))
			return
		}
		p.conn.send(systemMessage(msgExp, strconv.FormatInt(lost, 10)))
		p.conn.send(systemMessage(msgSoulHealed))
		s.resetRecoverableExp(p)
	})
	if asked {
		p.conn.send(questionWindow(questionSoulHealing, 0, strconv.FormatInt(price, 10)))
	}
}

// expand is CubeExpandService.expandCube and WarehouseService.expandWarehouse: the player is asked to pay for the
// next level if the npc sells it.
func (s *Server) expand(p *player, prices map[int32]int32, size, max int, question int32, grow func(), noKinah int32) {
	next := int32(size + 1)
	price, ok := prices[next]
	if !ok || int(next) > max {
		p.conn.send(systemMessage(msgCannotExpand))
		return
	}
	asked := p.putRequest(question, func(accepted bool) {
		if !accepted || int(next) != size+1 {
			return
		}
		if !s.decreaseKinah(p, int64(price)) {
			p.conn.send(systemMessage(int32(noKinah)))
			return
		}
		grow()
	})
	if asked {
		p.conn.send(questionWindow(question, 0, strconv.Itoa(int(price))))
	}
}

// craftMasters are the npcs who teach a crafting skill, by npc id: the skill and whether it is one of the six crafts.
var craftMasters = map[int32]struct {
	skill int32
	craft bool
}{
	204096: {30002, false}, 204257: {30003, false}, 204100: {40001, true}, 204104: {40002, true}, 204106: {40003, true},
	204110: {40004, true}, 204102: {40007, true}, 204108: {40008, true},
	203780: {30002, false}, 203782: {30003, false}, 203784: {40001, true}, 203788: {40002, true}, 203790: {40003, true},
	203793: {40004, true}, 203786: {40007, true}, 203792: {40008, true},
}

// craftSkillCost is what a master asks to teach the next level, by the level the player has.
var craftSkillCost = map[int32]int64{0: 3500, 99: 17000, 199: 115000, 299: 460000, 399: 1500000}

const questionCraftSkill = 900852

// learnCraftSkill is CraftSkillUpdateService.learnSkill: a master teaches the next stage of its skill for a price.
func (s *Server) learnCraftSkill(p *player, o *object) {
	master, ok := craftMasters[o.npc.ID]
	if !ok || p.level < 10 {
		return
	}
	level := int32(0)
	mastered := 0
	for _, k := range p.skills {
		if k.ID == master.skill {
			level = k.Level
		}
		if k.Level > 399 && slices.Contains([]int32{40001, 40002, 40003, 40004, 40007, 40008}, k.ID) {
			mastered++
		}
	}
	price, ok := craftSkillCost[level]
	if !ok {
		return
	}
	if master.craft && mastered >= 2 && level == 399 {
		s.tell(p, "You can only master 2 craft skill.")
		return
	}
	if p.kinah.Count < price {
		s.tell(p, "You don't have enough Kinah.")
		return
	}
	name := int32(0)
	if t := s.data.Skills[master.skill]; t != nil {
		name = t.NameID
	}
	asked := p.putRequest(questionCraftSkill, func(accepted bool) {
		if accepted && s.decreaseKinah(p, price) {
			s.addSkill(p, master.skill, level+1, true)
			s.autolearnRecipes(p, master.skill, level+1)
		}
	})
	if asked {
		p.conn.send(questionWindow(questionCraftSkill, 0, descriptionID(name), strconv.FormatInt(price, 10)))
	}
}
