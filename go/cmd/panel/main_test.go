package main

import (
	"testing"
	"time"
)

func TestSessionCookie(t *testing.T) {
	now := time.Now()
	cookie := sign(42, now.Add(time.Hour))
	if id, ok := verify(cookie, now); !ok || id != 42 {
		t.Fatalf("verify(%q) = %d, %v", cookie, id, ok)
	}
	if _, ok := verify(cookie, now.Add(2*time.Hour)); ok {
		t.Error("an expired cookie verified")
	}
	if _, ok := verify("43"+cookie[2:], now); ok {
		t.Error("a cookie with another account id verified")
	}
	if _, ok := verify("garbage", now); ok {
		t.Error("garbage verified")
	}
}

func TestCommas(t *testing.T) {
	for n, want := range map[int64]string{0: "0", 999: "999", 1000: "1,000", 999999886516: "999,999,886,516", -1234: "-1,234"} {
		if got := commas(n); got != want {
			t.Errorf("commas(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestPlaceKeepsSavedPositionsAndFillsGaps(t *testing.T) {
	a, b, c := &itemView{ID: 1}, &itemView{ID: 2}, &itemView{ID: 3}
	cells := make([]cellView, 4)
	place(cells, []*itemView{a, b, c}, []int64{2, 2, 99}) // b collides with a, c is out of range
	if cells[2].Item != a || cells[0].Item != b || cells[1].Item != c || cells[3].Item != nil {
		t.Errorf("cells = %v %v %v %v", cells[0].Item, cells[1].Item, cells[2].Item, cells[3].Item)
	}
}

func TestLevelFromExperience(t *testing.T) {
	a := &assets{exp: []int64{0, 650, 2567}}
	for exp, want := range map[int64]int{0: 1, 649: 1, 650: 2, 5000: 3} {
		if got := a.level(exp); got != want {
			t.Errorf("level(%d) = %d, want %d", exp, got, want)
		}
	}
}

func TestLocalPathKeepsRedirectsOnSite(t *testing.T) {
	for path, want := range map[string]bool{"/character?name=A": true, "/": true, "": false,
		"//evil.example": false, `/\evil.example`: false, "https://evil.example": false} {
		if got := localPath(path); got != want {
			t.Errorf("localPath(%q) = %v, want %v", path, got, want)
		}
	}
}
