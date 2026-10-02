package main

import (
	"bytes"
	"strings"
	"testing"

	"aionlightning/commands"
)

func TestHelpHidesAdminCommandsFromOtherAccounts(t *testing.T) {
	for _, u := range []*user{nil, {Name: "Player", AccessLevel: 0}, {Name: "GM", AccessLevel: 1}, {Name: "GM", AccessLevel: 2}} {
		v := commandHelp(u)
		if len(v.Admin) != 0 {
			t.Fatal("non-admin received command metadata")
		}
		var out bytes.Buffer
		if err := helpPage.Execute(&out, v); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "/ping") || strings.Contains(out.String(), "//spawn") || strings.Contains(out.String(), "//promote") {
			t.Fatal("non-admin help visibility incorrect")
		}
	}
}
func TestHelpShowsAllCommandsForAdmin(t *testing.T) {
	var out bytes.Buffer
	if err := helpPage.Execute(&out, commandHelp(&user{Name: "<admin>", AccessLevel: 3})); err != nil {
		t.Fatal(err)
	}
	if len(commands.Admin) != 61 {
		t.Fatalf("admin catalog=%d", len(commands.Admin))
	}
	for _, c := range commands.Admin {
		if !strings.Contains(out.String(), "//"+c.Name) {
			t.Errorf("missing %s", c.Name)
		}
	}
	if strings.Contains(out.String(), "<admin>") || !strings.Contains(out.String(), "&lt;admin&gt;") {
		t.Fatal("account name isn't escaped")
	}
}
