package game

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

type failingItemUpdate struct {
	itemSaver
	err error
}

func (f failingItemUpdate) UpdateItem(*store.Item) error { return f.err }

func TestKinahMutationDoesNotCommitOnPersistenceFailure(t *testing.T) {
	failure := errors.New("database unavailable")
	s := &Server{
		items: failingItemUpdate{err: failure},
		log:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	p := &player{kinah: &store.Item{UniqueID: 1, ItemID: data.Kinah, Count: 100}}

	if s.increaseKinah(p, 25) {
		t.Fatal("increaseKinah reported success after the database update failed")
	}
	if p.kinah.Count != 100 {
		t.Fatalf("increaseKinah changed memory to %d after persistence failed", p.kinah.Count)
	}
	if s.decreaseKinah(p, 25) {
		t.Fatal("decreaseKinah reported success after the database update failed")
	}
	if p.kinah.Count != 100 {
		t.Fatalf("decreaseKinah changed memory to %d after persistence failed", p.kinah.Count)
	}
	if s.addItem(p, data.Kinah, 25) {
		t.Fatal("addItem reported a kinah grant after the database update failed")
	}
	if p.kinah.Count != 100 {
		t.Fatalf("addItem changed kinah to %d after persistence failed", p.kinah.Count)
	}
}
