package data

import "testing"

func TestInstancePortalRequiresExternalRaceReturn(t *testing.T) {
	internal := &Portal{Instance: true, Exit: Point{MapID: 300040000}, Entry: []Point{{MapID: 300040000}}}
	wrongRace := &Portal{Instance: true, Exit: Point{MapID: 300040000}, Entry: []Point{{MapID: 220040000, Race: "ASMODIANS"}}}
	external := &Portal{Instance: true, Exit: Point{MapID: 300040000}, Entry: []Point{{MapID: 210040000, Race: "ELYOS"}}}
	d := &Data{PortalList: []*Portal{internal, wrongRace, external}}
	if got := d.InstancePortal(300040000, "ELYOS"); got != external {
		t.Fatalf("return portal = %v; want external Elyos entrance", got)
	}
	d.PortalList = []*Portal{internal, wrongRace}
	if got := d.InstancePortal(300040000, "ELYOS"); got != nil {
		t.Fatalf("unavailable external return = %v; want nil", got)
	}
}
