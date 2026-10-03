package login

import (
	"math"
	"testing"

	"aionlightning/wire"
)

func TestGameServerRejectsInvalidCounts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int32
	}{
		{"negative", -1},
		{"too_many", 1},
		{"maximum_integer", math.MaxInt32},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("registration", func(t *testing.T) {
				w := wire.Packet(0)
				w.C(1)
				w.C(4)
				w.B([]byte{127, 0, 0, 1})
				w.D(tc.count)
				g := &gameServerConn{}
				if g.register(wire.NewReader(w.Data[1:])) || g.server != nil {
					t.Fatal("invalid range count accepted")
				}
			})
			t.Run("account_list", func(t *testing.T) {
				w := wire.Packet(4)
				w.D(tc.count)
				g := &gameServerConn{server: &gameServer{}}
				if g.handle(4, wire.NewReader(w.Data[1:])) {
					t.Fatal("invalid account count accepted")
				}
			})
		})
	}
	t.Run("unterminated_account_name", func(t *testing.T) {
		w := wire.Packet(4)
		w.D(1)
		w.H('A')
		g := &gameServerConn{server: &gameServer{}}
		if g.handle(4, wire.NewReader(w.Data[1:])) {
			t.Fatal("unterminated account name accepted")
		}
	})
}
